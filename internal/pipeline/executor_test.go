package pipeline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hrhrprasath/dhi/internal/ai"
	"github.com/hrhrprasath/dhi/internal/summarizer"
	"github.com/hrhrprasath/dhi/internal/task"
)

func newTestExecutor(t *testing.T) *Executor {
	t.Helper()

	mgr := NewManager(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	taskMgr := task.NewManager(t.TempDir(), nil)
	sum := &summarizer.Summarizer{}

	return NewExecutor(mgr, sum, taskMgr, t.TempDir(), t.TempDir())
}

func TestFindAndExecute_TODOReturnsNonEmptyResult(t *testing.T) {
	e := newTestExecutor(t)

	raw, synth := e.FindAndExecute("EXECUTE_TODO", nil, "test task", "")

	if raw == "" {
		t.Fatal("expected non-empty raw result for EXECUTE_TODO")
	}
	if synth == "" {
		t.Fatal("expected non-empty synthesized result for EXECUTE_TODO")
	}
}

func TestFindAndExecute_TODOWithListAction(t *testing.T) {
	e := newTestExecutor(t)

	raw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "list"}, "", "")

	if raw == "" {
		t.Fatal("expected non-empty result for EXECUTE_TODO list")
	}
}

func TestFindAndExecute_TODOWithAddAction(t *testing.T) {
	e := newTestExecutor(t)

	raw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "add", "text": "buy milk"}, "", "")

	if raw == "" {
		t.Fatal("expected non-empty result for EXECUTE_TODO add")
	}
}

func TestFindAndExecute_SYSTEMReturnsEventUnknown(t *testing.T) {
	e := newTestExecutor(t)

	raw, synth := e.FindAndExecute("EXECUTE_SYSTEM", nil, "", "")

	if raw != "" {
		t.Fatalf("expected empty raw result for unknown pipeline, got %q", raw)
	}
	if synth != "Unable to determine action: event unknown" {
		t.Fatalf("expected 'Unable to determine action: event unknown', got %q", synth)
	}
}

func TestFindAndExecute_SCHEDULEReturnsNonEmptyResult(t *testing.T) {
	e := newTestExecutor(t)

	raw, synth := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{"action": "list"}, "", "")

	if raw == "" {
		t.Fatal("expected non-empty raw result for EXECUTE_SCHEDULE")
	}
	if synth == "" {
		t.Fatal("expected non-empty synthesized result for EXECUTE_SCHEDULE")
	}
}

func TestFindAndExecute_SCHEDULEWithListAction(t *testing.T) {
	e := newTestExecutor(t)

	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{"action": "list"}, "", "")

	if raw == "" {
		t.Fatal("expected non-empty result for EXECUTE_SCHEDULE list")
	}
}

func TestFindAndExecute_UnknownEventReturnsUnable(t *testing.T) {
	e := newTestExecutor(t)

	_, synth := e.FindAndExecute("EXECUTE_NONEXISTENT", nil, "", "")

	if synth != "Unable to determine action: event unknown" {
		t.Fatalf("expected 'Unable to determine action: event unknown', got %q", synth)
	}
}

func TestFindAndExecute_NonExecuteEventFallsBack(t *testing.T) {
	e := newTestExecutor(t)

	_, synth := e.FindAndExecute("CONVERSATION", nil, "", "")

	if synth != "Unable to determine action: event unknown" {
		t.Fatalf("expected 'Unable to determine action: event unknown' for non-EXECUTE_ event, got %q", synth)
	}
}

func TestFindAndExecute_InvalidParamsReturnsError(t *testing.T) {
	e := newTestExecutor(t)

	_, synth := e.FindAndExecute("EXECUTE_TODO", map[string]string{"invalid_param": "value"}, "", "")

	if synth == "" {
		t.Fatal("expected non-empty error message for invalid params")
	}
}

func TestFindAndExecute_CaseInsensitiveEvent(t *testing.T) {
	e := newTestExecutor(t)

	raw, synth := e.FindAndExecute("execute_todo", map[string]string{"action": "list"}, "", "")

	if raw == "" {
		t.Fatal("expected non-empty raw result for lowercase EXECUTE_TODO")
	}
	if synth == "" {
		t.Fatal("expected non-empty synthesized result for lowercase EXECUTE_TODO")
	}
}

func TestSetModel(t *testing.T) {
	e := newTestExecutor(t)

	e.SetModel("test-model-123")

	if e.Summarizer() == nil {
		t.Fatal("expected non-nil summarizer after SetModel")
	}
}

func TestListPipelines(t *testing.T) {
	e := newTestExecutor(t)

	names := e.ListPipelines()
	if len(names) < 2 {
		t.Fatalf("expected at least 2 pipelines, got %d", len(names))
	}

	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}
	if !found["todo"] {
		t.Fatal("expected 'todo' in pipeline list")
	}
	if !found["schedule"] {
		t.Fatal("expected 'schedule' in pipeline list")
	}
}

func TestInterpolate(t *testing.T) {
	e := newTestExecutor(t)

	params := map[string]string{
		"query": "hello world",
		"limit": "10",
	}

	result := e.interpolate("search {query} limit {limit}", params)
	expected := "search hello world limit 10"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}

	result = e.interpolate("{unknown} {query}", params)
	expected = "{unknown} hello world"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}

	result = e.interpolate("", params)
	if result != "" {
		t.Fatalf("expected empty, got %q", result)
	}

	result = e.interpolate("static text", params)
	if result != "static text" {
		t.Fatalf("expected 'static text', got %q", result)
	}

	result = e.interpolate("{query} and {query}", params)
	expected = "hello world and hello world"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestResolveScript(t *testing.T) {
	e := newTestExecutor(t)

	script := e.resolveScript("scripts/test.sh arg1 arg2")
	if !strings.Contains(script, "/test.sh") {
		t.Fatalf("expected resolved path containing test.sh, got %q", script)
	}
	if !strings.HasPrefix(script, e.scriptsDir) {
		t.Fatalf("expected script to start with scriptsDir, got %q", script)
	}

	script = e.resolveScript("echo hello")
	if script != "echo hello" {
		t.Fatalf("expected 'echo hello', got %q", script)
	}

	script = e.resolveScript("")
	if script != "" {
		t.Fatalf("expected empty, got %q", script)
	}
}

// --- runCmd ---

func TestRunCmd_Success(t *testing.T) {
	out, err := runCmd("echo", "hello", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Fatalf("expected 'hello world', got %q", out)
	}
}

func TestRunCmd_Error(t *testing.T) {
	out, err := runCmd("false")
	if err == nil {
		t.Fatal("expected error from false command")
	}
	if out != "" {
		t.Fatalf("expected empty output on error, got %q", out)
	}
}

func TestRunCmd_WithStderr(t *testing.T) {
	out, err := runCmd("bash", "-c", "echo stderr output >&2 && exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out, "stderr output") {
		t.Fatalf("expected stderr output in result, got %q", out)
	}
}

// --- executeBash ---

func TestExecuteBash_Success(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "bash", Script: "echo hello from bash"}
	result := e.executeBash(action, map[string]string{})
	if result != "hello from bash" {
		t.Fatalf("expected 'hello from bash', got %q", result)
	}
}

func TestExecuteBash_WithParams(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "bash", Script: "echo {message}"}
	result := e.executeBash(action, map[string]string{"message": "param test"})
	if result != "param test" {
		t.Fatalf("expected 'param test', got %q", result)
	}
}

func TestExecuteBash_Error(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "bash", Script: "exit 1"}
	result := e.executeBash(action, map[string]string{})
	if !strings.Contains(result, "Script failed") {
		t.Fatalf("expected 'Script failed' error, got %q", result)
	}
}

func TestExecuteBash_ResolvesScriptPath(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "bash", Script: "scripts/test.sh arg1"}
	result := e.executeBash(action, map[string]string{})
	if !strings.Contains(result, "Script failed") && !strings.Contains(result, "No such file") {
		t.Fatalf("expected error about missing script, got %q", result)
	}
}

// --- executeExec ---

func TestExecuteExec_Success(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "exec", Command: "echo", Args: []string{"hello exec"}}
	result := e.executeExec(action, map[string]string{})
	if result != "hello exec" {
		t.Fatalf("expected 'hello exec', got %q", result)
	}
}

func TestExecuteExec_WithInterpolation(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "exec", Command: "echo", Args: []string{"{msg}"}}
	result := e.executeExec(action, map[string]string{"msg": "interpolated"})
	if result != "interpolated" {
		t.Fatalf("expected 'interpolated', got %q", result)
	}
}

func TestExecuteExec_Error(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "exec", Command: "false"}
	result := e.executeExec(action, map[string]string{})
	if !strings.Contains(result, "Command failed") {
		t.Fatalf("expected 'Command failed' error, got %q", result)
	}
}

// --- executeHTTP ---

func TestExecuteHTTP_Error(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "http", URL: "http://127.0.0.1:1/nonexistent", Method: "GET"}
	result := e.executeHTTP(action, map[string]string{})
	if !strings.Contains(result, "HTTP request failed") {
		t.Fatalf("expected 'HTTP request failed' error, got %q", result)
	}
}

// --- executeAction ---

func TestExecuteAction_UnknownType(t *testing.T) {
	e := newTestExecutor(t)
	action := PipelineAction{Type: "python", Script: "print('hi')"}
	result := e.executeAction(&Pipeline{Action: action}, nil)
	if !strings.Contains(result, "Unknown action type") {
		t.Fatalf("expected 'Unknown action type' error, got %q", result)
	}
}

// --- eventToPipeline ---

func TestEventToPipeline_MatchingEvent(t *testing.T) {
	e := newTestExecutor(t)
	result := e.eventToPipeline("EXECUTE_TODO")
	if result != "todo" {
		t.Fatalf("expected 'todo', got %q", result)
	}
}

func TestEventToPipeline_LowercaseEvent(t *testing.T) {
	e := newTestExecutor(t)
	result := e.eventToPipeline("execute_todo")
	if result != "todo" {
		t.Fatalf("expected 'todo', got %q", result)
	}
}

func TestEventToPipeline_UnknownEvent(t *testing.T) {
	e := newTestExecutor(t)
	result := e.eventToPipeline("EXECUTE_NONEXISTENT")
	if result != "web_search" {
		t.Fatalf("expected 'web_search' fallback, got %q", result)
	}
}

func TestEventToPipeline_NonExecuteEvent(t *testing.T) {
	e := newTestExecutor(t)
	result := e.eventToPipeline("CONVERSATION")
	if result != "web_search" {
		t.Fatalf("expected 'web_search' fallback, got %q", result)
	}
}

// --- synthesizeOutput ---

func TestSynthesizeOutput_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "synthesized result"}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test-model", MaxTokens: 200, Temperature: 0.5}
	sum := summarizer.New(cfg)
	mgr := NewManager(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	taskMgr := task.NewManager(t.TempDir(), nil)
	e := NewExecutor(mgr, sum, taskMgr, t.TempDir(), t.TempDir())

	result := e.synthesizeOutput("raw data", "test_pipeline", "user query", "ctx")
	if result != "synthesized result" {
		t.Fatalf("expected 'synthesized result', got %q", result)
	}
}

// --- FindAndExecute with custom pipelines via YAML ---

func newExecutorWithYAML(t *testing.T, yamlContent string) *Executor {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelines.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(path)
	taskMgr := task.NewManager(t.TempDir(), nil)
	sum := &summarizer.Summarizer{}
	return NewExecutor(mgr, sum, taskMgr, t.TempDir(), t.TempDir())
}

func TestFindAndExecute_BashPipeline(t *testing.T) {
	yaml := `
pipelines:
  - name: test_bash
    description: Test bash
    weight: 0.5
    action:
      type: bash
      script: "echo hello from custom bash"
`
	e := newExecutorWithYAML(t, yaml)
	raw, synth := e.FindAndExecute("EXECUTE_TEST_BASH", nil, "", "")
	if raw != "hello from custom bash" {
		t.Fatalf("expected 'hello from custom bash', got %q", raw)
	}
	if synth != raw {
		t.Fatalf("expected synth to equal raw for non-summarize, got %q", synth)
	}
}

func TestFindAndExecute_ExecPipeline(t *testing.T) {
	yaml := `
pipelines:
  - name: test_exec
    description: Test exec
    weight: 0.5
    action:
      type: exec
      command: "echo"
      args: ["exec pipeline test"]
`
	e := newExecutorWithYAML(t, yaml)
	raw, synth := e.FindAndExecute("EXECUTE_TEST_EXEC", nil, "", "")
	if raw != "exec pipeline test" {
		t.Fatalf("expected 'exec pipeline test', got %q", raw)
	}
	if synth != raw {
		t.Fatalf("expected synth to equal raw, got %q", synth)
	}
}

func TestFindAndExecute_WithSummarize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "summarized output"}},
			},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "pipelines.yaml")
	yaml := `
pipelines:
  - name: test_summarize
    description: Test summarize
    weight: 0.5
    action:
      type: bash
      script: "echo data to summarize"
      summarize: true
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(path)
	taskMgr := task.NewManager(t.TempDir(), nil)
	cfg := ai.Config{BaseURL: srv.URL, Model: "test-model", MaxTokens: 200, Temperature: 0.5}
	sum := summarizer.New(cfg)
	e := NewExecutor(mgr, sum, taskMgr, t.TempDir(), t.TempDir())

	raw, synth := e.FindAndExecute("EXECUTE_TEST_SUMMARIZE", nil, "user input", "")
	if raw != "data to summarize" {
		t.Fatalf("expected 'data to summarize', got %q", raw)
	}
	if synth != "summarized output" {
		t.Fatalf("expected 'summarized output', got %q", synth)
	}
}

func TestFindAndExecute_WithIgnoreContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "ignored context result"}},
			},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "pipelines.yaml")
	yaml := `
pipelines:
  - name: test_ignore_ctx
    description: Test ignore context
    weight: 0.5
    action:
      type: bash
      script: "echo data"
      summarize: true
      summarize_ignore_context: true
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(path)
	taskMgr := task.NewManager(t.TempDir(), nil)
	cfg := ai.Config{BaseURL: srv.URL, Model: "test-model", MaxTokens: 200, Temperature: 0.5}
	sum := summarizer.New(cfg)
	e := NewExecutor(mgr, sum, taskMgr, t.TempDir(), t.TempDir())

	raw, synth := e.FindAndExecute("EXECUTE_TEST_IGNORE_CTX", nil, "user input", "should be ignored")
	if raw != "data" {
		t.Fatalf("expected 'data', got %q", raw)
	}
	if synth != "ignored context result" {
		t.Fatalf("expected 'ignored context result', got %q", synth)
	}
}

func TestFindAndExecute_EmptyOutputFromAction(t *testing.T) {
	yaml := `
pipelines:
  - name: test_empty
    description: Test empty output
    weight: 0.5
    action:
      type: bash
      script: "echo -n ''"
`
	e := newExecutorWithYAML(t, yaml)
	raw, synth := e.FindAndExecute("EXECUTE_TEST_EMPTY", nil, "", "")
	if raw != "" || synth != "" {
		t.Fatalf("expected both empty for no-output action, got raw=%q synth=%q", raw, synth)
	}
}

// --- resolveParams edge cases ---

func TestValidate_MissingRequiredParam(t *testing.T) {
	e := newTestExecutor(t)
	defs := []ParamDefinition{
		{Name: "query", Required: true},
	}
	err := e.validate(defs, map[string]string{})
	if !strings.Contains(err, "Missing required parameter") {
		t.Fatalf("expected 'Missing required parameter', got %q", err)
	}
}

func TestValidate_ValidParams(t *testing.T) {
	e := newTestExecutor(t)
	defs := []ParamDefinition{
		{Name: "query", Required: true},
	}
	err := e.validate(defs, map[string]string{"query": "test"})
	if err != "" {
		t.Fatalf("expected no error, got %q", err)
	}
}

func TestResolveParams_DefaultValue(t *testing.T) {
	e := newTestExecutor(t)
	defs := []ParamDefinition{
		{Name: "action", Required: false, Default: "list"},
	}
	result := e.resolveParams(defs, map[string]string{}, "")
	if result["action"] != "list" {
		t.Fatalf("expected default 'list', got %q", result["action"])
	}
}

func TestResolveParams_RequiredWithUserInput(t *testing.T) {
	e := newTestExecutor(t)
	defs := []ParamDefinition{
		{Name: "query", Required: true},
	}
	result := e.resolveParams(defs, map[string]string{}, "user typed this")
	if result["query"] != "user typed this" {
		t.Fatalf("expected 'user typed this', got %q", result["query"])
	}
}

func TestResolveParams_InputOverridesDefault(t *testing.T) {
	e := newTestExecutor(t)
	defs := []ParamDefinition{
		{Name: "action", Required: false, Default: "list"},
	}
	result := e.resolveParams(defs, map[string]string{"action": "add"}, "")
	if result["action"] != "add" {
		t.Fatalf("expected 'add', got %q", result["action"])
	}
}

// --- Handler edge cases ---

func TestFindAndExecute_TODODoneAction(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "done"}, "", "")
	if !strings.Contains(raw, "Missing todo id") {
		t.Fatalf("expected 'Missing todo id', got %q", raw)
	}
}

func TestFindAndExecute_TODODoneWithInvalidID(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "done", "text": "nonexistent"}, "", "")
	if !strings.Contains(raw, "not found") {
		t.Fatalf("expected 'not found', got %q", raw)
	}
}

func TestFindAndExecute_TODOUncompleteAction(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "undone"}, "", "")
	if !strings.Contains(raw, "Missing todo id") {
		t.Fatalf("expected 'Missing todo id', got %q", raw)
	}
}

func TestFindAndExecute_ScheduleAddMissingParams(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{"action": "add"}, "", "")
	if !strings.Contains(raw, "when' and 'command'") {
		t.Fatalf("expected 'when' and 'command' message, got %q", raw)
	}
}

func TestFindAndExecute_ScheduleDeleteMissingID(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{"action": "delete"}, "", "")
	if !strings.Contains(raw, "requires 'schedule_id'") {
		t.Fatalf("expected 'requires schedule_id' message, got %q", raw)
	}
}

func TestFindAndExecute_HandlerUnknown(t *testing.T) {
	yaml := `
pipelines:
  - name: test_handler
    description: Test unknown handler
    weight: 0.5
    action:
      type: internal
      handler: nonexistent
`
	e := newExecutorWithYAML(t, yaml)
	raw, synth := e.FindAndExecute("EXECUTE_TEST_HANDLER", nil, "", "")
	if !strings.Contains(raw, "Unknown handler") {
		t.Fatalf("expected 'Unknown handler', got %q", raw)
	}
	if synth != raw {
		t.Fatalf("expected synth to match raw, got %q", synth)
	}
}

func TestListPipelines_WithCustomYAML(t *testing.T) {
	yaml := `
pipelines:
  - name: custom_one
    description: Custom 1
    weight: 0.5
    action:
      type: bash
      script: "echo 1"
  - name: custom_two
    description: Custom 2
    weight: 0.6
    action:
      type: bash
      script: "echo 2"
`
	e := newExecutorWithYAML(t, yaml)
	names := e.ListPipelines()
	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}
	if !found["custom_one"] {
		t.Fatal("expected 'custom_one' in pipeline list")
	}
	if !found["custom_two"] {
		t.Fatal("expected 'custom_two' in pipeline list")
	}
	if !found["todo"] {
		t.Fatal("expected 'todo' (default) in pipeline list")
	}
}

func TestNewManager_EmptyPath(t *testing.T) {
	m := NewManager("")
	if m.PipelineCount() != 0 {
		t.Fatalf("expected 0 pipelines for empty path, got %d", m.PipelineCount())
	}
}

// --- Handler edge cases: more todo and schedule branches ---

func TestFindAndExecute_TODOAddEmptyText(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "add", "text": ""}, "", "")
	if !strings.Contains(raw, "Missing text for todo") {
		t.Fatalf("expected 'Missing text for todo', got %q", raw)
	}
}

func TestFindAndExecute_TODOCompleteWithValidID(t *testing.T) {
	e := newTestExecutor(t)

	addRaw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "add", "text": "test complete"}, "", "")
	if !strings.Contains(addRaw, "Todo created") {
		t.Fatalf("failed to create todo: %s", addRaw)
	}

	id := extractID(addRaw)
	if id == "" {
		t.Fatal("could not extract id from response")
	}

	doneRaw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "complete", "text": id}, "", "")
	if !strings.Contains(doneRaw, "Marked todo as done") {
		t.Fatalf("expected 'Marked todo as done', got %q", doneRaw)
	}
}

func TestFindAndExecute_TODOUncompleteWithValidID(t *testing.T) {
	e := newTestExecutor(t)

	addRaw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "add", "text": "test uncomplete"}, "", "")
	id := extractID(addRaw)
	if id == "" {
		t.Fatal("could not extract id")
	}

	e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "done", "text": id}, "", "")
	undoneRaw, _ := e.FindAndExecute("EXECUTE_TODO", map[string]string{"action": "undone", "text": id}, "", "")
	if !strings.Contains(undoneRaw, "Marked todo as not done") {
		t.Fatalf("expected 'Marked todo as not done', got %q", undoneRaw)
	}
}

func TestFindAndExecute_ScheduleAddWithWhenCommand(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{
		"action":  "add",
		"when":    "in 1 hour",
		"command": "test command",
	}, "", "")
	if !strings.Contains(raw, "Scheduled") {
		t.Fatalf("expected 'Scheduled' message, got %q", raw)
	}
}

func TestFindAndExecute_ScheduleAddWithSchedulesJSON(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{
		"action":    "add",
		"schedules": `[{"when": "in 5 minutes", "command": "task1"}, {"when": "tomorrow", "command": "task2"}]`,
	}, "", "")
	if !strings.Contains(raw, "Created") {
		t.Fatalf("expected 'Created' message, got %q", raw)
	}
}

func TestFindAndExecute_ScheduleAddInvalidSchedulesJSON(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{
		"action":    "add",
		"schedules": "not-json",
	}, "", "")
	if !strings.Contains(raw, "Invalid schedules JSON") {
		t.Fatalf("expected 'Invalid schedules JSON', got %q", raw)
	}
}

func TestFindAndExecute_ScheduleDeleteWithValidID(t *testing.T) {
	e := newTestExecutor(t)

	addRaw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{
		"action":  "add",
		"when":    "in 2 hours",
		"command": "del test",
	}, "", "")

	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{
		"action":      "delete",
		"schedule_id": "nonexistent-id-12345",
	}, "", "")
	if !strings.Contains(raw, "Schedule not found") {
		t.Fatalf("expected 'Schedule not found', got %q", raw)
	}
	_ = addRaw
}

func TestFindAndExecute_ScheduleUpdate(t *testing.T) {
	e := newTestExecutor(t)

	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{
		"action":      "update",
		"schedule_id": "nonexistent-id",
		"command":     "new command",
	}, "", "")
	if !strings.Contains(raw, "Schedule not found") {
		t.Fatalf("expected 'Schedule not found', got %q", raw)
	}
}

func TestFindAndExecute_ScheduleDefaultAction(t *testing.T) {
	e := newTestExecutor(t)
	raw, _ := e.FindAndExecute("EXECUTE_SCHEDULE", map[string]string{"action": "invalid"}, "", "")
	if !strings.Contains(raw, "add, list, delete, update") {
		t.Fatalf("expected 'add, list, delete, update', got %q", raw)
	}
}

// --- executeHTTP with auth header ---

func TestExecuteHTTP_WithAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authenticated"))
	}))
	defer srv.Close()

	e := newTestExecutor(t)
	action := PipelineAction{
		Type:   "http",
		URL:    srv.URL,
		Method: "GET",
		Auth: &HTTPAuth{
			Header: "Authorization",
			Value:  "Bearer test-token",
		},
	}
	result := e.executeHTTP(action, map[string]string{})
	if !strings.Contains(result, "authenticated") {
		t.Fatalf("expected 'authenticated', got %q", result)
	}
}

func TestExecuteHTTP_WithInterpolatedAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("X-API-Key")
		if auth != "my-secret-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := newTestExecutor(t)
	action := PipelineAction{
		Type:   "http",
		URL:    srv.URL,
		Method: "GET",
		Auth: &HTTPAuth{
			Header: "X-API-Key",
			Value:  "{api_key}",
		},
	}
	result := e.executeHTTP(action, map[string]string{"api_key": "my-secret-key"})
	if !strings.Contains(result, "ok") {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

// --- Helper ---

func extractID(s string) string {
	idx := strings.LastIndex(s, "(id: ")
	if idx == -1 {
		return ""
	}
	end := strings.Index(s[idx+5:], ")")
	if end == -1 {
		return ""
	}
	return s[idx+5 : idx+5+end]
}
