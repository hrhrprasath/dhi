package intent

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hrhrprasath/dhi/internal/ai"
	"github.com/hrhrprasath/dhi/internal/prompt"
	"github.com/hrhrprasath/dhi/internal/services"
)

type soCounters struct {
	systemone int
	chat      int
	chatBody  map[string]any
}

func newSystemOneServer(systemoneBody, extractBody string, c *soCounters) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/systemone":
			c.systemone++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(systemoneBody))
		case "/chat/completions":
			c.chat++
			_ = json.NewDecoder(r.Body).Decode(&c.chatBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(extractBody))
		default:
			http.NotFound(w, r)
		}
	}))
}

func writePipelinesYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pipelines.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func soConfig(srvURL, model string) ai.Config {
	return ai.Config{BaseURL: srvURL, Model: "base-model", SystemOneModel: model}
}

func extractEnvelope(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"content": content}}},
	})
	return string(b)
}

func approx(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

const siteSearchYAML = `pipelines:
  - name: site_search
    description: Open a search on a specific website
    keywords: [search on site, search on github]
    weight: 0.4
    params:
      - name: site
        required: true
        extract_hint: "Site to search on"
      - name: query
        required: true
        extract_hint: "What to search for"
    action:
      type: bash
      script: "s --provider {site} {query}"
`

func TestRoute_SystemOne_TodoWithExtraction(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"EXECUTE_TODO","confidence":0.85,"probabilities":{"EXECUTE_TODO":0.9,"CONVERSATION":0.1}}}}`,
		extractEnvelope(`{"action":"add","text":"buy milk"}`),
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	result, err := r.Route("add buy milk to my todo", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "EXECUTE_TODO" {
		t.Errorf("expected EXECUTE_TODO, got %q", result.Event)
	}
	if !approx(result.Confidence, 0.9) {
		t.Errorf("expected confidence 0.9 (1-p(CONVERSATION)), got %f", result.Confidence)
	}
	if result.Description == "" || strings.Contains(result.Description, "triggers:") {
		t.Errorf("expected clean description, got %q", result.Description)
	}
	var params map[string]string
	if err := json.Unmarshal(result.Params, &params); err != nil {
		t.Fatalf("params unmarshal: %v", err)
	}
	if params["action"] != "add" || params["text"] != "buy milk" {
		t.Errorf("expected action=add text=buy milk, got %v", params)
	}
	if c.chat != 1 {
		t.Errorf("expected 1 extraction chat call, got %d", c.chat)
	}
	if c.chatBody["reasoning_effort"] != "none" {
		t.Errorf("expected extraction call with reasoning_effort none, got %v", c.chatBody["reasoning_effort"])
	}
	if c.chatBody["temperature"] != float64(0) {
		t.Errorf("expected temperature 0, got %v", c.chatBody["temperature"])
	}
}

func TestRoute_SystemOne_ConversationSkipsExtraction(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"CONVERSATION","confidence":0.99,"probabilities":{"CONVERSATION":0.99,"EXECUTE_TODO":0.01}}}}`,
		``,
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	result, err := r.Route("how are you?", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "CONVERSATION" {
		t.Errorf("expected CONVERSATION, got %q", result.Event)
	}
	if !approx(result.Confidence, 0.99) {
		t.Errorf("expected confidence 0.99 (p(CONVERSATION)), got %f", result.Confidence)
	}
	if c.chat != 0 {
		t.Errorf("expected no extraction call for CONVERSATION, got %d", c.chat)
	}
	if result.Params != nil {
		t.Errorf("expected nil params, got %s", result.Params)
	}
}

func TestRoute_SystemOne_NoInputPipelineSkipsExtraction(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"EXECUTE_SYSTEM","confidence":0.9,"probabilities":{"EXECUTE_SYSTEM":0.95,"CONVERSATION":0.05}}}}`,
		``,
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	result, err := r.Route("show cpu and ram usage", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "EXECUTE_SYSTEM" {
		t.Errorf("expected EXECUTE_SYSTEM, got %q", result.Event)
	}
	if !approx(result.Confidence, 0.95) {
		t.Errorf("expected confidence 0.95, got %f", result.Confidence)
	}
	if c.chat != 0 {
		t.Errorf("expected no extraction call for SYSTEM (metric has default), got %d", c.chat)
	}
	if result.Params != nil {
		t.Errorf("expected nil params, got %s", result.Params)
	}
}

func TestRoute_SystemOne_SiteSearchExtraction(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"EXECUTE_SITE_SEARCH","confidence":0.67,"probabilities":{"EXECUTE_SITE_SEARCH":0.5,"EXECUTE_WEB_SEARCH":0.4,"CONVERSATION":0.01}}}}`,
		extractEnvelope(`{"site":"github","query":"bubble sort"}`),
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), writePipelinesYAML(t, siteSearchYAML))
	result, err := r.Route("search for bubble sort on github", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "EXECUTE_SITE_SEARCH" {
		t.Errorf("expected EXECUTE_SITE_SEARCH, got %q", result.Event)
	}
	if !approx(result.Confidence, 0.99) {
		t.Errorf("expected confidence 0.99 (1-0.01), got %f", result.Confidence)
	}
	if result.Description != "Open a search on a specific website" {
		t.Errorf("expected clean pipeline description, got %q", result.Description)
	}
	var params map[string]string
	if err := json.Unmarshal(result.Params, &params); err != nil {
		t.Fatalf("params unmarshal: %v", err)
	}
	if params["site"] != "github" || params["query"] != "bubble sort" {
		t.Errorf("expected site=github query=bubble sort, got %v", params)
	}
}

func TestRoute_SystemOne_EmptyChoiceReturnsError(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(`{"answers":{"intent":{"confidence":0.5}}}`, ``, c)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	if _, err := r.Route("hello", "", ""); err == nil || !strings.Contains(err.Error(), "no intent choice") {
		t.Errorf("expected 'no intent choice' error, got %v", err)
	}
}

func TestRoute_SystemOne_APIErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	if _, err := r.Route("hello", "", ""); err == nil || !strings.Contains(err.Error(), "systemone route failed") {
		t.Errorf("expected 'systemone route failed' error, got %v", err)
	}
}

func TestRoute_SystemOne_UnknownEventReturnsError(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"EXECUTE_BOGUS","confidence":0.9,"probabilities":{"EXECUTE_BOGUS":0.9,"CONVERSATION":0.1}}}}`,
		``,
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	if _, err := r.Route("hello", "", ""); err == nil || !strings.Contains(err.Error(), "unknown event") {
		t.Errorf("expected 'unknown event' error, got %v", err)
	}
}

func TestRoute_SystemOne_ExtractionBadJSONDegrades(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"EXECUTE_TODO","confidence":0.9,"probabilities":{"EXECUTE_TODO":0.9,"CONVERSATION":0.1}}}}`,
		extractEnvelope(`this is not json`),
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	result, err := r.Route("add buy milk", "", "")
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if result.Params != nil {
		t.Errorf("expected nil params after bad extraction, got %s", result.Params)
	}
}

func TestRoute_SystemOne_ExtractionFiltersUnknownParams(t *testing.T) {
	c := &soCounters{}
	srv := newSystemOneServer(
		`{"answers":{"intent":{"choice":"EXECUTE_SITE_SEARCH","confidence":0.9,"probabilities":{"EXECUTE_SITE_SEARCH":0.9,"CONVERSATION":0.1}}}}`,
		extractEnvelope(`{"site":"github","query":"bubble sort","bogus":"drop me","search_terms":["x"]}`),
		c,
	)
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, "decision-model"), services.NewLogger(), t.TempDir(), writePipelinesYAML(t, siteSearchYAML))
	result, err := r.Route("search bubble sort on github", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]string
	if err := json.Unmarshal(result.Params, &params); err != nil {
		t.Fatalf("params unmarshal: %v", err)
	}
	if _, ok := params["bogus"]; ok {
		t.Error("expected unknown param 'bogus' to be filtered")
	}
	if _, ok := params["search_terms"]; ok {
		t.Error("expected undeclared param 'search_terms' to be filtered")
	}
	if params["site"] != "github" || params["query"] != "bubble sort" {
		t.Errorf("expected site and query kept, got %v", params)
	}
}

func TestRoute_SystemOneNotConfigured_UsesChatPath(t *testing.T) {
	c := &soCounters{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/systemone" {
			t.Error("systemone must not be called when not configured")
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"event\":\"EXECUTE_TODO\",\"confidence\":0.9}"}}]}`))
	}))
	defer srv.Close()

	r := NewRouter(soConfig(srv.URL, ""), services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	result, err := r.Route("add buy milk", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "EXECUTE_TODO" {
		t.Errorf("expected chat path result EXECUTE_TODO, got %q", result.Event)
	}
	if c.systemone != 0 {
		t.Errorf("expected 0 systemone calls, got %d", c.systemone)
	}
}

func TestSystemOneConfidence(t *testing.T) {
	tests := []struct {
		name string
		ans  ai.SystemOneAnswer
		want float64
	}{
		{
			name: "execute uses inverse conversation probability",
			ans:  ai.SystemOneAnswer{Choice: "EXECUTE_TODO", Probabilities: map[string]float64{"EXECUTE_TODO": 0.9, "CONVERSATION": 0.1}},
			want: 0.9,
		},
		{
			name: "conversation uses own probability",
			ans:  ai.SystemOneAnswer{Choice: "CONVERSATION", Probabilities: map[string]float64{"CONVERSATION": 0.99, "EXECUTE_TODO": 0.01}},
			want: 0.99,
		},
		{
			name: "falls back to confidence field without probabilities",
			ans:  ai.SystemOneAnswer{Choice: "EXECUTE_TODO", Confidence: 0.85},
			want: 0.85,
		},
		{
			name: "defaults to 0.5 when nothing provided",
			ans:  ai.SystemOneAnswer{Choice: "EXECUTE_TODO"},
			want: 0.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := systemOneConfidence(tt.ans); !approx(got, tt.want) {
				t.Errorf("expected %f, got %f", tt.want, got)
			}
		})
	}
}

func TestNeedsInput(t *testing.T) {
	tests := []struct {
		name   string
		params []prompt.ParamInfo
		want   bool
	}{
		{"all defaults", []prompt.ParamInfo{{Name: "metric", Default: "all"}}, false},
		{"no default", []prompt.ParamInfo{{Name: "text"}}, true},
		{"required", []prompt.ParamInfo{{Name: "site", Required: true, Default: ""}}, true},
		{"no params", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsInput(tt.params); got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestFindPipeline(t *testing.T) {
	pipelines := []prompt.PipelineInfo{{Name: "TODO"}, {Name: "SITE_SEARCH"}}
	if p := findPipeline("EXECUTE_TODO", pipelines); p == nil || p.Name != "TODO" {
		t.Errorf("expected TODO pipeline, got %v", p)
	}
	if p := findPipeline("CONVERSATION", pipelines); p != nil {
		t.Errorf("expected nil for CONVERSATION, got %v", p)
	}
	if p := findPipeline("EXECUTE_BOGUS", pipelines); p != nil {
		t.Errorf("expected nil for unknown event, got %v", p)
	}
}

func TestParseExtractedParams(t *testing.T) {
	defs := []prompt.ParamInfo{{Name: "site"}, {Name: "query"}}
	raw, err := parseExtractedParams(`Here you go: {"site": "github", "query": "sort", "extra": "x"}`, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m["site"] != "github" || m["query"] != "sort" {
		t.Errorf("expected filtered params, got %v", m)
	}

	if _, err := parseExtractedParams("no json here", defs); err == nil {
		t.Error("expected error for response without JSON")
	}
	if _, err := parseExtractedParams(`{"unrelated": "x"}`, defs); err == nil {
		t.Error("expected error when no declared params present")
	}
}
