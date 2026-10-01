package pipeline

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hrhrprasath/dhi/internal/summarizer"
	"github.com/hrhrprasath/dhi/internal/task"
	"github.com/hrhrprasath/dhi/internal/todo"
)

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

type Executor struct {
	manager    *Manager
	summarizer *summarizer.Summarizer
	handlers   *Handlers
	dataDir    string
	scriptsDir string
}

func NewExecutor(manager *Manager, sum *summarizer.Summarizer, taskMgr *task.Manager, dataDir, scriptsDir string) *Executor {
	return &Executor{
		manager:    manager,
		summarizer: sum,
		handlers:   NewHandlers(todo.NewManager(dataDir), taskMgr),
		dataDir:    dataDir,
		scriptsDir: scriptsDir,
	}
}

func (e *Executor) SetModel(name string) {
	e.summarizer.SetModel(name)
}

func (e *Executor) FindAndExecute(event string, params map[string]string, userInput, convCtx string) (raw string, synthesized string) {
	pipelineName := e.eventToPipeline(event)
	pl := e.manager.GetPipeline(pipelineName)
	if pl == nil {
		return "", "Unable to determine action: event unknown"
	}

	resolved := e.resolveParams(pl.Params, params, userInput)
	if err := e.validate(pl.Params, resolved); err != "" {
		return "", "Parameter validation failed: " + err
	}

	output := e.executeAction(pl, resolved)
	if output == "" {
		return "", output
	}

	if pl.Action.Summarize {
		ctx := convCtx
		if pl.Action.IgnoreContext {
			ctx = ""
		}
		synth := e.synthesizeOutput(output, pl.Name, userInput, ctx)
		return output, synth
	}
	return output, output
}

func (e *Executor) ListPipelines() []string {
	var names []string
	for _, p := range e.manager.ListPipelines() {
		names = append(names, p.Name)
	}
	return names
}

func (e *Executor) eventToPipeline(event string) string {
	upper := strings.ToUpper(event)
	if strings.HasPrefix(upper, "EXECUTE_") {
		name := strings.ToLower(strings.TrimPrefix(upper, "EXECUTE_"))
		if e.manager.GetPipeline(name) != nil {
			return name
		}
	}
	return "web_search"
}

func (e *Executor) validate(defs []ParamDefinition, params map[string]string) string {
	valid := make(map[string]bool)
	for _, d := range defs {
		valid[d.Name] = true
	}
	for k := range params {
		if !valid[k] {
			return fmt.Sprintf("Invalid parameter '%s'", k)
		}
	}
	for _, d := range defs {
		if d.Required {
			if v, ok := params[d.Name]; !ok || v == "" {
				return fmt.Sprintf("Missing required parameter: %s", d.Name)
			}
		}
	}
	return ""
}

func (e *Executor) resolveParams(defs []ParamDefinition, input map[string]string, userInput string) map[string]string {
	result := make(map[string]string)
	for k, v := range input {
		result[k] = v
	}
	for _, d := range defs {
		if _, ok := result[d.Name]; !ok {
			if d.Default != "" {
				result[d.Name] = d.Default
			} else if d.Required && userInput != "" {
				result[d.Name] = userInput
			}
		}
	}
	return result
}

func (e *Executor) executeAction(pl *Pipeline, params map[string]string) string {
	switch pl.Action.Type {
	case "http":
		return e.executeHTTP(pl.Action, params)
	case "exec":
		return e.executeExec(pl.Action, params)
	case "bash":
		return e.executeBash(pl.Action, params)
	case "internal":
		return e.handlers.Execute(pl.Action.Handler, params)
	default:
		return fmt.Sprintf("Unknown action type: %s", pl.Action.Type)
	}
}

func (e *Executor) executeHTTP(action PipelineAction, params map[string]string) string {
	url := e.interpolate(action.URL, params)
	method := e.interpolate(action.Method, params)

	args := []string{"-s", "--max-time", "10", "-X", method}
	if action.Auth != nil {
		h := e.interpolate(action.Auth.Header, params)
		v := e.interpolate(action.Auth.Value, params)
		args = append(args, "-H", fmt.Sprintf("%s: %s", h, v))
	}
	args = append(args, url)

	out, err := runCmd("curl", args...)
	if err != nil {
		return fmt.Sprintf("HTTP request failed: %s", err)
	}
	return out
}

func (e *Executor) executeExec(action PipelineAction, params map[string]string) string {
	cmd := e.interpolate(action.Command, params)
	var args []string
	for _, a := range action.Args {
		args = append(args, e.interpolate(a, params))
	}
	out, err := runCmd(cmd, args...)
	if err != nil {
		return fmt.Sprintf("Command failed: %s", err)
	}
	return out
}

func (e *Executor) executeBash(action PipelineAction, params map[string]string) string {
	script := e.interpolate(action.Script, params)
	script = e.resolveScript(script)
	out, err := runCmd("bash", "-c", script)
	if err != nil {
		return fmt.Sprintf("Script failed: %s", err)
	}
	return out
}

func (e *Executor) resolveScript(script string) string {
	fields := strings.Fields(script)
	if len(fields) == 0 {
		return script
	}
	if strings.HasPrefix(fields[0], "scripts/") {
		rel := fields[0][len("scripts/"):]
		fields[0] = filepath.Join(e.scriptsDir, rel)
		return strings.Join(fields, " ")
	}
	return script
}

func (e *Executor) interpolate(template string, params map[string]string) string {
	result := template
	for k, v := range params {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

func (e *Executor) Summarizer() *summarizer.Summarizer {
	return e.summarizer
}

func (e *Executor) synthesizeOutput(raw, pipeline, userInput, convCtx string) string {
	return e.summarizer.Summarize(raw, pipeline, userInput, convCtx)
}
