package config

import (
	"os"
	"sync"
)

var (
	ollamaEndpointOnce sync.Once
	ollamaEndpointVal  string
)

func OllamaEndpoint() string {
	ollamaEndpointOnce.Do(func() {
		if e := os.Getenv("DHI_OLLAMA_ENDPOINT"); e != "" {
			ollamaEndpointVal = e
		} else {
			ollamaEndpointVal = "http://localhost:11434"
		}
	})
	return ollamaEndpointVal
}
