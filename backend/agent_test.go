package backend

import "testing"

func TestNormalizeAgentSettingsDefaultsModel(t *testing.T) {
	settings := normalizeAgentSettings(AgentSettings{
		APIKey: "  test-key  ",
	})

	if settings.APIKey != "test-key" {
		t.Fatalf("expected trimmed API key, got %q", settings.APIKey)
	}
	if settings.Model != defaultAgentModel {
		t.Fatalf("expected default model %q, got %q", defaultAgentModel, settings.Model)
	}
	if settings.ReasoningEffort != "medium" {
		t.Fatalf("expected default reasoning effort %q, got %q", "medium", settings.ReasoningEffort)
	}
}

func TestNormalizeAgentRole(t *testing.T) {
	if got := normalizeAgentRole(" User "); got != "user" {
		t.Fatalf("expected user role, got %q", got)
	}
	if got := normalizeAgentRole("bogus"); got != "" {
		t.Fatalf("expected empty role for unknown value, got %q", got)
	}
}

func TestCombineInstructionsIncludesSystemPrompt(t *testing.T) {
	value := combineInstructions("Prefer concise answers.")
	if !strings.Contains(value, "Prefer concise answers.") {
		t.Fatalf("expected combined instructions to include custom system prompt")
	}
}
