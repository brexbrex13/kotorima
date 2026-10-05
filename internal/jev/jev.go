// Package jev is a minimal client for TypeSafe AI's Jev decision model.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrUnauthorized = errors.New("jev: unauthorized")
	ErrTooLarge     = errors.New("jev: request too large")
)

type Option struct{ Key, Description string }

type Question struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

// Noul criteria must be nested; top-level true/false are silently ignored by the API.
func Noul(instructions, whenTrue, whenFalse string) Question {
	b, _ := json.Marshal(map[string]string{"true": whenTrue, "false": whenFalse})
	return Question{Type: "noul", Instructions: instructions, Criteria: b}
}

// Choice keeps option order, which affects answers when there are few options.
// An empty description is sent as null so the option is judged by its name.
func Choice(instructions string, options []Option) Question {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, o := range options {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(o.Key)
		b.Write(k)
		b.WriteByte(':')
		if o.Description == "" {
			b.WriteString("null")
		} else {
			d, _ := json.Marshal(o.Description)
			b.Write(d)
		}
	}
	b.WriteByte('}')
	return Question{Type: "choice", Instructions: instructions, Criteria: b.Bytes()}
}

type Answer struct {
	Generated     bool               `json:"-"` // accepted generation; not a calibrated probability
	Type          string             `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Score         float64            `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// Endpoint selects a Jev or OpenAI-compatible connection. Provider "" means TypeSafe.
// Account is used by Cloudflare; BaseURL and Model by user-defined connections.
type Endpoint struct {
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Provider       string `json:"provider"`
	Account        string `json:"account"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
}

var cloudflareAccount = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

func ValidateEndpoint(ep Endpoint) error {
	if ep.TimeoutSeconds < 0 || ep.TimeoutSeconds > 120 {
		return errors.New("AIの待ち時間は0〜120秒で指定してください")
	}
	switch ep.Provider {
	case "", "typesafe", "vercel":
		return nil
	case "cloudflare":
		if !cloudflareAccount.MatchString(ep.Account) {
			return errors.New("CloudflareのアカウントIDは英数字32桁で入力してください")
		}
		return nil
	case "custom", "openai", "local":
		u, e := url.Parse(ep.BaseURL)
		local := e == nil && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
		if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !local) {
			return errors.New("ベースURLは https:// で入力してください（http は localhost のみ）")
		}
		if ep.Provider == "local" && !isLoopback(u.Hostname()) {
			return errors.New("ローカルAIは localhost・127.0.0.1・[::1] を指定してください")
		}
		if strings.TrimSpace(ep.Model) == "" {
			return errors.New("モデル名を入力してください")
		}
		return nil
	}
	return errors.New("不明なJevプロバイダーです")
}

type Client struct {
	AskURL     string
	ModelsURL  string // empty: probe with a one-question Ask
	Model      string
	Key        string
	Compatible bool // OpenAI-compatible Chat Completions
	Wrap       bool // Cloudflare: send {model, input}, answers may sit under "result"
	HTTP       *http.Client
}

func New(key string) *Client {
	c, _ := NewFor(Endpoint{}, key)
	return c
}

func NewFor(ep Endpoint, key string) (*Client, error) {
	if e := ValidateEndpoint(ep); e != nil {
		return nil, e
	}
	c := &Client{Key: key, HTTP: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	switch ep.Provider {
	case "vercel":
		c.AskURL, c.ModelsURL, c.Model = "https://ai-gateway.vercel.sh/typesafe/v1/systemone", "https://ai-gateway.vercel.sh/typesafe/v1/models", "typesafe-ai/jev"
	case "cloudflare":
		c.AskURL, c.Model, c.Wrap = "https://api.cloudflare.com/client/v4/accounts/"+ep.Account+"/ai/run", "typesafe/jev", true
	case "openai", "local":
		base := strings.TrimRight(ep.BaseURL, "/")
		c.AskURL, c.Model, c.Compatible = base+"/chat/completions", strings.TrimSpace(ep.Model), true
	case "custom":
		base := strings.TrimRight(ep.BaseURL, "/")
		c.AskURL, c.ModelsURL, c.Model = base+"/v1/systemone", base+"/v1/models", strings.TrimSpace(ep.Model)
	default:
		c.AskURL, c.ModelsURL, c.Model = "https://api.typesafe.ai/v1/systemone", "https://api.typesafe.ai/v1/models", "jev-latest"
	}
	return c, nil
}

func (c *Client) Ask(ctx context.Context, state any, questions map[string]Question) (map[string]Answer, error) {
	if c.Compatible {
		return c.askCompatible(ctx, state, questions)
	}
	payload := map[string]any{"model": c.Model, "state": state, "questions": questions}
	if c.Wrap {
		payload = map[string]any{"model": c.Model, "input": map[string]any{"state": state, "questions": questions}}
	}
	body, e := json.Marshal(payload)
	if e != nil {
		return nil, e
	}
	var out struct {
		Answers map[string]Answer `json:"answers"`
		Result  *struct {
			Answers map[string]Answer `json:"answers"`
		} `json:"result"`
	}
	if e = c.do(ctx, http.MethodPost, c.AskURL, body, &out); e != nil {
		return nil, e
	}
	if out.Answers == nil && out.Result != nil {
		return out.Result.Answers, nil
	}
	return out.Answers, nil
}

func (c *Client) Models(ctx context.Context) error {
	if c.Compatible {
		_, e := c.Ask(ctx, "connection test", map[string]Question{"ok": Noul("接続テストです。true を返してください。", "yes", "no")})
		return e
	}
	if c.ModelsURL == "" {
		_, e := c.Ask(ctx, "connection test", map[string]Question{"ok": Noul("This is a connection test.", "yes", "no")})
		return e
	}
	return c.do(ctx, http.MethodGet, c.ModelsURL, nil, nil)
}

func (c *Client) do(ctx context.Context, method, target string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, e := http.NewRequestWithContext(ctx, method, target, reader)
	if e != nil {
		return e
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := c.HTTP.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if e != nil {
		return e
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case res.StatusCode == http.StatusBadRequest && bytes.Contains(b, []byte("max_tokens_exceeded")):
		return ErrTooLarge
	case res.StatusCode/100 != 2:
		return fmt.Errorf("AI: HTTP %d", res.StatusCode)
	}
	if out == nil {
		return nil
	}
	if e = json.Unmarshal(b, out); e != nil {
		return fmt.Errorf("jev: bad response: %w", e)
	}
	return nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// KeySlot binds user-defined endpoints to their own credentials.
func KeySlot(ep Endpoint) string {
	p := ep.Provider
	if p == "" {
		p = "typesafe"
	}
	if p == "openai" || p == "local" {
		return p + "@" + strings.TrimRight(strings.TrimSpace(ep.BaseURL), "/")
	}
	return p
}
