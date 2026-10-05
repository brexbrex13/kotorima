package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"kotorima/internal/jev"
	"kotorima/internal/smartadd"
	"strings"
	"time"
)

type jevAPI interface {
	Ask(ctx context.Context, state any, questions map[string]jev.Question) (map[string]jev.Answer, error)
	Models(ctx context.Context) error
}

type JevStatus struct {
	KeyConfigured bool   `json:"key_configured"`
	Supported     bool   `json:"supported"`
	Configured    bool   `json:"configured"`
	Hint          string `json:"hint"`
	Invalid       bool   `json:"invalid"`
	Revision      uint64 `json:"revision"`
	Unavailable   bool   `json:"unavailable"`
}

var jevProviders = map[string]bool{"typesafe": true, "vercel": true, "cloudflare": true, "custom": true, "openai": true, "local": true}

func providerName(p string) string {
	if p == "" {
		return "typesafe"
	}
	return p
}

func (a *App) jevFor(ep jev.Endpoint, key string) (jevAPI, error) {
	if a.newJev != nil {
		return a.newJev(ep, key)
	}
	return jev.NewFor(ep, key)
}

// The key file holds one JSON object of provider → key. A file written before providers
// existed holds the bare TypeSafe key and is read as such.
func (a *App) loadJevKeys() map[string]string {
	keys := map[string]string{}
	if a.jevKeys == nil || !a.jevKeys.Supported() {
		return keys
	}
	raw, e := a.jevKeys.Load()
	if e != nil {
		log.Printf("jev key: %v", e)
		return keys
	}
	if strings.HasPrefix(raw, "{") && json.Unmarshal([]byte(raw), &keys) == nil {
		return keys
	}
	if raw != "" {
		keys["typesafe"] = raw
	}
	return keys
}

func (a *App) jevKeyFor(provider string) string { return a.loadJevKeys()[providerName(provider)] }

func (a *App) jevIsBad(provider string) bool {
	a.jevMu.Lock()
	defer a.jevMu.Unlock()
	return a.jevBad[providerName(provider)]
}

func (a *App) SuggestQuickAdd(title string, generation uint64) (smartadd.Suggestion, error) {
	if generation != a.quickGeneration.Load() {
		return smartadd.Suggestion{}, nil
	}
	return a.suggest(title, true, "quick")
}

// SuggestTask serves the main window, which has no quick-add generation.
func (a *App) SuggestTask(title string, askCategory bool) (smartadd.Suggestion, error) {
	return a.suggest(title, askCategory, "main")
}

func (a *App) suggest(title string, askCategory bool, channel string) (smartadd.Suggestion, error) {
	var none smartadd.Suggestion
	if strings.TrimSpace(title) == "" {
		return none, nil
	}
	snap, e := a.store.Snapshot()
	if e != nil || !snap.Settings.SmartAdd {
		return none, e
	}
	ep := snap.Settings.JevEndpoint()
	slot := jev.KeySlot(ep)
	fallback := smartadd.Decide(smartadd.Build(snap, title, 0, false), nil)
	a.jevMu.Lock()
	revision := a.jevRevision
	key := a.jevKeyFor(slot)
	stopped, bad := a.aiStopped[slot], a.jevBad[slot]
	a.jevMu.Unlock()
	if key == "" && !jev.KeyOptional(ep) {
		return fallback, nil
	}
	if stopped || bad {
		fallback.Invalid, fallback.Unavailable = bad, true
		return fallback, nil
	}
	client, e := a.jevFor(ep, key)
	if e != nil {
		log.Printf("jev: %v", e)
		a.finishAIConnection(slot, true, false, revision)
		fallback.Unavailable = true
		return fallback, nil
	}
	timeout := aiTimeout(snap.Settings.AITimeoutSeconds, ep.Provider)
	ctx, cancel := a.startAIRequest(channel, timeout)
	defer cancel()
	limit := smartadd.MaxOpenTasks
	for attempt := 0; attempt < 2; attempt++ {
		r := smartadd.Build(snap, title, limit, askCategory)
		answers, e := client.Ask(ctx, r.State, r.Questions)
		if errors.Is(ctx.Err(), context.Canceled) {
			return none, nil
		}
		if !a.aiRequestCurrent(revision, snap.Settings.JevEndpoint(), snap.Settings.SmartAdd) {
			return none, nil
		}
		switch {
		case e == nil:
			return smartadd.Decide(r, answers), nil
		case errors.Is(e, context.Canceled):
			return fallback, nil
		case errors.Is(e, jev.ErrUnauthorized):
			a.finishAIConnection(slot, true, true, revision)
			fallback.Invalid, fallback.Unavailable = true, true
			return fallback, nil
		case errors.Is(e, jev.ErrTooLarge) && attempt == 0:
			limit = max(len(r.State.OpenTasks)/2, 1)
		default:
			log.Printf("jev: %v", e)
			a.finishAIConnection(slot, true, false, revision)
			fallback.Unavailable = true
			return fallback, nil
		}
	}
	a.finishAIConnection(slot, true, false, revision)
	fallback.Unavailable = true
	return fallback, nil
}

// GetJevStatus reports on provider, or on the provider in saved settings when it is "".
func (a *App) GetJevStatus(provider string) JevStatus {
	var ep jev.Endpoint
	if provider == "" && a.store != nil {
		if snap, e := a.store.Snapshot(); e == nil {
			ep = snap.Settings.JevEndpoint()
			provider = jev.KeySlot(ep)
		}
	}
	if strings.HasPrefix(provider, "local@") {
		ep = jev.Endpoint{Provider: "local", BaseURL: strings.TrimPrefix(provider, "local@"), Model: "status"}
	}
	a.jevMu.Lock()
	defer a.jevMu.Unlock()
	supported := a.jevKeys != nil && a.jevKeys.Supported()
	key := a.jevKeyFor(provider)
	st := JevStatus{KeyConfigured: key != "", Supported: supported || jev.KeyOptional(ep), Configured: key != "" || jev.KeyOptional(ep), Invalid: a.jevBad[providerName(provider)], Revision: a.jevRevision}
	st.Unavailable = a.aiStopped[providerName(provider)]
	if len(key) >= 8 {
		st.Hint = key[len(key)-4:]
	}
	return st
}

func (a *App) saveJevKey(provider, key string) error {
	provider = providerName(provider)
	kind, base, scoped := strings.Cut(provider, "@")
	if !jevProviders[kind] || (scoped && (!jev.Compatible(kind) || jev.ValidateEndpoint(jev.Endpoint{Provider: kind, BaseURL: base, Model: "check"}) != nil)) || (!scoped && jev.Compatible(kind)) {
		return errors.New("不明なJevプロバイダーです")
	}
	if a.jevKeys == nil || !a.jevKeys.Supported() {
		return errors.New("この環境では使えません")
	}
	a.jevMu.Lock()
	keys := a.loadJevKeys()
	if key == "" {
		delete(keys, provider)
	} else {
		keys[provider] = key
	}
	var e error
	if len(keys) == 0 {
		e = a.jevKeys.Clear()
	} else {
		b, _ := json.Marshal(keys)
		e = a.jevKeys.Save(string(b))
	}
	if e != nil {
		a.jevMu.Unlock()
		return e
	}
	if a.jevBad == nil {
		a.jevBad = map[string]bool{}
	}
	a.jevBad[provider] = false
	delete(a.aiStopped, provider)
	a.jevRevision++
	for _, p := range a.aiPending {
		p.cancel()
	}
	a.jevMu.Unlock()
	a.emitJev()
	return nil
}

func (a *App) SetJevKey(provider, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("APIキーを入力してください")
	}
	return a.saveJevKey(provider, key)
}

func (a *App) ClearJevKey(provider string) error {
	if a.jevKeys == nil {
		return nil
	}
	return a.saveJevKey(provider, "")
}

// TestJevKey checks the endpoint as currently entered in the settings form, which may not
// be saved yet, with the key saved for its provider.
func (a *App) TestJevKey(ep jev.Endpoint) (string, error) {
	slot := jev.KeySlot(ep)
	a.jevMu.Lock()
	revision := a.jevRevision
	key := a.jevKeyFor(slot)
	a.jevMu.Unlock()
	if key == "" && !jev.KeyOptional(ep) {
		return "", errors.New("APIキーが未設定です")
	}
	client, e := a.jevFor(ep, key)
	if e != nil {
		return "", e
	}
	timeout := aiTimeout(ep.TimeoutSeconds, ep.Provider)
	if ep.TimeoutSeconds == 0 && !jev.Compatible(ep.Provider) {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	e = client.Models(ctx)
	a.jevMu.Lock()
	changed := a.jevRevision != revision
	a.jevMu.Unlock()
	if changed {
		return "", errors.New("接続設定が変更されました。もう一度テストしてください")
	}
	switch {
	case e == nil:
		if !a.finishAIConnection(slot, false, false, revision) {
			return "", errors.New("接続設定が変更されました。もう一度確認してください")
		}
		return "ok", nil
	case errors.Is(e, jev.ErrUnauthorized):
		a.finishAIConnection(slot, true, true, revision)
		return "invalid", nil
	default:
		log.Printf("jev: %v", e)
		a.finishAIConnection(slot, true, false, revision)
		return "unreachable", nil
	}
}

func (a *App) finishAIConnection(slot string, stopped, bad bool, revision uint64) bool {
	a.jevMu.Lock()
	if a.jevRevision != revision {
		a.jevMu.Unlock()
		return false
	}
	if a.aiStopped == nil {
		a.aiStopped = map[string]bool{}
	}
	if a.jevBad == nil {
		a.jevBad = map[string]bool{}
	}
	if stopped && a.jevBad[slot] {
		bad = true
	}
	a.aiStopped[slot], a.jevBad[slot] = stopped, bad
	a.jevRevision++
	for _, p := range a.aiPending {
		p.cancel()
	}
	a.jevMu.Unlock()
	a.emitJev()
	return true
}

func (a *App) emitJev() {
	if a.desktop != nil {
		a.desktop.Event.Emit("board:jev")
	}
}

func aiTimeout(seconds int, provider string) time.Duration {
	if seconds > 0 && seconds <= 120 {
		return time.Duration(seconds) * time.Second
	}
	if jev.Compatible(provider) {
		return 20 * time.Second
	}
	return 1500 * time.Millisecond
}

func (a *App) aiRequestCurrent(revision uint64, ep jev.Endpoint, enabled bool) bool {
	a.jevMu.Lock()
	current := a.jevRevision == revision
	a.jevMu.Unlock()
	if !current {
		return false
	}
	snap, e := a.store.Snapshot()
	return e == nil && enabled && snap.Settings.SmartAdd && snap.Settings.JevEndpoint() == ep
}

func (a *App) setJevBadCurrent(slot string, bad bool, revision uint64) {
	a.jevMu.Lock()
	defer a.jevMu.Unlock()
	if a.jevRevision != revision {
		return
	}
	if a.jevBad == nil {
		a.jevBad = map[string]bool{}
	}
	a.jevBad[slot] = bad
}

// Each input window has at most one active inference request.
type pendingAI struct {
	cancel context.CancelFunc
	id     uint64
}

func (a *App) startAIRequest(channel string, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	a.jevMu.Lock()
	if a.aiPending == nil {
		a.aiPending = map[string]pendingAI{}
	}
	if old, ok := a.aiPending[channel]; ok {
		old.cancel()
	}
	a.aiSequence++
	id := a.aiSequence
	a.aiPending[channel] = pendingAI{cancel: cancel, id: id}
	a.jevMu.Unlock()
	return ctx, func() {
		cancel()
		a.jevMu.Lock()
		if a.aiPending[channel].id == id {
			delete(a.aiPending, channel)
		}
		a.jevMu.Unlock()
	}
}
