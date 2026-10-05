package main

import (
	"context"
	"encoding/json"
	"errors"
	"kotorima/internal/board"
	"kotorima/internal/jev"
	"kotorima/internal/smartadd"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type memKeys struct {
	raw       string
	supported bool
}

func (m *memKeys) Supported() bool       { return m.supported }
func (m *memKeys) Load() (string, error) { return m.raw, nil }
func (m *memKeys) Save(raw string) error { m.raw = raw; return nil }
func (m *memKeys) Clear() error          { m.raw = ""; return nil }

func keysJSON(m map[string]string) string {
	b, _ := json.Marshal(m)
	return string(b)
}

type fakeJev struct {
	keys    []string
	eps     []jev.Endpoint
	sent    []int
	answers map[string]jev.Answer
	errs    []error
	models  error
}

func (f *fakeJev) Ask(_ context.Context, state any, _ map[string]jev.Question) (map[string]jev.Answer, error) {
	f.sent = append(f.sent, len(state.(smartadd.State).OpenTasks))
	if len(f.errs) > 0 {
		e := f.errs[0]
		f.errs = f.errs[1:]
		if e != nil {
			return nil, e
		}
	}
	return f.answers, nil
}
func (f *fakeJev) Models(context.Context) error { return f.models }

// smartApp stores raw as the key file content: a plain string is a legacy TypeSafe key.
func smartApp(t *testing.T, smart bool, raw string, edit ...func(*board.Settings)) (*App, *fakeJev) {
	t.Helper()
	s, e := board.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	v := board.Defaults()
	v.SmartAdd = smart
	for _, f := range edit {
		f(&v)
	}
	if e = s.SaveSettings(v); e != nil {
		t.Fatal(e)
	}
	f := &fakeJev{answers: map[string]jev.Answer{"important": {Noul: 0.99}}}
	a := &App{store: s, jevKeys: &memKeys{raw: raw, supported: true}}
	a.newJev = func(ep jev.Endpoint, k string) (jevAPI, error) {
		if e := jev.ValidateEndpoint(ep); e != nil {
			return nil, e
		}
		f.eps = append(f.eps, ep)
		f.keys = append(f.keys, k)
		return f, nil
	}
	return a, f
}

const account = "0123456789abcdef0123456789abcdef"

func cloudflare(v *board.Settings) {
	v.JevProvider = "cloudflare"
	v.JevCloudflareAccount = account
}

func TestSuggestQuickAddReturnsDecision(t *testing.T) {
	a, f := smartApp(t, true, "k1")
	s, e := a.SuggestQuickAdd("大事な用事", 0)
	if e != nil || !s.Important || len(f.sent) != 1 || f.keys[0] != "k1" || f.eps[0].Provider != "" {
		t.Fatalf("%+v %v %v", s, e, f.sent)
	}
}

func TestSuggestUsesSelectedProviderAndItsKey(t *testing.T) {
	a, f := smartApp(t, true, keysJSON(map[string]string{"typesafe": "ts-key", "cloudflare": "cf-key"}), cloudflare)
	if _, e := a.SuggestTask("大事な用事", true); e != nil {
		t.Fatal(e)
	}
	if len(f.eps) != 1 || f.eps[0].Provider != "cloudflare" || f.eps[0].Account != account || f.keys[0] != "cf-key" {
		t.Fatalf("%+v %v", f.eps, f.keys)
	}
}

func TestSuggestQuickAddIsSilentWhenDisabled(t *testing.T) {
	for _, tc := range []struct {
		smart bool
		raw   string
		gen   uint64
		title string
		edit  func(*board.Settings)
	}{
		{false, "k", 0, "x", func(*board.Settings) {}},
		{true, "", 0, "x", func(*board.Settings) {}},
		{true, "k", 9, "x", func(*board.Settings) {}},
		{true, "k", 0, "   ", func(*board.Settings) {}},
		// The legacy key belongs to TypeSafe, not to the selected Cloudflare provider.
		{true, "k", 0, "x", cloudflare},
	} {
		a, f := smartApp(t, tc.smart, tc.raw, tc.edit)
		s, e := a.SuggestQuickAdd(tc.title, tc.gen)
		if e != nil || s != (smartadd.Suggestion{}) || len(f.sent) != 0 {
			t.Fatalf("%+v: %+v %v %v", tc, s, e, f.sent)
		}
	}
}

func TestSuggestTaskIgnoresQuickAddGeneration(t *testing.T) {
	a, f := smartApp(t, true, "k")
	a.quickGeneration.Store(5)
	s, e := a.SuggestTask("大事な用事", false)
	if e != nil || !s.Important || len(f.sent) != 1 {
		t.Fatalf("%+v %v %v", s, e, f.sent)
	}
	if s, _ := a.SuggestTask("  ", true); s != (smartadd.Suggestion{}) || len(f.sent) != 1 {
		t.Fatal("blank title asked")
	}
}

func TestSuggestQuickAddHalvesTasksOnceWhenTooLarge(t *testing.T) {
	a, f := smartApp(t, true, "k")
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, e := a.store.SaveTask(board.Task{Title: title, Status: "pending"}); e != nil {
			t.Fatal(e)
		}
	}
	f.errs = []error{jev.ErrTooLarge, nil}
	s, e := a.SuggestQuickAdd("x", 0)
	if e != nil || !s.Important || len(f.sent) != 2 || f.sent[0] != 4 || f.sent[1] != 2 {
		t.Fatalf("%+v %v %v", s, e, f.sent)
	}
	f.sent, f.errs = nil, []error{jev.ErrTooLarge, jev.ErrTooLarge}
	s, e = a.SuggestQuickAdd("y", 0)
	if e != nil || !s.Unavailable || len(f.sent) != 2 {
		t.Fatalf("%+v %v %v", s, e, f.sent)
	}
}

func TestSuggestQuickAddSwallowsTransientErrors(t *testing.T) {
	a, f := smartApp(t, true, "k")
	f.errs = []error{errors.New("network down")}
	if s, e := a.SuggestQuickAdd("x", 0); e != nil || !s.Unavailable {
		t.Fatalf("%+v %v", s, e)
	}
}

func TestFailureStopsNetworkAndConnectionCheckResumes(t *testing.T) {
	a, f := smartApp(t, true, "key")
	if _, e := a.store.SaveTask(board.Task{Title: "既存の用事"}); e != nil {
		t.Fatal(e)
	}
	f.errs = []error{errors.New("network down")}
	s, e := a.SuggestTask("既存の用事", true)
	if e != nil || !s.Unavailable || s.Duplicate == nil {
		t.Fatal(s, e)
	}
	for _, title := range []string{"別の用事", "既存の用事"} {
		s, e = a.SuggestTask(title, true)
		if e != nil || !s.Unavailable || len(f.sent) != 1 {
			t.Fatal(s, e, f.sent)
		}
	}
	if result, e := a.TestJevKey(jev.Endpoint{}); e != nil || result != "ok" {
		t.Fatal(result, e)
	}
	if a.GetJevStatus("").Unavailable {
		t.Fatal("connection check did not resume AI")
	}
	s, e = a.SuggestTask("別の用事", true)
	if e != nil || !s.Important || len(f.sent) != 2 {
		t.Fatal(s, e, f.sent)
	}
}

func TestUnauthorizedKeyStaysInvalidUntilSavedAgain(t *testing.T) {
	a, f := smartApp(t, true, "bad")
	f.errs = []error{jev.ErrUnauthorized}
	if s, _ := a.SuggestQuickAdd("x", 0); !s.Invalid {
		t.Fatalf("%+v", s)
	}
	if s, _ := a.SuggestQuickAdd("x", 0); !s.Invalid || len(f.sent) != 1 {
		t.Fatal("asked again with an invalid key")
	}
	if !a.GetJevStatus("").Invalid || !a.GetJevStatus("typesafe").Invalid {
		t.Fatal("status not invalid")
	}
	if e := a.SetJevKey("typesafe", "  good-key-5678 \n"); e != nil {
		t.Fatal(e)
	}
	st := a.GetJevStatus("typesafe")
	if st.Invalid || !st.Configured || st.Hint != "5678" || a.jevKeyFor("typesafe") != "good-key-5678" {
		t.Fatalf("%+v", st)
	}
	if s, _ := a.SuggestQuickAdd("x", 0); s.Invalid || len(f.sent) != 2 {
		t.Fatalf("%+v", s)
	}
}

func TestInvalidIsTrackedPerProvider(t *testing.T) {
	a, f := smartApp(t, true, keysJSON(map[string]string{"typesafe": "ts-key", "cloudflare": "cf-key"}), cloudflare)
	f.errs = []error{jev.ErrUnauthorized}
	if s, _ := a.SuggestTask("x", true); !s.Invalid {
		t.Fatalf("%+v", s)
	}
	if !a.GetJevStatus("cloudflare").Invalid || a.GetJevStatus("typesafe").Invalid {
		t.Fatal("invalid leaked across providers")
	}
}

func TestKeysAreKeptPerProviderAndLegacyKeyIsMigrated(t *testing.T) {
	a, _ := smartApp(t, true, "legacy-ts-key")
	if st := a.GetJevStatus("typesafe"); !st.Configured || st.Hint != "-key" {
		t.Fatalf("%+v", st)
	}
	if e := a.SetJevKey("cloudflare", "cf-token-1234"); e != nil {
		t.Fatal(e)
	}
	if a.jevKeyFor("typesafe") != "legacy-ts-key" || a.jevKeyFor("cloudflare") != "cf-token-1234" || a.GetJevStatus("vercel").Configured {
		t.Fatal(a.jevKeys.(*memKeys).raw)
	}
	if e := a.ClearJevKey("typesafe"); e != nil {
		t.Fatal(e)
	}
	if a.GetJevStatus("typesafe").Configured || !a.GetJevStatus("cloudflare").Configured {
		t.Fatal(a.jevKeys.(*memKeys).raw)
	}
	if e := a.SetJevKey("openai", "x"); e == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestSetJevKeyRejectsBlankAndClearWorks(t *testing.T) {
	a, _ := smartApp(t, true, "old-key-1234")
	if e := a.SetJevKey("typesafe", " \n "); e == nil {
		t.Fatal("blank key accepted")
	}
	if e := a.ClearJevKey("typesafe"); e != nil || a.GetJevStatus("").Configured {
		t.Fatal(e)
	}
}

func TestJevStatusHidesShortKeysAndReportsUnsupported(t *testing.T) {
	a, _ := smartApp(t, true, "abc")
	if st := a.GetJevStatus(""); !st.Configured || st.Hint != "" {
		t.Fatalf("%+v", st)
	}
	a.jevKeys = &memKeys{supported: false}
	if st := a.GetJevStatus(""); st.Supported || st.Configured {
		t.Fatalf("%+v", st)
	}
	if st := (&App{}).GetJevStatus("typesafe"); st.Supported {
		t.Fatalf("%+v", st)
	}
}

func TestTestJevKeyResults(t *testing.T) {
	a, f := smartApp(t, true, "k")
	for _, tc := range []struct {
		err  error
		want string
	}{{nil, "ok"}, {jev.ErrUnauthorized, "invalid"}, {errors.New("dns"), "unreachable"}} {
		f.models = tc.err
		if got, e := a.TestJevKey(jev.Endpoint{}); e != nil || got != tc.want {
			t.Fatal(tc.want, got, e)
		}
	}
	if !a.GetJevStatus("").Invalid {
		t.Fatal("invalid not remembered")
	}
	if _, e := a.TestJevKey(jev.Endpoint{Provider: "vercel"}); e == nil {
		t.Fatal("missing vercel key accepted")
	}
	a.SetJevKey("cloudflare", "cf")
	if _, e := a.TestJevKey(jev.Endpoint{Provider: "cloudflare", Account: "bad"}); e == nil {
		t.Fatal("bad account accepted")
	}
	if got, e := a.TestJevKey(jev.Endpoint{Provider: "cloudflare", Account: account}); e != nil || got != "unreachable" || f.eps[len(f.eps)-1].Account != account {
		t.Fatal(got, e)
	}
}

func TestSnapshotNeverContainsKey(t *testing.T) {
	a, _ := smartApp(t, true, "tsk-secret-1234")
	v, e := a.GetSnapshot()
	if e != nil || !v.Settings.SmartAdd {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "tsk-secret-1234") {
		t.Fatal("key in snapshot")
	}
}

func TestQuickAddHeight(t *testing.T) {
	for _, tc := range []struct {
		options bool
		rows    int
		want    int
	}{{false, 0, 105}, {false, 1, 131}, {false, 3, 183}, {false, 9, 339}, {false, -1, 105}, {true, 2, 482}} {
		if got := quickAddHeight(tc.options, tc.rows); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

type callbackJev struct{ ask func() error }

func (c callbackJev) Ask(context.Context, any, map[string]jev.Question) (map[string]jev.Answer, error) {
	e := c.ask()
	return map[string]jev.Answer{"important": {Noul: 1}}, e
}
func (c callbackJev) Models(context.Context) error { return c.ask() }

func TestOldUnauthorizedResponseCannotInvalidateNewKey(t *testing.T) {
	a, _ := smartApp(t, true, "old-key-1234")
	a.newJev = func(jev.Endpoint, string) (jevAPI, error) {
		return callbackJev{ask: func() error {
			if e := a.SetJevKey("typesafe", "new-key-5678"); e != nil {
				t.Fatal(e)
			}
			return jev.ErrUnauthorized
		}}, nil
	}
	s, e := a.SuggestTask("提出する", true)
	if e != nil || s != (smartadd.Suggestion{}) || a.GetJevStatus("").Invalid {
		t.Fatal(s, e)
	}
}

func TestDisabledDuringRequestDiscardsAnswer(t *testing.T) {
	a, _ := smartApp(t, true, "key-1234")
	a.newJev = func(jev.Endpoint, string) (jevAPI, error) {
		return callbackJev{ask: func() error {
			snap, e := a.store.Snapshot()
			if e != nil {
				return e
			}
			snap.Settings.SmartAdd = false
			return a.store.SaveSettings(snap.Settings)
		}}, nil
	}
	s, e := a.SuggestTask("提出する", true)
	if e != nil || s != (smartadd.Suggestion{}) {
		t.Fatal(s, e)
	}
}

func TestLocalAIWithoutKeyAndScopedCredentials(t *testing.T) {
	ep := jev.Endpoint{Provider: "local", BaseURL: "http://localhost:1234/v1", Model: "m"}
	a, f := smartApp(t, true, "", func(v *board.Settings) {
		v.JevProvider = ep.Provider
		v.JevCustomURL = ep.BaseURL
		v.JevCustomModel = ep.Model
	})
	if st := a.GetJevStatus(""); !st.Configured || !st.Supported {
		t.Fatal(st)
	}
	s, e := a.SuggestTask("提出する", true)
	if e != nil || !s.Important || len(f.sent) != 1 || f.keys[0] != "" {
		t.Fatal(s, e, f)
	}
	if _, e := a.TestJevKey(ep); e != nil {
		t.Fatal(e)
	}
	if e := a.SetJevKey("openai@https://a.example/v1", "secret-a"); e != nil {
		t.Fatal(e)
	}
	if a.GetJevStatus("openai@https://b.example/v1").Configured {
		t.Fatal("key shared across hosts")
	}
}

type contextualJev struct {
	ask func(context.Context) (map[string]jev.Answer, error)
}

func (c contextualJev) Ask(ctx context.Context, _ any, _ map[string]jev.Question) (map[string]jev.Answer, error) {
	return c.ask(ctx)
}
func (c contextualJev) Models(context.Context) error { return nil }
func TestNewInputCancelsOlderInferenceInSameWindow(t *testing.T) {
	a, _ := smartApp(t, true, "key-1234")
	started := make(chan struct{})
	done := make(chan smartadd.Suggestion, 1)
	var calls atomic.Int32
	a.newJev = func(jev.Endpoint, string) (jevAPI, error) {
		return contextualJev{ask: func(ctx context.Context) (map[string]jev.Answer, error) {
			if calls.Add(1) == 1 {
				close(started)
				<-ctx.Done()
				return nil, ctx.Err()
			}
			return map[string]jev.Answer{"important": {Noul: 1}}, nil
		}}, nil
	}
	go func() { s, _ := a.SuggestTask("最初の入力", true); done <- s }()
	<-started
	s, e := a.SuggestTask("変更後の入力", true)
	if e != nil || !s.Important {
		t.Fatal(s, e)
	}
	select {
	case old := <-done:
		if old != (smartadd.Suggestion{}) {
			t.Fatal(old)
		}
	case <-time.After(time.Second):
		t.Fatal("older inference was not cancelled")
	}
}
