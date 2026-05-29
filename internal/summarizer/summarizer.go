package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dhi_harness/internal/prompt"
)

type Summarizer struct {
	client   *http.Client
	model    string
	endpoint string
}

func New(model, endpoint string) *Summarizer {
	return &Summarizer{
		client:   &http.Client{Timeout: 30 * time.Second},
		model:    model,
		endpoint: endpoint,
	}
}

func (s *Summarizer) Summarize(rawData, pipeline, userInput, conversationContext string) string {
	p := prompt.SynthesisPrompt(rawData, pipeline, userInput, conversationContext)

	req := map[string]interface{}{
		"model":  s.model,
		"prompt": p,
		"stream": false,
	}

	data, _ := json.Marshal(req)
	resp, err := s.client.Post(
		fmt.Sprintf("%s/api/generate", s.endpoint),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Sprintf("[Summary unavailable - %s]", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("[Summary unavailable - API returned status %d]", resp.StatusCode)
	}

	var ollamaResp struct {
		Response string `json:"response"`
	}
	body, _ := io.ReadAll(resp.Body)
	if json.Unmarshal(body, &ollamaResp) != nil {
		return "[Summary unavailable - failed to parse response]"
	}

	return ollamaResp.Response
}
