package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func normalizeAgentSettings(settings AgentSettings) AgentSettings {
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.Model = strings.TrimSpace(settings.Model)
	if settings.Model == "" {
		settings.Model = defaultAgentModel
	}
	settings.ReasoningEffort = normalizeReasoningEffort(settings.ReasoningEffort)
	if settings.ReasoningEffort == "" {
		settings.ReasoningEffort = "medium"
	}
	settings.SystemPrompt = strings.TrimSpace(settings.SystemPrompt)
	if settings.SystemPrompt == "" {
		settings.SystemPrompt = defaultAgentSystemPrompt
	}
	return settings
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none", "minimal", "low", "medium", "high", "xhigh":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func (s *Service) loadAgentSettings() (AgentSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := agentSettingsPath()
	if err != nil {
		return AgentSettings{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AgentSettings{Model: defaultAgentModel}, nil
		}
		return AgentSettings{}, err
	}

	var settings AgentSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return AgentSettings{}, err
	}
	return normalizeAgentSettings(settings), nil
}

func (s *Service) saveAgentSettings(settings AgentSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := agentSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(normalizeAgentSettings(settings), "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func agentSettingsPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "gutgd", "settings.json"), nil
}
