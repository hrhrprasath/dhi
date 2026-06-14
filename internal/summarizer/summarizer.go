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
	options  map[string]any
}

func New(endpoint string) *Summarizer {
	return &Summarizer{
		client:   &http.Client{Timeout: 4 * time.Minute},
		model:    "",
		endpoint: endpoint,
	}
}

func (s *Summarizer) SetModel(model string) {
	s.model = model
}

func (s *Summarizer) SetContextSize(size int) {
	s.options = map[string]any{
		"num_ctx":     size,
		"num_predict": 1024,
	}
}

func (s *Summarizer) Summarize(rawData, pipeline, userInput, conversationContext string) string {
	p := prompt.SynthesisPrompt(rawData, pipeline, userInput, conversationContext)

	req := map[string]any{
		"model":   s.model,
		"prompt":  p,
		"stream":  false,
		"options": s.options,
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
