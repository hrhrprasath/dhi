package intent

import (
	"os"
	"testing"

	"github.com/hrhrprasath/dhi/internal/ai"
	"github.com/hrhrprasath/dhi/internal/services"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestLiveSystemOneSmoke(t *testing.T) {
	if os.Getenv("DHI_LIVE") == "" {
		t.Skip("set DHI_LIVE=1 to run against live Ollama")
	}

	pipelines := os.Getenv("DHI_PIPELINES")
	if pipelines == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("set DHI_PIPELINES to a pipelines.yaml for live testing")
		}
		pipelines = home + "/.dhi/pipelines.yaml"
	}

	r := NewRouter(ai.Config{
		BaseURL:         "http://localhost:11434/v1",
		Model:           envOr("DHI_BASE_MODEL", "qwen3.5:2b"),
		SystemOneModel:  envOr("DHI_SO_MODEL", "tev1:0.8b-mxfp8"),
		MaxTokens:       512,
		Temperature:     0.7,
		ReasoningEffort: "none",
	}, services.NewLogger(), t.TempDir(), pipelines)

	cases := []struct {
		input string
		want  string
	}{
		{"search for bubble sort on github", "EXECUTE_SITE_SEARCH"},
		{"stock price of reliance", "EXECUTE_STOCKS"},
		{"https://example.com/blog/2026/post", "EXECUTE_FETCH_URL"},
		{"show cpu and ram usage", "EXECUTE_SYSTEM"},
		{"add buy groceries to my todo", "EXECUTE_TODO"},
		{"how are you?", "CONVERSATION"},
		{"tell me a joke", "CONVERSATION"},
		{"schedule meeting at 3pm tomorrow", "EXECUTE_SCHEDULE"},
	}

	for _, tc := range cases {
		got, err := r.Route(tc.input, "", "")
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.input, err)
			continue
		}
		t.Logf("%-45q -> %-22s conf=%.2f params=%s",
			tc.input, got.Event, got.Confidence, string(got.Params))
		if got.Event != tc.want {
			t.Errorf("%q: expected %s, got %s (conf %.2f)", tc.input, tc.want, got.Event, got.Confidence)
		}
		if got.Event != "CONVERSATION" && got.Confidence < 0.8 {
			t.Errorf("%q: confidence %.2f below controller gate 0.8", tc.input, got.Confidence)
		}
	}
}
