package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type WebhookConfig struct {
	Addr       string
	WebhookURL string
}

type webhookProjectsResponse struct {
	Name     string        `json:"name"`
	Projects []ProjectInfo `json:"projects"`
}

type webhookServicesResponse struct {
	Services []ServiceInfo `json:"services"`
}

type webhookResolveResponse struct {
	Cluster string `json:"cluster"`
	Address string `json:"address"`
}

type webhookAgentAuthResponse struct {
	Cluster string `json:"cluster"`
}

func (c *WebhookConfig) ValidToken(token string) bool {
	_, err := c.ClientProjects(token)
	return err == nil
}

func (c *WebhookConfig) ClientName(token string) string {
	resp, err := c.callWebhook("/projects", token)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var result webhookProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return result.Name
}

func (c *WebhookConfig) ClientProjects(token string) ([]ProjectInfo, error) {
	resp, err := c.callWebhook("/projects", token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var result webhookProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode webhook response: %w", err)
	}
	return result.Projects, nil
}

func (c *WebhookConfig) ClientServices(token, projectID string) ([]ServiceInfo, error) {
	resp, err := c.callWebhook("/services?project="+projectID, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var result webhookServicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode webhook response: %w", err)
	}
	return result.Services, nil
}

func (c *WebhookConfig) ResolveService(token, projectID, serviceID string) (ServiceRoute, error) {
	resp, err := c.callWebhook("/resolve?project="+projectID+"&service="+serviceID, token)
	if err != nil {
		return ServiceRoute{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ServiceRoute{}, fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return ServiceRoute{}, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var result webhookResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ServiceRoute{}, fmt.Errorf("failed to decode webhook response: %w", err)
	}
	if result.Address == "" {
		return ServiceRoute{}, fmt.Errorf("service not allowed: %s", serviceID)
	}
	return ServiceRoute{Cluster: result.Cluster, Address: result.Address}, nil
}

func (c *WebhookConfig) ValidAgentToken(token string) (string, bool) {
	resp, err := c.callWebhook("/agent-auth", token)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var result webhookAgentAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", false
	}
	return result.Cluster, result.Cluster != ""
}

func (c *WebhookConfig) callWebhook(path, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.WebhookURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	return resp, nil
}
