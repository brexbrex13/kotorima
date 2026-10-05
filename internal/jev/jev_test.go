package jev

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, status int, body string, seen *[]byte, header *http.Header) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen, _ = io.ReadAll(r.Body)
		}
		if header != nil {
			*header = r.Header.Clone()
			(*header).Set("X-Path", r.Method+" "+r.URL.Path)
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	c := New("secret-key")
	c.AskURL = srv.URL + "/v1/systemone"
	c.ModelsURL = srv.URL + "/v1/models"
	return c
}

func TestAskSendsWireFormat(t *testing.T) {
	var body []byte
	var header http.Header
	c := serve(t, 200, `{"model":"jev-latest","answers":{"cat":{"type":"choice","choice":"b","confidence":0.9,"probabilities":{"b":0.9}},"imp":{"type":"noul","noul":0.7}}}`, &body, &header)
	got, e := c.Ask(context.Background(), map[string]string{"input": "x"}, map[string]Question{
		"cat": Choice("which", []Option{{Key: "z"}, {Key: "b", Description: "desc"}}),
		"imp": Noul("important", "yes", "no"),
	})
	if e != nil {
		t.Fatal(e)
	}
	if header.Get("Authorization") != "Bearer secret-key" || header.Get("X-Path") != "POST /v1/systemone" {
		t.Fatal(header)
	}
	s := string(body)
	for _, want := range []string{
		`"model":"jev-latest"`,
		`"criteria":{"z":null,"b":"desc"}`,
		`"criteria":{"false":"no","true":"yes"}`,
		`"state":{"input":"x"}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("%s not in %s", want, s)
		}
	}
	if got["cat"].Choice != "b" || got["cat"].Confidence != 0.9 || got["imp"].Noul != 0.7 {
		t.Fatal(got)
	}
}

func TestChoiceEscapesKeysAndDescriptions(t *testing.T) {
	q := Choice("i", []Option{{Key: "a\"b\\c\n😀"}, {Key: "<x>", Description: "line\n\"q\""}})
	var m map[string]any
	if e := json.Unmarshal(q.Criteria, &m); e != nil {
		t.Fatal(e, string(q.Criteria))
	}
	if _, ok := m["a\"b\\c\n😀"]; !ok || m["<x>"] != "line\n\"q\"" {
		t.Fatal(m)
	}
}

func TestErrorsAreClassified(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{401, `{}`, ErrUnauthorized},
		{403, `{}`, ErrUnauthorized},
		{400, `{"detail":{"error_type":"max_tokens_exceeded"}}`, ErrTooLarge},
	}
	for _, tc := range cases {
		_, e := serve(t, tc.status, tc.body, nil, nil).Ask(context.Background(), "s", map[string]Question{})
		if !errors.Is(e, tc.want) {
			t.Fatal(tc.status, e)
		}
	}
	_, e := serve(t, 500, `oops`, nil, nil).Ask(context.Background(), "s", map[string]Question{})
	if e == nil || errors.Is(e, ErrUnauthorized) || errors.Is(e, ErrTooLarge) {
		t.Fatal(e)
	}
	_, e = serve(t, 200, `not json`, nil, nil).Ask(context.Background(), "s", map[string]Question{})
	if e == nil {
		t.Fatal("bad body accepted")
	}
}

func TestModelsUsesGet(t *testing.T) {
	var header http.Header
	if e := serve(t, 200, `[]`, nil, &header).Models(context.Background()); e != nil {
		t.Fatal(e)
	}
	if header.Get("X-Path") != "GET /v1/models" || header.Get("Authorization") != "Bearer secret-key" {
		t.Fatal(header)
	}
	if e := serve(t, 401, `{}`, nil, nil).Models(context.Background()); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
}

func TestAskHonoursContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	c := New("k")
	c.AskURL = srv.URL
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, e := c.Ask(ctx, "s", map[string]Question{}); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}

func TestNewForBuildsProviderEndpoints(t *testing.T) {
	account := "0123456789abcdef0123456789ABCDEF"
	for _, tc := range []struct {
		ep                 Endpoint
		ask, models, model string
		wrap               bool
	}{
		{Endpoint{}, "https://api.typesafe.ai/v1/systemone", "https://api.typesafe.ai/v1/models", "jev-latest", false},
		{Endpoint{Provider: "typesafe"}, "https://api.typesafe.ai/v1/systemone", "https://api.typesafe.ai/v1/models", "jev-latest", false},
		{Endpoint{Provider: "vercel"}, "https://ai-gateway.vercel.sh/typesafe/v1/systemone", "https://ai-gateway.vercel.sh/typesafe/v1/models", "typesafe-ai/jev", false},
		{Endpoint{Provider: "cloudflare", Account: account}, "https://api.cloudflare.com/client/v4/accounts/" + account + "/ai/run", "", "typesafe/jev", true},
		{Endpoint{Provider: "custom", BaseURL: "https://jev.example.com/api/", Model: "my-jev"}, "https://jev.example.com/api/v1/systemone", "https://jev.example.com/api/v1/models", "my-jev", false},
		{Endpoint{Provider: "custom", BaseURL: "http://localhost:8787", Model: "m"}, "http://localhost:8787/v1/systemone", "http://localhost:8787/v1/models", "m", false},
	} {
		c, e := NewFor(tc.ep, "k")
		if e != nil {
			t.Fatal(tc.ep, e)
		}
		if c.AskURL != tc.ask || c.ModelsURL != tc.models || c.Model != tc.model || c.Wrap != tc.wrap || c.Key != "k" {
			t.Fatalf("%+v: %+v", tc.ep, c)
		}
	}
}

func TestValidateEndpointRejectsIncompleteOrUnsafeSettings(t *testing.T) {
	for _, ep := range []Endpoint{
		{Provider: "openai"},
		{Provider: "cloudflare"},
		{Provider: "cloudflare", Account: "not-hex"},
		{Provider: "custom", Model: "m"},
		{Provider: "custom", BaseURL: "http://jev.example.com", Model: "m"},
		{Provider: "custom", BaseURL: "ftp://localhost", Model: "m"},
		{Provider: "custom", BaseURL: "https://jev.example.com"},
	} {
		if e := ValidateEndpoint(ep); e == nil {
			t.Fatalf("%+v accepted", ep)
		}
		if _, e := NewFor(ep, "k"); e == nil {
			t.Fatalf("NewFor %+v accepted", ep)
		}
	}
}

func TestCloudflareWrapsInputAndUnwrapsResult(t *testing.T) {
	var body []byte
	c := serve(t, 200, `{"result":{"model":"typesafe/jev","answers":{"imp":{"type":"noul","noul":0.8}}},"success":true,"errors":[]}`, &body, nil)
	c.Wrap = true
	c.Model = "typesafe/jev"
	got, e := c.Ask(context.Background(), "s", map[string]Question{"imp": Noul("i", "y", "n")})
	if e != nil || got["imp"].Noul != 0.8 {
		t.Fatal(got, e)
	}
	if !strings.Contains(string(body), `"input":{"questions":{"imp":`) || !strings.Contains(string(body), `"model":"typesafe/jev"`) || strings.Contains(string(body), `{"model":"typesafe/jev","questions"`) {
		t.Fatal(string(body))
	}
	c = serve(t, 200, `{"model":"typesafe/jev","answers":{"imp":{"type":"noul","noul":0.6}}}`, nil, nil)
	c.Wrap = true
	if got, e = c.Ask(context.Background(), "s", map[string]Question{}); e != nil || got["imp"].Noul != 0.6 {
		t.Fatal(got, e)
	}
}

func TestModelsProbesWithAskWhenNoModelsEndpoint(t *testing.T) {
	var body []byte
	var header http.Header
	c := serve(t, 200, `{"answers":{"ok":{"type":"noul","noul":0.9}}}`, &body, &header)
	c.ModelsURL = ""
	if e := c.Models(context.Background()); e != nil {
		t.Fatal(e)
	}
	if header.Get("X-Path") != "POST /v1/systemone" || !strings.Contains(string(body), `"type":"noul"`) {
		t.Fatal(header, string(body))
	}
	c = serve(t, 403, `{"success":false}`, nil, nil)
	c.ModelsURL = ""
	if e := c.Models(context.Background()); !errors.Is(e, ErrUnauthorized) {
		t.Fatal(e)
	}
}
