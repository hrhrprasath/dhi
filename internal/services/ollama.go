package services

import (
	"fmt"
	"net/http"
)

type OllamaService struct {
	endpoint string
	client   *http.Client
}

type OllamaStatus struct {
	Online bool
	Status string
}

func NewOllamaService(endpoint string) *OllamaService {
	return &OllamaService{
		endpoint: endpoint,
		client:   &http.Client{},
	}
}

func (o *OllamaService) HealthCheck() bool {
	resp, err := o.client.Get(fmt.Sprintf("%s/api/tags", o.endpoint))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (o *OllamaService) CheckStatus() OllamaStatus {
	resp, err := o.client.Get(fmt.Sprintf("%s/api/tags", o.endpoint))
	if err != nil {
		return OllamaStatus{Online: false, Status: "OFFLINE"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return OllamaStatus{Online: true, Status: "IDLE"}
	}
	return OllamaStatus{Online: false, Status: fmt.Sprintf("ERROR %d", resp.StatusCode)}
}
