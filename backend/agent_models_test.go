package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchAgentModelsUsesBodylessGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("expected empty request body, got %q", string(body))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.4"},{"id":"gpt-5-mini"}]}`))
	}))
	defer server.Close()

	previousClient := agentModelsHTTPClient
	agentModelsHTTPClient = server.Client()
	t.Cleanup(func() {
		agentModelsHTTPClient = previousClient
	})

	items, err := fetchAgentModels(context.Background(), AgentSettings{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})
	if err != nil {
		t.Fatalf("fetchAgentModels returned error: %v", err)
	}
	if len(items) != 2 || items[0].ID != "gpt-5.4" || items[1].ID != "gpt-5-mini" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestFetchAgentModelsPropagatesAPIMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid request"}`))
	}))
	defer server.Close()

	previousClient := agentModelsHTTPClient
	agentModelsHTTPClient = server.Client()
	t.Cleanup(func() {
		agentModelsHTTPClient = previousClient
	})

	_, err := fetchAgentModels(context.Background(), AgentSettings{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid request") {
		t.Fatalf("expected propagated API error, got %v", err)
	}
}
