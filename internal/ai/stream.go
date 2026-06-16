package ai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func parseSSEStream(resp *http.Response) (<-chan string, <-chan error) {
	tokenCh := make(chan string, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(tokenCh)
		defer close(errCh)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			var errResp ErrorResponse
			if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
				errCh <- fmt.Errorf("API error %d: %s", resp.StatusCode, errResp.Error.Message)
			} else {
				errCh <- fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				continue
			}

			if line == "data: [DONE]" {
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			data = strings.TrimSpace(data)

			if data == "" || data == "[DONE]" {
				return
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					tokenCh <- content
				}
				if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("stream read error: %w", err)
		}
	}()

	return tokenCh, errCh
}

func joinStream(resp *http.Response) (string, error) {
	tokenCh, errCh := parseSSEStream(resp)

	var sb strings.Builder
	var firstErr error

loop:
	for {
		select {
		case token, ok := <-tokenCh:
			if !ok {
				break loop
			}
			sb.WriteString(token)
		case err, ok := <-errCh:
			if ok && err != nil {
				firstErr = err
			}
		}
	}

	return sb.String(), firstErr
}
