package prompt

import (
	"strings"
	"testing"
)

func TestBuildSystemOneCriteria_IncludesPipelinesAndConversation(t *testing.T) {
	pipelines := []PipelineInfo{
		{Name: "stocks", Description: "Stock queries", Keywords: []string{"share", "nifty"}},
		{Name: "system", Description: "System metrics"},
	}
	crit := BuildSystemOneCriteria(pipelines)

	if len(crit) != 3 {
		t.Fatalf("expected 3 criteria, got %d: %v", len(crit), crit)
	}
	if _, ok := crit["CONVERSATION"]; !ok {
		t.Error("expected CONVERSATION entry")
	}
	if _, ok := crit["EXECUTE_STOCKS"]; !ok {
		t.Error("expected EXECUTE_STOCKS entry")
	}
	if !strings.Contains(crit["EXECUTE_STOCKS"], "triggers: share, nifty") {
		t.Errorf("expected keyword triggers in criteria, got %q", crit["EXECUTE_STOCKS"])
	}
	if strings.Contains(crit["EXECUTE_SYSTEM"], "triggers:") {
		t.Errorf("expected no triggers without keywords, got %q", crit["EXECUTE_SYSTEM"])
	}
}

func TestBuildSystemOneCriteria_EmptyListStillHasConversation(t *testing.T) {
	crit := BuildSystemOneCriteria(nil)
	if len(crit) != 1 || crit["CONVERSATION"] == "" {
		t.Errorf("expected only CONVERSATION entry, got %v", crit)
	}
}

func TestSystemOneInstructions_CoversRoutingRules(t *testing.T) {
	for _, want := range []string{"CONVERSATION", "EXECUTE_FETCH_URL", "EXECUTE_SITE_SEARCH", "EXECUTE_TODO", "EXECUTE_SCHEDULE"} {
		if !strings.Contains(SystemOneInstructions, want) {
			t.Errorf("expected instructions to mention %s", want)
		}
	}
}
