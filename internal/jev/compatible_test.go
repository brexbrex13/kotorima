package jev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompatibleWireAndCandidateValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "" {
			t.Error(r.URL, r.Header)
		}
		var payload struct {
			Model          string
			Messages       []struct{ Role, Content string }
			ResponseFormat map[string]string `json:"response_format"`
		}
		if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
			t.Error(e)
		}
		if payload.Model != "local-model" || payload.ResponseFormat["type"] != "json_object" || len(payload.Messages) != 2 || payload.Messages[0].Content != decisionPrompt {
			t.Error(payload)
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"important":true,"category":"invented","deadline":"tomorrow","reminder":123,"unknown":"x"}`}, "finish_reason": "stop"}}})
	}))
	defer srv.Close()
	c, e := NewFor(Endpoint{Provider: "local", BaseURL: srv.URL + "/v1", Model: "local-model"}, "")
	if e != nil {
		t.Fatal(e)
	}
	answers, e := c.Ask(context.Background(), map[string]string{"input": "明日までに提出"}, map[string]Question{
		"important": Noul("i", "y", "n"), "category": Choice("c", []Option{{Key: "work"}}), "deadline": Choice("d", []Option{{Key: "tomorrow"}}), "reminder": Choice("r", []Option{{Key: "off"}}),
	})
	if e != nil || !answers["deadline"].Generated || answers["deadline"].Confidence != 0 || answers["important"].Noul != 1 || len(answers) != 2 {
		t.Fatal(answers, e)
	}
}

func TestCompatibleRejectsBrokenOrTruncatedResponses(t *testing.T) {
	for _, tc := range []struct{ content, finish string }{{`[]`, "stop"}, {`{"ok":true}`, "length"}, {`{"ok":"true"}`, "stop"}, {`null`, "stop"}, {`{"ok":null}`, "stop"}, {`not json`, "stop"}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": tc.content}, "finish_reason": tc.finish}}})
		}))
		c, _ := NewFor(Endpoint{Provider: "local", BaseURL: srv.URL + "/v1", Model: "m"}, "")
		if _, e := c.Ask(context.Background(), "x", map[string]Question{"ok": Noul("i", "y", "n")}); e == nil {
			t.Error(tc)
		}
		srv.Close()
	}
}

func TestCompatibleEndpointAndKeyScope(t *testing.T) {
	for _, ep := range []Endpoint{{Provider: "local", BaseURL: "http://remote.example/v1", Model: "m"}, {Provider: "openai", BaseURL: "https://user:pass@example.com/v1", Model: "m"}, {Provider: "openai", BaseURL: "https://example.com/v1?key=x", Model: "m"}} {
		if ValidateEndpoint(ep) == nil {
			t.Error(ep)
		}
	}
	a := Endpoint{Provider: "openai", BaseURL: "https://a.example/v1/", Model: "m"}
	b := Endpoint{Provider: "openai", BaseURL: "https://b.example/v1", Model: "m"}
	if KeySlot(a) == KeySlot(b) || KeySlot(a) != "openai@https://a.example/v1" {
		t.Fatal(KeySlot(a), KeySlot(b))
	}
	if !KeyOptional(Endpoint{Provider: "local", BaseURL: "http://[::1]:1234/v1", Model: "m"}) {
		t.Fatal("IPv6 localhost rejected")
	}
}
