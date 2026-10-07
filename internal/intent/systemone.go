package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/hrhrprasath/dhi/internal/ai"
	"github.com/hrhrprasath/dhi/internal/prompt"
)

func (r *Router) routeSystemOne(userInput string) (*ParsedIntent, error) {
	pipelines := prompt.LoadPipelineInfos(r.pipelinePath)
	criteria := prompt.BuildSystemOneCriteria(pipelines)

	req := ai.SystemOneRequest{
		Model: r.systemOneModel,
		State: userInput,
		Questions: map[string]ai.SystemOneQuestion{
			"intent": {
				Type:         "choice",
				Instructions: prompt.SystemOneInstructions,
				Criteria:     criteria,
			},
		},
	}

	resp, err := r.client.SystemOne(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("systemone route failed: %w", err)
	}

	ans, ok := resp.Answers["intent"]
	if !ok || ans.Choice == "" {
		return nil, fmt.Errorf("systemone returned no intent choice")
	}

	p := findPipeline(ans.Choice, pipelines)
	if !strings.EqualFold(ans.Choice, "CONVERSATION") && p == nil {
		return nil, fmt.Errorf("systemone returned unknown event: %s", ans.Choice)
	}

	parsed := &ParsedIntent{
		Event:      ans.Choice,
		Confidence: systemOneConfidence(ans),
	}

	if p != nil {
		parsed.Description = cleanCriteriaDescription(criteria[ans.Choice])
		if needsInput(p.Params) {
			if params, perr := r.extractParams(userInput, p); perr == nil {
				parsed.Params = params
			} else {
				r.logger.Debug("ROUTER", fmt.Sprintf("Param extraction skipped: %s", perr))
			}
		}
	}

	r.savePrompt(fmt.Sprintf("systemone request:\n%s\n\nresponse:\n%s",
		mustJSON(req), mustJSON(resp)))

	r.logger.Info("ROUTER", fmt.Sprintf("Intent via systemone: %s (confidence: %.2f)",
		parsed.Event, parsed.Confidence))
	return parsed, nil
}

func systemOneConfidence(ans ai.SystemOneAnswer) float64 {
	if len(ans.Probabilities) > 0 {
		if p, ok := ans.Probabilities["CONVERSATION"]; ok {
			if strings.EqualFold(ans.Choice, "CONVERSATION") {
				return clampConfidence(p)
			}
			return clampConfidence(1 - p)
		}
	}
	if ans.Confidence > 0 {
		return clampConfidence(ans.Confidence)
	}
	return 0.5
}

func clampConfidence(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func findPipeline(event string, pipelines []prompt.PipelineInfo) *prompt.PipelineInfo {
	if strings.EqualFold(event, "CONVERSATION") {
		return nil
	}
	name := strings.ToLower(strings.TrimPrefix(strings.ToUpper(event), "EXECUTE_"))
	for i := range pipelines {
		if strings.ToLower(pipelines[i].Name) == name {
			return &pipelines[i]
		}
	}
	return nil
}

func needsInput(params []prompt.ParamInfo) bool {
	for _, p := range params {
		if p.Required || p.Default == "" {
			return true
		}
	}
	return false
}

func cleanCriteriaDescription(desc string) string {
	if i := strings.Index(desc, " | "); i >= 0 {
		return desc[:i]
	}
	return desc
}

func (r *Router) extractParams(userInput string, p *prompt.PipelineInfo) (json.RawMessage, error) {
	resp, err := r.client.Chat(context.Background(), ai.ChatRequest{
		Model: r.model,
		Messages: []ai.ChatMessage{
			{Role: "user", Content: buildExtractPrompt(userInput, p)},
		},
		MaxTokens:       200,
		Temperature:     0,
		DisableThinking: true,
	})
	if err != nil {
		return nil, fmt.Errorf("extract request failed: %w", err)
	}
	return parseExtractedParams(resp, p.Params)
}

func buildExtractPrompt(userInput string, p *prompt.PipelineInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Extract parameters for the %q pipeline from the user input below.\n", p.Name)
	sb.WriteString("Params:\n")
	for _, param := range p.Params {
		s := "- " + param.Name
		if param.Required {
			s += " (required)"
		}
		if param.Default != "" {
			s += fmt.Sprintf(" (default: %s)", param.Default)
		}
		if param.ExtractHint != "" {
			s += " - " + param.ExtractHint
		}
		sb.WriteString(s + "\n")
	}
	sb.WriteString("\nReturn ONLY a JSON object mapping param names to values extracted from the input. " +
		"No explanation, no markdown. Omit params whose value is not present in the input.\n\n")
	fmt.Fprintf(&sb, "User input: %s\nJSON:", userInput)
	return sb.String()
}

func parseExtractedParams(raw string, defs []prompt.ParamInfo) (json.RawMessage, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object in extraction response: %s", raw)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(raw[start:end+1]), &m); err != nil {
		return nil, fmt.Errorf("invalid extraction JSON: %w", err)
	}

	valid := make(map[string]bool, len(defs))
	for _, d := range defs {
		valid[d.Name] = true
	}

	out := make(map[string]string)
	for k, v := range m {
		if !valid[k] {
			continue
		}
		switch val := v.(type) {
		case string:
			out[k] = val
		case map[string]any, []any:
			if b, err := json.Marshal(val); err == nil {
				out[k] = string(b)
			}
		default:
			out[k] = fmt.Sprintf("%v", val)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid params extracted")
	}
	return json.Marshal(out)
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
