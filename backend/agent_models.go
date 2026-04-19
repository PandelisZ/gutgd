package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultAgentBaseURL = "https://api.openai.com/v1/"

var agentModelsHTTPClient = &http.Client{Timeout: 30 * time.Second}

type agentModelsListResponse struct {
	Data []AgentModelOption `json:"data"`
}

type agentModelsErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

func fetchAgentModels(ctx context.Context, settings AgentSettings) ([]AgentModelOption, error) {
	endpoint, err := agentModelsEndpoint(settings.BaseURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.APIKey)

	res, err := agentModelsHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return nil, agentModelsHTTPError(endpoint, res.StatusCode, body)
	}

	var payload agentModelsListResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func agentModelsEndpoint(baseURL string) (string, error) {
	value := strings.TrimSpace(baseURL)
	if value == "" {
		value = defaultAgentBaseURL
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid agent base url %q: %w", value, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid agent base url %q", value)
	}
	if parsed.Path == "" || !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.ResolveReference(&url.URL{Path: "models"}).String(), nil
}

func agentModelsHTTPError(endpoint string, statusCode int, body []byte) error {
	var payload agentModelsErrorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		message := strings.TrimSpace(payload.Error.Message)
		if message == "" {
			message = strings.TrimSpace(payload.Message)
		}
		if message != "" {
			return fmt.Errorf("GET %q: %d %s", endpoint, statusCode, message)
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Errorf("GET %q: %d", endpoint, statusCode)
	}
	return fmt.Errorf("GET %q: %d %s", endpoint, statusCode, text)
}
