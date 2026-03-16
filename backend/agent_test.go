package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/responses"
)

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

func TestTranscriptItemsFromResponseOutputIncludesReasoningAndMessages(t *testing.T) {
	reasoning := mustResponseOutputItem(t, `{
		"type":"reasoning",
		"id":"rs_1",
		"summary":[{"type":"summary_text","text":"Need to find Slack first."}]
	}`)
	message := mustResponseOutputItem(t, `{
		"type":"message",
		"id":"msg_1",
		"role":"assistant",
		"status":"completed",
		"content":[{"type":"output_text","text":"I found the Slack window."}]
	}`)
	reasoningItems := transcriptItemsFromResponseOutput(reasoning)
	if len(reasoningItems) != 1 || reasoningItems[0].Kind != "reasoning" || reasoningItems[0].Content != "Need to find Slack first." {
		t.Fatalf("unexpected reasoning transcript items: %+v", reasoningItems)
	}

	messageItems := transcriptItemsFromResponseOutput(message)
	if len(messageItems) != 1 || messageItems[0].Kind != "message" || messageItems[0].Content != "I found the Slack window." {
		t.Fatalf("unexpected message transcript items: %+v", messageItems)
	}
}

func TestTranscriptItemsFromResponseOutputSkipsFunctionCalls(t *testing.T) {
	toolCall := mustResponseOutputItem(t, `{
		"type":"function_call",
		"id":"fc_1",
		"call_id":"call_1",
		"name":"find_window_by_title",
		"arguments":"{\"title\":\"Slack\",\"use_regex\":true}",
		"status":"completed"
	}`)

	toolCallItems := transcriptItemsFromResponseOutput(toolCall)
	if len(toolCallItems) != 0 {
		t.Fatalf("expected function calls to be handled only by the explicit tool-call path, got %+v", toolCallItems)
	}
}

func TestResponseOutputToInputItemPreservesReasoningAndFunctionCalls(t *testing.T) {
	reasoning := mustResponseOutputItem(t, `{
		"type":"reasoning",
		"id":"rs_1",
		"summary":[{"type":"summary_text","text":"Plan the next click."}],
		"encrypted_content":"encrypted-reasoning"
	}`)
	toolCall := mustResponseOutputItem(t, `{
		"type":"function_call",
		"id":"fc_1",
		"call_id":"call_1",
		"name":"activate_window",
		"arguments":"{\"handle\":3560}",
		"status":"completed"
	}`)

	reasoningInput, ok := responseOutputToInputItem(reasoning)
	if !ok {
		t.Fatalf("expected reasoning output to convert into an input item")
	}
	reasoningJSON := marshalJSON(t, reasoningInput)
	if !strings.Contains(reasoningJSON, `"type":"reasoning"`) || !strings.Contains(reasoningJSON, `"encrypted_content":"encrypted-reasoning"`) {
		t.Fatalf("expected reasoning input item to preserve reasoning payload, got %s", reasoningJSON)
	}

	toolCallInput, ok := responseOutputToInputItem(toolCall)
	if !ok {
		t.Fatalf("expected tool call output to convert into an input item")
	}
	toolCallJSON := marshalJSON(t, toolCallInput)
	if !strings.Contains(toolCallJSON, `"type":"function_call"`) || !strings.Contains(toolCallJSON, `"call_id":"call_1"`) || !strings.Contains(toolCallJSON, `"name":"activate_window"`) {
		t.Fatalf("expected function call input item to preserve tool call payload, got %s", toolCallJSON)
	}
}

func TestAccumulateAgentUsageAddsAcrossSteps(t *testing.T) {
	total := AgentUsage{}
	accumulateAgentUsage(&total, &responses.Response{
		Usage: responses.ResponseUsage{
			InputTokens:  10,
			OutputTokens: 4,
			OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{
				ReasoningTokens: 2,
			},
			TotalTokens: 14,
		},
	})
	accumulateAgentUsage(&total, &responses.Response{
		Usage: responses.ResponseUsage{
			InputTokens:  6,
			OutputTokens: 3,
			OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{
				ReasoningTokens: 1,
			},
			TotalTokens: 9,
		},
	})

	if total.InputTokens != 16 || total.OutputTokens != 7 || total.ReasoningTokens != 3 || total.TotalTokens != 23 {
		t.Fatalf("unexpected accumulated usage: %+v", total)
	}
}

func TestAgentToolsExposeSingleCallTextEntryTools(t *testing.T) {
	service := NewService()
	tools := service.agentToolsForGOOS("linux")

	foundTypeText := false
	foundTypeTextBlock := false
	for _, tool := range tools {
		switch tool.Name {
		case "type_text":
			foundTypeText = true
			if !strings.Contains(tool.Description, "sentences") {
				t.Fatalf("expected type_text description to mention sentence-level text entry, got %q", tool.Description)
			}
		case "type_text_block":
			foundTypeTextBlock = true
			if !strings.Contains(tool.Description, "single tool call") {
				t.Fatalf("expected type_text_block description to mention single tool call text entry, got %q", tool.Description)
			}
		}
	}

	if !foundTypeText {
		t.Fatal("expected type_text to be exposed")
	}
	if !foundTypeTextBlock {
		t.Fatal("expected type_text_block to be exposed")
	}
}

func TestAgentToolsMatchPlatformAvailability(t *testing.T) {
	service := NewService()

	darwinTools := service.agentToolsForGOOS("darwin")
	if !hasAgentTool(darwinTools, "press_special_key") {
		t.Fatal("expected press_special_key to be exposed on darwin")
	}
	for _, hidden := range []string{"tap_keys", "press_keys", "release_keys", "highlight_region"} {
		if hasAgentTool(darwinTools, hidden) {
			t.Fatalf("expected %s to be hidden on darwin", hidden)
		}
	}

	linuxTools := service.agentToolsForGOOS("linux")
	if hasAgentTool(linuxTools, "press_special_key") {
		t.Fatal("expected press_special_key to be hidden off darwin")
	}
	for _, exposed := range []string{"tap_keys", "press_keys", "release_keys", "highlight_region"} {
		if !hasAgentTool(linuxTools, exposed) {
			t.Fatalf("expected %s to be exposed off darwin", exposed)
		}
	}
}

func TestAgentDeveloperPromptForDarwinMentionsSafeFallbacks(t *testing.T) {
	prompt := agentDeveloperPromptForGOOS("darwin")
	for _, needle := range []string{
		"macOS 26+",
		"capture_screen and capture_region use the safe OS screenshot fallback",
		"Prefer press_special_key",
		"tap_keys, press_keys, release_keys, and highlight_region are intentionally unavailable",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected darwin prompt to contain %q, got %q", needle, prompt)
		}
	}

	linuxPrompt := agentDeveloperPromptForGOOS("linux")
	if strings.Contains(linuxPrompt, "intentionally unavailable on macOS 26+") {
		t.Fatalf("expected non-darwin prompt to omit darwin-only warning, got %q", linuxPrompt)
	}
}

func TestContinueInputItemsAppendsSyntheticContinueMessage(t *testing.T) {
	outputs := []responses.ResponseInputItemUnionParam{
		responses.ResponseInputItemParamOfFunctionCallOutput("call_1", `{"ok":true}`),
	}

	items := continueInputItems(outputs)
	if len(items) != 2 {
		t.Fatalf("expected one tool output plus one continue message, got %d items", len(items))
	}

	payload := marshalJSON(t, items[1])
	if !strings.Contains(payload, `"role":"user"`) || !strings.Contains(payload, `"content":"continue"`) {
		t.Fatalf("expected synthetic continue user message, got %s", payload)
	}
}

func TestAppendAssistantAfterTrailingContinueMovesContinueToEnd(t *testing.T) {
	items := []AgentTranscriptItem{
		{Kind: "tool_output", Name: "capture_region", CallID: "call_1", Output: `{"ok":true}`},
		{Kind: "message", Role: "user", Content: continueMessage},
	}

	got := appendAssistantAfterTrailingContinue(items, AgentTranscriptItem{
		Kind:    "message",
		Role:    "assistant",
		Content: "The agent reached the maximum tool-call depth before producing a final answer. Review the tool activity above and refine the request if needed.",
	})

	if len(got) != 3 {
		t.Fatalf("expected 3 transcript items, got %d", len(got))
	}
	if got[1].Role != "assistant" {
		t.Fatalf("expected assistant message before trailing continue, got %+v", got)
	}
	if got[2].Role != "user" || got[2].Content != continueMessage {
		t.Fatalf("expected trailing continue message at end, got %+v", got)
	}
}

func TestAppendAssistantAfterTrailingContinueLeavesTranscriptUntouchedWithoutTrailingContinue(t *testing.T) {
	items := []AgentTranscriptItem{
		{Kind: "message", Role: "assistant", Content: "Done."},
	}

	got := appendAssistantAfterTrailingContinue(items, AgentTranscriptItem{
		Kind:    "message",
		Role:    "assistant",
		Content: "The agent reached the maximum tool-call depth before producing a final answer. Review the tool activity above and refine the request if needed.",
	})

	if len(got) != 2 {
		t.Fatalf("expected assistant message to append normally, got %+v", got)
	}
	if got[0].Content != "Done." || got[1].Role != "assistant" {
		t.Fatalf("unexpected transcript ordering: %+v", got)
	}
}

func TestAgentFunctionCallOutputAttachesScreenshotAsImageInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	output := marshalToolPayload(CaptureResult{
		Path:    path,
		Message: "Saved capture",
		Offset:  Point{X: 2860, Y: 1510},
		Scale:   Scale{X: 2, Y: 2},
	})

	item := agentFunctionCallOutput("call_capture", output)
	payload := marshalJSON(t, item)
	if !strings.Contains(payload, `"type":"function_call_output"`) {
		t.Fatalf("expected function_call_output payload, got %s", payload)
	}
	if !strings.Contains(payload, `"type":"input_image"`) {
		t.Fatalf("expected screenshot output to attach input_image, got %s", payload)
	}
	if !strings.Contains(payload, `"image_url":"data:image/png;base64,`) {
		t.Fatalf("expected screenshot output to use a data URL, got %s", payload)
	}
	if !strings.Contains(payload, `"type":"input_text"`) {
		t.Fatalf("expected screenshot output to preserve textual tool output, got %s", payload)
	}
	if !strings.Contains(payload, `screen offset: (2860, 1510)`) {
		t.Fatalf("expected screenshot output text payload to preserve offset metadata, got %s", payload)
	}
	if !strings.Contains(payload, `screenshot scale: x=2.0000, y=2.0000`) {
		t.Fatalf("expected screenshot output text payload to preserve scale metadata, got %s", payload)
	}
	if !strings.Contains(payload, `screen_x = 2860 + image_x / 2.0000`) || !strings.Contains(payload, `screen_y = 1510 + image_y / 2.0000`) {
		t.Fatalf("expected screenshot output text payload to include coordinate conversion guidance, got %s", payload)
	}
}

func TestAgentToolsExposeAnalyzeScreenshot(t *testing.T) {
	service := NewService()
	tool, ok := findAgentTool(service.agentToolsForGOOS("darwin"), "analyze_screenshot")
	if !ok {
		t.Fatal("expected analyze_screenshot to be exposed")
	}
	if !strings.Contains(tool.Description, "screen coordinates") {
		t.Fatalf("expected analyze_screenshot description to mention screen coordinates, got %q", tool.Description)
	}

	required, ok := tool.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("expected analyze_screenshot required fields to be []string, got %#v", tool.Parameters["required"])
	}
	if len(required) != 5 || required[0] != "detail" || required[1] != "offset" || required[2] != "path" || required[3] != "prompt" || required[4] != "scale" {
		t.Fatalf("expected analyze_screenshot required fields [detail offset path prompt scale], got %#v", required)
	}
}

func findAgentTool(tools []agentTool, name string) (agentTool, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return agentTool{}, false
}

func hasAgentTool(tools []agentTool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func mustResponseOutputItem(t *testing.T, raw string) responses.ResponseOutputItemUnion {
	t.Helper()

	var item responses.ResponseOutputItemUnion
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatalf("failed to decode response output item: %v", err)
	}
	return item
}

func marshalJSON(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return string(data)
}
