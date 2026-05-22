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

type webhookServicesResponse struct {
	Name     string        `json:"name"`
	Services []ServiceInfo `json:"services"`
}

type webhookResolveResponse struct {
	Address string `json:"address"`
}

func (c *WebhookConfig) ValidToken(token string) bool {
	_, err := c.ClientServices(token)
	return err == nil
}

func (c *WebhookConfig) ClientName(token string) string {
	resp, err := c.callWebhook("/services", token)
	if err != nil {
		return ""
	}
	var result webhookServicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return result.Name
}

func (c *WebhookConfig) ClientServices(token string) ([]ServiceInfo, error) {
	resp, err := c.callWebhook("/services", token)
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

func (c *WebhookConfig) ResolveService(token, serviceID string) (string, error) {
	resp, err := c.callWebhook("/resolve?service="+serviceID, token)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var result webhookResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode webhook response: %w", err)
	}
	if result.Address == "" {
		return "", fmt.Errorf("service not allowed: %s", serviceID)
	}
	return result.Address, nil
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
