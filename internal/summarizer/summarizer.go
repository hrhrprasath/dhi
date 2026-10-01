package summarizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/hrhrprasath/dhi/internal/ai"
	"github.com/hrhrprasath/dhi/internal/prompt"
)

type Summarizer struct {
	client *ai.Client
	model  string
	config ai.Config
}

func New(cfg ai.Config) *Summarizer {
	return &Summarizer{
		client: ai.NewClient(cfg),
		model:  cfg.Model,
		config: cfg,
	}
}

func (s *Summarizer) SetModel(model string) {
	s.model = model
}

func (s *Summarizer) Summarize(rawData, pipeline, userInput, conversationContext string) string {
	p := prompt.SynthesisPrompt(rawData, pipeline, userInput, conversationContext)

	messages := []ai.ChatMessage{
		{Role: "system", Content: p},
		{Role: "user", Content: userInput},
	}

	resp, err := s.client.Chat(context.Background(), ai.ChatRequest{
		Model:       s.model,
		Messages:    messages,
		MaxTokens:   s.config.MaxTokens,
		Temperature: s.config.Temperature,
	})
	if err != nil {
		return fmt.Sprintf("[Summary unavailable - %s]", err)
	}

	return strings.TrimSpace(resp)
}
