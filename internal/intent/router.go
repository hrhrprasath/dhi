package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dhi_harness/internal/ai"
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
	BuildWeightLine(pipelineName, userInput string, baseWeight float64) string
	BuildCalibrationBlock() string
	BuildKeywordBlock(userInput string, pipelineKeywords map[string][]string) string
	CalibrateWithEvent(rawConfidence float64, event string) float64
}

type Router struct {
	client       *ai.Client
	model        string
	config       ai.Config
	logger       *services.Logger
	weights      WeightProvider
	dataDir      string
	pipelinePath string
}

func NewRouter(cfg ai.Config, logger *services.Logger, dataDir, pipelinePath string) *Router {
	logger.Info("ROUTER", fmt.Sprintf("AI Router initialized (provider: %s)", cfg.Provider))
	return &Router{
		client:       ai.NewClient(cfg),
		model:        cfg.Model,
		config:       cfg,
		logger:       logger,
		dataDir:      dataDir,
		pipelinePath: pipelinePath,
	}
}

func (r *Router) SetModel(model string) {
	r.model = model
}

func (r *Router) SetWeightProvider(wp WeightProvider) {
	r.weights = wp
}

func (r *Router) buildWeightLines(userInput string) map[string]string {
	if r.weights == nil {
		return nil
	}
	pipelines := prompt.LoadPipelineInfos(r.pipelinePath)
	lines := make(map[string]string, len(pipelines))
	for _, p := range pipelines {
		wl := r.weights.BuildWeightLine(p.Name, userInput, p.Weight)
		if wl != "" {
			lines[strings.ToLower(p.Name)] = wl
		}
	}
	return lines
}

func (r *Router) Route(userInput, summaryText, topicTitle string) (*ParsedIntent, error) {
	r.logger.Info("ROUTER", fmt.Sprintf("User input: %s", userInput))

	calBlock := ""
	keywordBlock := ""
	weightLines := r.buildWeightLines(userInput)
	if r.weights != nil {
		calBlock = r.weights.BuildCalibrationBlock()
		pipelines := prompt.LoadPipelineInfos(r.pipelinePath)
		pipelineKeywords := make(map[string][]string)
		for _, p := range pipelines {
			if len(p.Keywords) > 0 {
				pipelineKeywords[strings.ToLower(p.Name)] = p.Keywords
			}
		}
		keywordBlock = r.weights.BuildKeywordBlock(userInput, pipelineKeywords)
	}

	systemPrompt := prompt.ClassifyPrompt(userInput, summaryText, topicTitle, calBlock, keywordBlock, weightLines, r.pipelinePath)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userInput},
	}

	resp, err := r.client.Chat(context.Background(), ai.ChatRequest{
		Model:       r.model,
		Messages:    messages,
		MaxTokens:   r.config.MaxTokens,
		Temperature: r.config.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("route request failed: %w", err)
	}

	r.savePrompt(systemPrompt + "\n\n---\n\n" + resp)

	r.logger.Debug("ROUTER", fmt.Sprintf("Raw response:\n%s", resp))

	parsed, err := parseJSON(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}

	if r.weights != nil {
		original := parsed.Confidence
		parsed.Confidence = r.weights.CalibrateWithEvent(parsed.Confidence, parsed.Event)
		if parsed.Confidence != original {
			r.logger.Info("ROUTER", fmt.Sprintf("Calibrated confidence: %.2f \u2192 %.2f", original, parsed.Confidence))
		}
	}

	r.logger.Info("ROUTER", fmt.Sprintf("Intent: %s (confidence: %.2f)", parsed.Event, parsed.Confidence))
	return parsed, nil
}

func (r *Router) GenerateTitle(userInput string) (string, error) {
	titlePrompt := fmt.Sprintf(
		"Extract 3-4 key words from the query below to form a concise title. "+
			"Return ONLY the title, nothing else.\n\nQuery: %s\n\nTitle:", userInput)

	messages := []ai.ChatMessage{
		{Role: "user", Content: titlePrompt},
	}

	resp, err := r.client.Chat(context.Background(), ai.ChatRequest{
		Model:       r.model,
		Messages:    messages,
		MaxTokens:   50,
		Temperature: 0.3,
	})
	if err != nil {
		return "", fmt.Errorf("title generation request failed: %w", err)
	}

	return strings.TrimSpace(resp), nil
}

func (r *Router) Chat(userInput, summaryText, searchResults, topicTitle string) (string, error) {
	r.logger.Info("ROUTER", fmt.Sprintf("Chat generation for: %s", userInput))

	convCtx := ""
	if searchResults != "" {
		convCtx = "\n\n" + searchResults
	}
	systemContent := fmt.Sprintf(`%s

%s

CONVERSATION HISTORY:
%s%s

TOPIC: %s`,
		prompt.GuardrailPrefix, prompt.Persona, summaryText, convCtx, topicTitle)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: userInput},
	}

	resp, err := r.client.Chat(context.Background(), ai.ChatRequest{
		Model:       r.model,
		Messages:    messages,
		MaxTokens:   r.config.MaxTokens,
		Temperature: r.config.Temperature,
	})
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}

	r.logger.Debug("ROUTER", fmt.Sprintf("Chat response: %d chars", len(resp)))
	return strings.TrimSpace(resp), nil
}

func (r *Router) ChatStream(userInput, summaryText, searchResults, topicTitle string) (<-chan string, <-chan error) {
	r.logger.Info("ROUTER", fmt.Sprintf("Chat stream for: %s", userInput))

	convCtx := ""
	if searchResults != "" {
		convCtx = "\n\n" + searchResults
	}
	systemContent := fmt.Sprintf(`%s

%s

CONVERSATION HISTORY:
%s%s

TOPIC: %s`,
		prompt.GuardrailPrefix, prompt.Persona, summaryText, convCtx, topicTitle)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: userInput},
	}

	return r.client.ChatStream(context.Background(), ai.ChatRequest{
		Model:       r.model,
		Messages:    messages,
		MaxTokens:   r.config.MaxTokens,
		Temperature: r.config.Temperature,
	})
}

func (r *Router) HealthCheck() bool {
	return r.client.HealthCheck(context.Background())
}

func (r *Router) savePrompt(p string) {
	dir := filepath.Join(r.dataDir, "intent")
	_ = os.MkdirAll(dir, 0755)
	filename := time.Now().Format("2006-01-02T15-04-05") + ".md"
	_ = os.WriteFile(filepath.Join(dir, filename), []byte("# Intent Prompt\n\n"+p), 0644)
}

func parseJSON(raw string) (*ParsedIntent, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response: %s", raw)
	}
	trimmed := raw[start : end+1]

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
