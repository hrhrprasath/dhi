package intent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dhi_harness/internal/prompt"
	"dhi_harness/internal/services"
)

type ParsedIntent struct {
	Event       string          `json:"event"`
	Params      json.RawMessage `json:"params,omitempty"`
	Description string          `json:"description,omitempty"`
	Confidence  float64         `json:"confidence"`
}

type WeightProvider interface {
	BuildWeightLine(pipelineName string, baseWeight float64) string
	BuildCalibrationBlock() string
	BuildKeywordBlock(userInput string, pipelineKeywords map[string][]string) string
	CalibrateWithEvent(rawConfidence float64, event string) float64
}

type intentRequest struct {
	Model  string          `json:"model"`
	Prompt string          `json:"prompt"`
	Stream bool            `json:"stream"`
	Format json.RawMessage `json:"format,omitempty"`
}

type intentResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type Router struct {
	client   *http.Client
	model    string
	endpoint string
	logger   *services.Logger
	weights  WeightProvider
}

func NewRouter(model, endpoint string, logger *services.Logger) *Router {
	logger.Info("ROUTER", "Intent Router initialized")
	return &Router{
		client:   &http.Client{Timeout: 30 * time.Second},
		model:    model,
		endpoint: endpoint,
		logger:   logger,
	}
}

func (r *Router) SetWeightProvider(wp WeightProvider) {
	r.weights = wp
}

func (r *Router) buildWeightLines() map[string]string {
	if r.weights == nil {
		return nil
	}
	pipelines := prompt.LoadPipelineInfos()
	lines := make(map[string]string, len(pipelines))
	for _, p := range pipelines {
		wl := r.weights.BuildWeightLine(p.Name, p.Weight)
		if wl != "" {
			lines[strings.ToLower(p.Name)] = wl
		}
	}
	return lines
}

func (r *Router) Route(userInput, summaryText string) (*ParsedIntent, error) {
	r.logger.Info("ROUTER", fmt.Sprintf("User input: %s", userInput))

	calBlock := ""
	keywordBlock := ""
	weightLines := r.buildWeightLines()
	if r.weights != nil {
		calBlock = r.weights.BuildCalibrationBlock()
		pipelines := prompt.LoadPipelineInfos()
		pipelineKeywords := make(map[string][]string)
		for _, p := range pipelines {
			if len(p.Keywords) > 0 {
				pipelineKeywords[strings.ToLower(p.Name)] = p.Keywords
			}
		}
		keywordBlock = r.weights.BuildKeywordBlock(userInput, pipelineKeywords)
	}

	p := prompt.ClassifyPrompt(userInput, summaryText, calBlock, keywordBlock, weightLines)
	r.savePrompt(p)

	req := intentRequest{
		Model:  r.model,
		Prompt: p,
		Stream: false,
		Format: json.RawMessage(`{"type": "object"}`),
	}

	resp, err := r.client.Post(
		fmt.Sprintf("%s/api/generate", r.endpoint),
		"application/json",
		toReader(req),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp intentResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	r.logger.Debug("ROUTER", fmt.Sprintf("Ollama raw response:\n%s", ollamaResp.Response))

	parsed, err := parseJSON(ollamaResp.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}

	if r.weights != nil {
		original := parsed.Confidence
		parsed.Confidence = r.weights.CalibrateWithEvent(parsed.Confidence, parsed.Event)
		if parsed.Confidence != original {
			r.logger.Info("ROUTER", fmt.Sprintf("Calibrated confidence: %.2f → %.2f", original, parsed.Confidence))
		}
	}

	r.logger.Info("ROUTER", fmt.Sprintf("Intent: %s (confidence: %.2f)", parsed.Event, parsed.Confidence))
	return parsed, nil
}

func (r *Router) Chat(userInput, summaryText, searchResults string) (string, error) {
	r.logger.Info("ROUTER", fmt.Sprintf("Chat generation for: %s", userInput))
	p := prompt.ConversationPrompt(userInput, summaryText, searchResults)

	req := intentRequest{
		Model:  r.model,
		Prompt: p,
		Stream: false,
	}

	resp, err := r.client.Post(
		fmt.Sprintf("%s/api/generate", r.endpoint),
		"application/json",
		toReader(req),
	)
	if err != nil {
		return "", fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp intentResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode chat response: %w", err)
	}

	r.logger.Debug("ROUTER", fmt.Sprintf("Chat response: %d chars", len(ollamaResp.Response)))
	return strings.TrimSpace(ollamaResp.Response), nil
}

func (r *Router) HealthCheck() bool {
	resp, err := r.client.Get(fmt.Sprintf("%s/api/tags", r.endpoint))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (r *Router) savePrompt(p string) {
	dir := filepath.Join(".data", "intent")
	os.MkdirAll(dir, 0755)
	filename := time.Now().Format("2006-01-02T15-04-05") + ".md"
	os.WriteFile(filepath.Join(dir, filename), []byte("# Intent Prompt\n\n"+p), 0644)
}

func parseJSON(raw string) (*ParsedIntent, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var parsed ParsedIntent
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w - Response: %s", err, raw)
	}
	if parsed.Event == "" {
		return nil, fmt.Errorf("missing 'event' field in: %s", raw)
	}
	if parsed.Confidence == 0 {
		parsed.Confidence = 0.5
	}
	return &parsed, nil
}

func toReader(v any) io.Reader {
	data, _ := json.Marshal(v)
	return bytes.NewReader(data)
}
