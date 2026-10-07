package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSystemOne_ReturnsAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/systemone" {
			t.Errorf("expected /systemone, got %s", r.URL.Path)
		}
		var req SystemOneRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Model != "decision-model" {
			t.Errorf("expected model decision-model, got %q", req.Model)
		}
		if req.State != "show cpu" {
			t.Errorf("expected state 'show cpu', got %q", req.State)
		}
		q, ok := req.Questions["intent"]
		if !ok || q.Type != "choice" || len(q.Criteria) == 0 {
			t.Errorf("expected choice question with criteria, got %+v", q)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"answers":{"intent":{"type":"choice","choice":"EXECUTE_SYSTEM","confidence":0.9,"probabilities":{"EXECUTE_SYSTEM":0.95,"CONVERSATION":0.05}}}}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	resp, err := c.SystemOne(context.Background(), SystemOneRequest{
		Model: "decision-model",
		State: "show cpu",
		Questions: map[string]SystemOneQuestion{
			"intent": {Type: "choice", Instructions: "route", Criteria: map[string]string{"EXECUTE_SYSTEM": "metrics"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ans := resp.Answers["intent"]
	if ans.Choice != "EXECUTE_SYSTEM" {
		t.Errorf("expected EXECUTE_SYSTEM, got %q", ans.Choice)
	}
	if ans.Probabilities["EXECUTE_SYSTEM"] != 0.95 {
		t.Errorf("expected probability 0.95, got %f", ans.Probabilities["EXECUTE_SYSTEM"])
	}
}

func TestSystemOne_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	_, err := c.SystemOne(context.Background(), SystemOneRequest{Model: "m"})
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "API returned 500") {
		t.Errorf("expected API returned 500, got %v", err)
	}
}

func TestSystemOne_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not json"))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	if _, err := c.SystemOne(context.Background(), SystemOneRequest{Model: "m"}); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestSystemOne_EmptyAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	if _, err := c.SystemOne(context.Background(), SystemOneRequest{Model: "m"}); err == nil {
		t.Fatal("expected error on empty answers")
	}
}

func TestChat_DisableThinking_SendsReasoningEffortNone(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, ReasoningEffort: "high"})
	_, err := c.Chat(context.Background(), ChatRequest{
		Model:           "m",
		Messages:        []ChatMessage{{Role: "user", Content: "hi"}},
		DisableThinking: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["reasoning_effort"] != "none" {
		t.Errorf("expected reasoning_effort 'none', got %v", got["reasoning_effort"])
	}
}

func TestChat_ConfigNoneStrippedWhenNotForced(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, ReasoningEffort: "none"})
	_, err := c.Chat(context.Background(), ChatRequest{
		Model:    "m",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := got["reasoning_effort"]; ok && v == "none" {
		t.Error("expected reasoning_effort to be omitted when config is none")
	}
}
