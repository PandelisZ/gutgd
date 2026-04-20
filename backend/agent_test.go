package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/shared"
	"github.com/openai/openai-go/v3/responses"
)

func TestNormalizeAgentSettingsDefaultsModel(t *testing.T) {
	settings := normalizeAgentSettings(AgentSettings{
		APIKey: "  test-key  ",
	})

	if settings.APIKey != "test-key" {
		t.Fatalf("expected trimmed API key, got %q", settings.APIKey)
	}
	if settings.BaseURL != "" {
		t.Fatalf("expected empty base URL by default, got %q", settings.BaseURL)
	}
	if settings.Model != defaultAgentModel {
		t.Fatalf("expected default model %q, got %q", defaultAgentModel, settings.Model)
	}
	if settings.ReasoningEffort != "medium" {
		t.Fatalf("expected default reasoning effort %q, got %q", "medium", settings.ReasoningEffort)
	}
	if settings.SystemPrompt != defaultAgentSystemPrompt {
		t.Fatalf("expected default system prompt to be populated, got %q", settings.SystemPrompt)
	}
}

func TestMergeAgentSettingsWithEnvironmentPrefersSavedValues(t *testing.T) {
	merged := mergeAgentSettingsWithEnvironment(
		AgentSettings{
			APIKey:  "saved-key",
			BaseURL: "https://saved.example/v1",
		},
		AgentSettings{
			APIKey:  "env-key",
			BaseURL: "https://env.example/v1",
		},
	)

	if merged.APIKey != "saved-key" {
		t.Fatalf("expected saved api key to win, got %q", merged.APIKey)
	}
	if merged.BaseURL != "https://saved.example/v1" {
		t.Fatalf("expected saved base url to win, got %q", merged.BaseURL)
	}
}

func TestMergeAgentSettingsWithEnvironmentFillsMissingValues(t *testing.T) {
	merged := mergeAgentSettingsWithEnvironment(
		AgentSettings{},
		AgentSettings{
			APIKey:  "env-key",
			BaseURL: "https://env.example/v1",
		},
	)

	if merged.APIKey != "env-key" {
		t.Fatalf("expected env api key to be applied, got %q", merged.APIKey)
	}
	if merged.BaseURL != "https://env.example/v1" {
		t.Fatalf("expected env base url to be applied, got %q", merged.BaseURL)
	}
}

func TestAgentSettingsStatusReportsCredentialSource(t *testing.T) {
	if status := agentSettingsStatus(AgentSettings{APIKey: "saved-key"}, AgentSettings{APIKey: "env-key"}); !status.HasAPIKey || status.APIKeySource != "saved" {
		t.Fatalf("expected saved api key status, got %+v", status)
	}
	if status := agentSettingsStatus(AgentSettings{}, AgentSettings{APIKey: "env-key"}); !status.HasAPIKey || status.APIKeySource != "environment" {
		t.Fatalf("expected environment api key status, got %+v", status)
	}
	if status := agentSettingsStatus(AgentSettings{}, AgentSettings{}); status.HasAPIKey || status.APIKeySource != "missing" {
		t.Fatalf("expected missing api key status, got %+v", status)
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

func TestCombineAgentInstructionsAddsStrictBackgroundOnlyGuidance(t *testing.T) {
	value := combineAgentInstructions(AgentSettings{
		SystemPrompt:         "Prefer concise answers.",
		StrictBackgroundOnly: true,
	})
	for _, needle := range []string{
		"Prefer concise answers.",
		"Strict background-only mouse mode is enabled.",
		"Raw pointer tools are intentionally unavailable.",
		"unsupported_background_action",
	} {
		if !strings.Contains(value, needle) {
			t.Fatalf("expected combined agent instructions to contain %q, got %q", needle, value)
		}
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

func TestAgentToolsExposeCoordinateSpaceControls(t *testing.T) {
	service := NewService()
	tools := service.agentToolsForGOOS("linux")

	for _, name := range []string{
		"get_coordinate_space",
		"switch_to_active_window_space",
		"switch_to_window_space",
		"switch_to_screen_space",
		"translate_image_point_to_screen",
		"capture_screen",
		"capture_active_window",
		"capture_window",
	} {
		if !hasAgentTool(tools, name) {
			t.Fatalf("expected %s to be exposed", name)
		}
	}
	if hasAgentTool(tools, "capture_region") {
		t.Fatal("expected capture_region to be hidden from the agent tool surface")
	}
}

func TestAgentToolsExposeMouseDownAndMouseUp(t *testing.T) {
	service := NewService()
	tools := service.agentToolsForGOOS("linux")

	for _, name := range []string{"mouse_down", "mouse_up"} {
		if !hasAgentTool(tools, name) {
			t.Fatalf("expected %s to be exposed", name)
		}
	}
}

func TestAgentToolsExposeAccessibilityMetadataTools(t *testing.T) {
	service := NewService()
	tools := service.agentToolsForGOOS("darwin")

	for _, name := range []string{
		"get_permission_readiness",
		"search_ax_elements",
		"focus_ax_element",
		"perform_ax_element_action",
		"get_focused_window_metadata",
		"get_window_accessibility_snapshot",
		"act_on_window_accessibility_element",
		"get_focused_element_metadata",
		"get_element_at_point_metadata",
		"raise_focused_window",
		"perform_focused_element_action",
		"perform_element_action_at_point",
		"focus_element_at_point",
	} {
		if !hasAgentTool(tools, name) {
			t.Fatalf("expected %s to be exposed", name)
		}
	}

	for _, test := range []struct {
		name    string
		needles []string
	}{
		{name: "search_ax_elements", needles: []string{"inspect-then-act", "scope", "focus_ax_element", "perform_ax_element_action", "permission_blocked"}},
		{name: "focus_ax_element", needles: []string{"search_ax_elements", "matches", "ref", "permission_blocked"}},
		{name: "perform_ax_element_action", needles: []string{"search_ax_elements", "ref", "AX action", "permission_blocked"}},
	} {
		tool, ok := findAgentTool(tools, test.name)
		if !ok {
			t.Fatalf("expected %s to be exposed", test.name)
		}
		for _, needle := range test.needles {
			if !strings.Contains(tool.Description, needle) {
				t.Fatalf("expected %s description to contain %q, got %q", test.name, needle, tool.Description)
			}
		}
	}
}

func TestSearchAXElementsToolSchemaIncludesWindowHandleScope(t *testing.T) {
	service := NewService()
	tool, ok := findAgentTool(service.agentToolsForGOOS("darwin"), "search_ax_elements")
	if !ok {
		t.Fatal("expected search_ax_elements to be exposed")
	}

	schemaJSON, err := json.Marshal(tool.Parameters)
	if err != nil {
		t.Fatalf("marshal tool schema: %v", err)
	}
	schema := string(schemaJSON)
	for _, needle := range []string{`"window_handle"`, string(common.AXSearchScopeWindowHandle)} {
		if !strings.Contains(schema, needle) {
			t.Fatalf("expected search_ax_elements schema to contain %q, got %s", needle, schema)
		}
	}
}

func TestAgentToolsExposeRunLuaScript(t *testing.T) {
	service := NewService()
	tool, ok := findAgentTool(service.agentToolsForGOOS("linux"), "run_lua_script")
	if !ok {
		t.Fatal("expected run_lua_script to be exposed")
	}
	if !strings.Contains(tool.Description, "many repeated or math-driven actions") {
		t.Fatalf("expected run_lua_script description to mention repetitive sequences, got %q", tool.Description)
	}

	required, ok := tool.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("expected run_lua_script required fields to be []string, got %#v", tool.Parameters["required"])
	}
	for _, field := range []string{"instruction_budget", "script"} {
		if !slices.Contains(required, field) {
			t.Fatalf("expected run_lua_script required fields to include %q, got %#v", field, required)
		}
	}
}

func TestRunLuaScriptToolSupportsLoopsAndToolSchemas(t *testing.T) {
	callCount := 0
	tools := []agentTool{
		{
			Name:        "move_mouse_line",
			Description: "Move the pointer in a straight line.",
			Parameters: objectSchema(map[string]any{
				"x": integerSchema("x"),
				"y": integerSchema("y"),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				callCount++
				var payload map[string]any
				if err := json.Unmarshal([]byte(raw), &payload); err != nil {
					return nil, err
				}
				return map[string]any{
					"index": callCount,
					"x":     payload["x"],
					"y":     payload["y"],
				}, nil
			},
		},
	}

	result, err := runLuaScriptTool(nil, tools, `{"script":"local results = {}\nfor i = 1, 4 do\n  local point, tool_err = tools.move_mouse_line({ x = i, y = i * 2 })\n  if tool_err then error(tool_err) end\n  results[i] = point\nend\nreturn { total = #results, first_description = tool_schemas.move_mouse_line.description, last_y = results[4].y }"}`)
	if err != nil {
		t.Fatalf("runLuaScriptTool returned error: %v", err)
	}

	luaResult, ok := result.(AgentLuaScriptResult)
	if !ok {
		t.Fatalf("expected AgentLuaScriptResult, got %T", result)
	}
	if luaResult.ToolCalls != 4 {
		t.Fatalf("expected 4 tool calls, got %d", luaResult.ToolCalls)
	}

	resultMap, ok := luaResult.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected structured Lua result map, got %#v", luaResult.Result)
	}
	if resultMap["total"] != 4 {
		t.Fatalf("expected total=4, got %#v", resultMap["total"])
	}
	if resultMap["last_y"] != 8 {
		t.Fatalf("expected last_y=8, got %#v", resultMap["last_y"])
	}
	if resultMap["first_description"] != "Move the pointer in a straight line." {
		t.Fatalf("unexpected tool schema description: %#v", resultMap["first_description"])
	}
}

func TestRunLuaScriptToolAllowsUnlimitedCallsByDefault(t *testing.T) {
	callCount := 0
	tools := []agentTool{
		{
			Name:        "move_mouse_line",
			Description: "Move the pointer in a straight line.",
			Parameters: objectSchema(map[string]any{
				"x": integerSchema("x"),
				"y": integerSchema("y"),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				callCount++
				return map[string]any{"count": callCount}, nil
			},
		},
	}

	result, err := runLuaScriptTool(nil, tools, `{"script":"for i = 1, 40 do local _, tool_err = tools.move_mouse_line({ x = i, y = i }) if tool_err then error(tool_err) end end return { done = true }"}`)
	if err != nil {
		t.Fatalf("runLuaScriptTool returned error: %v", err)
	}
	luaResult, ok := result.(AgentLuaScriptResult)
	if !ok {
		t.Fatalf("expected AgentLuaScriptResult, got %T", result)
	}
	if luaResult.ToolCalls != 40 {
		t.Fatalf("expected 40 tool calls without an explicit max_tool_calls limit, got %d", luaResult.ToolCalls)
	}
}

func TestRunLuaScriptToolExposesGeometryHelpers(t *testing.T) {
	result, err := runLuaScriptTool(nil, nil, `{"script":"local pts = geom.circle(100, 200, 10, 4, 1)\nlocal line = geom.smooth_line(0, 0, 10, 10, 3, 1)\nlocal snapped = geom.snap_point(12.2, 19.8, 5)\nlocal rounded = geom.round_to_step(13.2, 4)\nreturn { count = #pts, first = pts[1], line_count = #line, snapped = snapped, rounded = rounded }"}`)
	if err != nil {
		t.Fatalf("runLuaScriptTool returned error: %v", err)
	}

	luaResult, ok := result.(AgentLuaScriptResult)
	if !ok {
		t.Fatalf("expected AgentLuaScriptResult, got %T", result)
	}
	resultMap, ok := luaResult.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected structured Lua result map, got %#v", luaResult.Result)
	}
	if resultMap["count"] != 4 {
		t.Fatalf("expected 4 circle points, got %#v", resultMap["count"])
	}
	if resultMap["line_count"] != 3 {
		t.Fatalf("expected 3 smooth line points, got %#v", resultMap["line_count"])
	}
	if resultMap["rounded"] != 12 {
		t.Fatalf("expected rounded value 12, got %#v", resultMap["rounded"])
	}

	first, ok := resultMap["first"].(map[string]any)
	if !ok || first["x"] != 110 || first["y"] != 200 {
		t.Fatalf("unexpected first circle point: %#v", resultMap["first"])
	}
	snapped, ok := resultMap["snapped"].(map[string]any)
	if !ok || snapped["x"] != 10 || snapped["y"] != 20 {
		t.Fatalf("unexpected snapped point: %#v", resultMap["snapped"])
	}
}

func TestRunLuaScriptToolPersistsGlobalsAcrossCalls(t *testing.T) {
	session := &agentLuaSession{}
	tools := []agentTool{
		{
			Name:        "move_mouse_line",
			Description: "Move the pointer in a straight line.",
			Parameters: objectSchema(map[string]any{
				"x": integerSchema("x"),
				"y": integerSchema("y"),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				return map[string]any{"ok": true}, nil
			},
		},
	}

	_, err := runLuaScriptTool(session, tools, `{"script":"counter = (counter or 0) + 1\nfunction next_point(x, y)\n  return { x = x + counter, y = y + counter }\nend\nreturn { counter = counter }"}`)
	if err != nil {
		t.Fatalf("first runLuaScriptTool returned error: %v", err)
	}
	result, err := runLuaScriptTool(session, tools, `{"script":"counter = counter + 1\nlocal point = next_point(10, 20)\nreturn { counter = counter, point = point }"}`)
	if err != nil {
		t.Fatalf("second runLuaScriptTool returned error: %v", err)
	}

	luaResult, ok := result.(AgentLuaScriptResult)
	if !ok {
		t.Fatalf("expected AgentLuaScriptResult, got %T", result)
	}
	resultMap, ok := luaResult.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected structured Lua result map, got %#v", luaResult.Result)
	}
	if resultMap["counter"] != 2 {
		t.Fatalf("expected persisted counter=2, got %#v", resultMap["counter"])
	}
	point, ok := resultMap["point"].(map[string]any)
	if !ok || point["x"] != 12 || point["y"] != 22 {
		t.Fatalf("unexpected persisted function result: %#v", resultMap["point"])
	}
}

func TestSwitchToWindowSpaceRejectsUnknownHandle(t *testing.T) {
	service := newTestServiceWithWindows(WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})
	state := newAgentCoordinateState()
	tool, ok := findAgentTool(service.agentToolsForGOOSWithState("linux", state, nil), "switch_to_window_space")
	if !ok {
		t.Fatal("expected switch_to_window_space to be exposed")
	}

	_, err := tool.Run(`{"handle":999}`)
	if !errors.Is(err, errWindowHandleNotFound) {
		t.Fatalf("expected errWindowHandleNotFound, got %v", err)
	}
	if state.Mode != "screen" || state.Window != nil {
		t.Fatalf("expected invalid switch_to_window_space call to leave coordinate state unchanged, got %+v", state)
	}
}

func TestAgentRawInputToolsFocusSelectedWindowInWindowSpace(t *testing.T) {
	service := newTestServiceWithWindows(
		WindowSummary{Handle: 1, Title: "Terminal", Region: Region{Left: 20, Top: 20, Width: 500, Height: 300}},
		WindowSummary{Handle: 7, Title: "Slack", Region: Region{Left: 300, Top: 120, Width: 800, Height: 600}},
	)
	state := &agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 7,
			Title:  "Slack",
			Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
		},
	}
	tools := service.agentToolsForGOOSWithState("darwin", state, nil)

	typeTextTool, ok := findAgentTool(tools, "type_text")
	if !ok {
		t.Fatal("expected type_text to be exposed")
	}
	if _, err := typeTextTool.Run(`{"text":"hello"}`); err != nil {
		t.Fatalf("type_text returned error: %v", err)
	}

	clickTool, ok := findAgentTool(tools, "click_mouse")
	if !ok {
		t.Fatal("expected click_mouse to be exposed")
	}
	if _, err := clickTool.Run(`{"button":"left"}`); err != nil {
		t.Fatalf("click_mouse returned error: %v", err)
	}

	windowProvider, err := service.nut.Registry.Window()
	if err != nil {
		t.Fatalf("window provider: %v", err)
	}
	fakeWindows, ok := windowProvider.(*fakeBackendWindowProvider)
	if !ok {
		t.Fatalf("expected fake backend window provider, got %T", windowProvider)
	}
	if len(fakeWindows.focusCalls) != 1 || fakeWindows.focusCalls[0] != shared.WindowHandle(7) {
		t.Fatalf("expected raw input to focus window 7 once, got %+v", fakeWindows.focusCalls)
	}

	keyboardProvider, err := service.nut.Registry.Keyboard()
	if err != nil {
		t.Fatalf("keyboard provider: %v", err)
	}
	fakeKeyboard, ok := keyboardProvider.(*fakeBackendKeyboardProvider)
	if !ok {
		t.Fatalf("expected fake backend keyboard provider, got %T", keyboardProvider)
	}
	if strings.Join(fakeKeyboard.typedText, "") != "hello" {
		t.Fatalf("unexpected typed text: %+v", fakeKeyboard.typedText)
	}

	mouseProvider, err := service.nut.Registry.Mouse()
	if err != nil {
		t.Fatalf("mouse provider: %v", err)
	}
	fakeMouse, ok := mouseProvider.(*fakeBackendMouseProvider)
	if !ok {
		t.Fatalf("expected fake backend mouse provider, got %T", mouseProvider)
	}
	if len(fakeMouse.clicks) != 1 || fakeMouse.clicks[0] != shared.ButtonLeft {
		t.Fatalf("unexpected mouse clicks: %+v", fakeMouse.clicks)
	}
}

func TestStrictBackgroundOnlyModeHidesRawPointerTools(t *testing.T) {
	service := NewService()
	tools := service.agentToolsForGOOSWithStateAndSettings("darwin", newAgentCoordinateState(), nil, AgentSettings{
		StrictBackgroundOnly: true,
	})

	for _, hidden := range []string{
		"get_mouse_position",
		"set_mouse_position",
		"move_mouse_line",
		"click_mouse",
		"mouse_down",
		"mouse_up",
		"double_click_mouse",
		"scroll_mouse",
		"drag_mouse",
	} {
		if hasAgentTool(tools, hidden) {
			t.Fatalf("expected %s to be hidden in strict background-only mode", hidden)
		}
	}

	tool, ok := findAgentTool(tools, "act_on_window_accessibility_element")
	if !ok {
		t.Fatal("expected act_on_window_accessibility_element to remain exposed")
	}
	if !strings.Contains(tool.Description, "fails closed") {
		t.Fatalf("expected strict act_on_window_accessibility_element description, got %q", tool.Description)
	}
}

func TestStrictBackgroundOnlyActOnWindowAccessibilityElementUsesBackgroundVirtualPath(t *testing.T) {
	service, _, snapshot, mouse := newBackgroundVirtualActionFixture(t)
	tools := service.agentToolsForGOOSWithStateAndSettings("darwin", newAgentCoordinateState(), nil, AgentSettings{
		StrictBackgroundOnly: true,
	})
	tool, ok := findAgentTool(tools, "act_on_window_accessibility_element")
	if !ok {
		t.Fatal("expected act_on_window_accessibility_element to be exposed")
	}

	raw, err := tool.Run(fmt.Sprintf(`{"snapshot_id":%q,"element_id":%q,"action":"click"}`, snapshot.SnapshotID, snapshot.Elements[1].ID))
	if err != nil {
		t.Fatalf("act_on_window_accessibility_element returned error: %v", err)
	}

	result, ok := raw.(WindowAccessibilityElementActionResult)
	if !ok {
		t.Fatalf("expected WindowAccessibilityElementActionResult, got %T", raw)
	}
	if result.Mode != "background_virtual" || result.ScreenPoint != (Point{X: 1016, Y: 712}) {
		t.Fatalf("expected strict background-virtual action result, got %+v", result)
	}
	if len(mouse.setPositions) != 0 || len(mouse.clicks) != 0 || len(mouse.doubleClicks) != 0 {
		t.Fatalf("expected strict mode to avoid real mouse input, got set=%+v click=%+v double=%+v", mouse.setPositions, mouse.clicks, mouse.doubleClicks)
	}
}

func TestAgentToolsMatchPlatformAvailability(t *testing.T) {
	service := NewService()

	darwinTools := service.agentToolsForGOOS("darwin")
	if !hasAgentTool(darwinTools, "press_special_key") {
		t.Fatal("expected press_special_key to be exposed on darwin")
	}
	for _, exposed := range []string{"tap_keys", "press_keys", "release_keys"} {
		if !hasAgentTool(darwinTools, exposed) {
			t.Fatalf("expected %s to be exposed on darwin", exposed)
		}
	}
	if hasAgentTool(darwinTools, "highlight_region") {
		t.Fatalf("expected %s to be hidden on darwin", "highlight_region")
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
		"capture_screen, capture_active_window, and capture_window use the safe OS screenshot fallback",
		"capture_active_window",
		"capture_window",
		"Prefer tap_keys for real shortcuts like cmd+space",
		"press_special_key for single non-text keys",
		"get_permission_readiness",
		"get_focused_window_metadata",
		"get_focused_element_metadata",
		"get_element_at_point_metadata",
		"search_ax_elements",
		"focus_ax_element",
		"perform_ax_element_action",
		"window_handle scope",
		"permission_blocked",
		"Accessibility permission is required",
		"Highlight_region remains intentionally unavailable",
		"switch_to_active_window_space",
		"load_image_for_context",
		"inspect that returned image first",
		"translate_image_point_to_screen",
		"structured fallback",
		"absolute screen coordinates, not window-relative coordinates",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected darwin prompt to contain %q, got %q", needle, prompt)
		}
	}

	linuxPrompt := agentDeveloperPromptForGOOS("linux")
	if !strings.Contains(linuxPrompt, "prefer tap_keys with the full key list in one call") {
		t.Fatalf("expected non-darwin prompt to keep tap_keys guidance, got %q", linuxPrompt)
	}
	if strings.Contains(linuxPrompt, "intentionally unavailable on macOS 26+") {
		t.Fatalf("expected non-darwin prompt to omit darwin-only warning, got %q", linuxPrompt)
	}
}

func TestTranslatePointToScreenSpaceUsesWindowOrigin(t *testing.T) {
	state := agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 7,
			Title:  "Slack",
			Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
		},
	}

	got := translatePointToScreenSpace(Point{X: 15, Y: 25}, state)
	if got != (Point{X: 315, Y: 145}) {
		t.Fatalf("unexpected translated point: %#v", got)
	}
}

func TestTranslateRegionToScreenSpaceUsesWindowOrigin(t *testing.T) {
	state := agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 7,
			Title:  "Slack",
			Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
		},
	}

	got := translateRegionToScreenSpace(Region{Left: 10, Top: 20, Width: 40, Height: 50}, state)
	if got != (Region{Left: 310, Top: 140, Width: 40, Height: 50}) {
		t.Fatalf("unexpected translated region: %#v", got)
	}
}

func TestGetElementAtPointMetadataToolTranslatesWindowCoordinates(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{
		elementAtPoint: common.UIElementMetadata{
			Role:       "AXButton",
			Title:      "Send",
			Enabled:    true,
			Frame:      common.Rect{X: 460, Y: 180, Width: 90, Height: 24},
			FrameKnown: true,
			Actions:    []string{"AXPress"},
		},
	}
	service := newTestServiceWithWindowsAndAccessibility(accessibility, WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})
	state := &agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 7,
			Title:  "Slack",
			Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
		},
	}
	tool, ok := findAgentTool(service.agentToolsForGOOSWithState("darwin", state, nil), "get_element_at_point_metadata")
	if !ok {
		t.Fatal("expected get_element_at_point_metadata to be exposed")
	}

	raw, err := tool.Run(`{"x":160,"y":60}`)
	if err != nil {
		t.Fatalf("get_element_at_point_metadata returned error: %v", err)
	}

	result, ok := raw.(AgentTranslatedPointResult)
	if !ok {
		t.Fatalf("expected AgentTranslatedPointResult, got %T", raw)
	}
	if result.Requested != (Point{X: 160, Y: 60}) || result.ScreenPoint != (Point{X: 460, Y: 180}) {
		t.Fatalf("unexpected translated point result: %+v", result)
	}
	if accessibility.lastPoint != (shared.Point{X: 460, Y: 180}) {
		t.Fatalf("expected provider query point (460,180), got %+v", accessibility.lastPoint)
	}
	metadata, ok := result.Result.(UIElementMetadataResult)
	if !ok {
		t.Fatalf("expected UIElementMetadataResult, got %T", result.Result)
	}
	if metadata.Title != "Send" || metadata.Frame.Left != 460 || metadata.Frame.Top != 180 {
		t.Fatalf("unexpected metadata result: %+v", metadata)
	}
}

func TestAccessibilityActionToolsTranslateWindowCoordinates(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{}
	service := newTestServiceWithWindowsAndAccessibility(accessibility, WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})
	state := &agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 7,
			Title:  "Slack",
			Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
		},
	}

	pointActionTool, ok := findAgentTool(service.agentToolsForGOOSWithState("darwin", state, nil), "perform_element_action_at_point")
	if !ok {
		t.Fatal("expected perform_element_action_at_point to be exposed")
	}
	raw, err := pointActionTool.Run(`{"x":160,"y":60,"action":"AXShowMenu"}`)
	if err != nil {
		t.Fatalf("perform_element_action_at_point returned error: %v", err)
	}
	pointActionResult, ok := raw.(AgentTranslatedPointResult)
	if !ok {
		t.Fatalf("expected AgentTranslatedPointResult, got %T", raw)
	}
	if pointActionResult.ScreenPoint != (Point{X: 460, Y: 180}) {
		t.Fatalf("unexpected translated screen point: %+v", pointActionResult)
	}
	if accessibility.lastElementActionAtPoint != (shared.Point{X: 460, Y: 180}) || accessibility.lastElementActionAtPointAction != common.AXShowMenu {
		t.Fatalf("unexpected forwarded point action: point=%+v action=%q", accessibility.lastElementActionAtPoint, accessibility.lastElementActionAtPointAction)
	}

	focusTool, ok := findAgentTool(service.agentToolsForGOOSWithState("darwin", state, nil), "focus_element_at_point")
	if !ok {
		t.Fatal("expected focus_element_at_point to be exposed")
	}
	raw, err = focusTool.Run(`{"x":50,"y":40}`)
	if err != nil {
		t.Fatalf("focus_element_at_point returned error: %v", err)
	}
	focusResult, ok := raw.(AgentTranslatedPointResult)
	if !ok {
		t.Fatalf("expected AgentTranslatedPointResult, got %T", raw)
	}
	if focusResult.ScreenPoint != (Point{X: 350, Y: 160}) {
		t.Fatalf("unexpected focus translated point: %+v", focusResult)
	}
	if accessibility.lastFocusElementAtPoint != (shared.Point{X: 350, Y: 160}) {
		t.Fatalf("unexpected forwarded focus point: %+v", accessibility.lastFocusElementAtPoint)
	}
}

func TestAgentCoordinateStatePersistsByResponseID(t *testing.T) {
	service := NewService()
	service.saveAgentCoordinateState("resp_1", &agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 99,
			Title:  "Browser",
			Region: Region{Left: 50, Top: 70, Width: 1200, Height: 900},
		},
	})

	got := service.loadAgentCoordinateState("resp_1")
	if got.Mode != "window" || got.Window == nil || got.Window.Handle != 99 || got.Window.Region.Left != 50 || got.Window.Region.Top != 70 {
		t.Fatalf("unexpected persisted coordinate state: %+v", got)
	}
}

func TestAgentTranscriptStatePersistsByResponseID(t *testing.T) {
	service := NewService()
	items := []AgentTranscriptItem{
		{Kind: "message", Role: "assistant", Content: "I found the target window."},
		{Kind: "tool_output", Name: "capture_screen", Output: `{"path":"capture.png"}`},
	}

	service.saveAgentTranscriptState("resp_1", items)
	got := service.loadAgentTranscriptState("resp_1")
	if len(got) != 2 {
		t.Fatalf("expected 2 transcript items, got %d", len(got))
	}
	if got[0].Content != "I found the target window." || got[1].Name != "capture_screen" {
		t.Fatalf("unexpected transcript state: %+v", got)
	}

	got[0].Content = "mutated"
	reloaded := service.loadAgentTranscriptState("resp_1")
	if reloaded[0].Content != "I found the target window." {
		t.Fatalf("expected transcript state to be copied defensively, got %+v", reloaded)
	}
}

func TestContinueInputItemsAppendsSyntheticContinueMessage(t *testing.T) {
	outputs := []responses.ResponseInputItemUnionParam{
		responses.ResponseInputItemParamOfFunctionCallOutput("call_1", `{"ok":true}`),
	}

	items := continueInputItems("<agent_state>memory</agent_state>", outputs)
	if len(items) != 3 {
		t.Fatalf("expected scaffold, one tool output, plus one continue message, got %d items", len(items))
	}

	scaffoldPayload := marshalJSON(t, items[0])
	if !strings.Contains(scaffoldPayload, `"role":"developer"`) || !strings.Contains(scaffoldPayload, `\u003cagent_state\u003ememory\u003c/agent_state\u003e`) {
		t.Fatalf("expected developer scaffold message first, got %s", scaffoldPayload)
	}

	payload := marshalJSON(t, items[2])
	if !strings.Contains(payload, `"role":"user"`) || !strings.Contains(payload, `"content":"continue"`) {
		t.Fatalf("expected synthetic continue user message, got %s", payload)
	}
}

func TestAgentLoopScaffoldIncludesMemoryAndGoal(t *testing.T) {
	scaffold := agentLoopScaffold("Find the send button.", []AgentTranscriptItem{
		{Kind: "tool_output", Name: "capture_region", Output: `{"path":"capture.png"}`},
		{Kind: "message", Role: "assistant", Content: "I found the compose area."},
	}, &agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 9,
			Title:  "Slack",
			Region: Region{Left: 100, Top: 200, Width: 800, Height: 600},
		},
	})

	for _, needle := range []string{
		"<evaluation_previous_step>",
		"<memory>",
		"<plan>",
		"<trajectory_plan>",
		"<todo_list>",
		"<next_goal>",
		"<agent_history>",
		"window space for \"Slack\"",
		"Find the send button.",
	} {
		if !strings.Contains(scaffold, needle) {
			t.Fatalf("expected scaffold to contain %q, got %q", needle, scaffold)
		}
	}
}

func TestAgentFunctionCallOutputAttachesScreenshotAsImageInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	output := marshalToolPayload(CaptureResult{
		Path:          path,
		Message:       "Saved capture",
		Offset:        Point{X: 2860, Y: 1510},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 800, Height: 600},
		DeliveredSize: Size{Width: 400, Height: 300},
		OriginalScale: Scale{X: 2, Y: 2},
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
	if !strings.Contains(payload, `"detail":"original"`) {
		t.Fatalf("expected fresh capture image attachment detail to be original, got %s", payload)
	}
	if !strings.Contains(payload, `"type":"input_text"`) {
		t.Fatalf("expected screenshot output to preserve textual tool output, got %s", payload)
	}
	for _, needle := range []string{
		`screen offset: (2860, 1510)`,
		`delivered scale: x=1.0000, y=1.0000`,
		`original size: 800x600`,
		`delivered size: 400x300`,
		`original scale: x=2.0000, y=2.0000`,
		`translate_image_point_to_screen`,
		`original_x = image_x * 800 / 400`,
		`screen_x = 2860 + original_x / 2.0000`,
	} {
		if !strings.Contains(payload, needle) {
			t.Fatalf("expected screenshot output text payload to contain %q, got %s", needle, payload)
		}
	}
}

func TestAgentFunctionCallOutputAttachesNestedCaptureRegionScreenshotAsImageInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested-capture.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	output := marshalToolPayload(AgentTranslatedRegionResult{
		CoordinateSpace: AgentCoordinateSpace{Mode: "window", Origin: Point{X: 120, Y: 80}},
		Requested:       Region{Left: 10, Top: 20, Width: 400, Height: 300},
		ScreenRegion:    Region{Left: 130, Top: 100, Width: 400, Height: 300},
		Result: CaptureResult{
			Path:          path,
			Message:       "Saved capture",
			Offset:        Point{X: 130, Y: 100},
			Scale:         Scale{X: 2, Y: 2},
			OriginalSize:  Size{Width: 800, Height: 600},
			DeliveredSize: Size{Width: 800, Height: 600},
			OriginalScale: Scale{X: 2, Y: 2},
		},
	})

	item := agentFunctionCallOutput("call_capture_region", output)
	payload := marshalJSON(t, item)
	if !strings.Contains(payload, `"type":"input_image"`) {
		t.Fatalf("expected nested capture_region output to attach input_image, got %s", payload)
	}
	if !strings.Contains(payload, `"detail":"original"`) {
		t.Fatalf("expected nested capture_region image attachment detail to be original, got %s", payload)
	}
	if !strings.Contains(payload, `translate_image_point_to_screen`) {
		t.Fatalf("expected nested capture instruction text to mention translate_image_point_to_screen, got %s", payload)
	}
}

func TestTranslateImagePointToScreenToolUsesCaptureMetadata(t *testing.T) {
	service := NewService()
	tool, ok := findAgentTool(service.agentToolsForGOOS("linux"), "translate_image_point_to_screen")
	if !ok {
		t.Fatal("expected translate_image_point_to_screen to be exposed")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write capture: %v", err)
	}
	if err := writeCaptureMetadata(CaptureResult{
		Path:          path,
		Message:       "Saved capture",
		Offset:        Point{X: 50, Y: 80},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 800, Height: 600},
		DeliveredSize: Size{Width: 400, Height: 300},
		OriginalScale: Scale{X: 2, Y: 2},
	}); err != nil {
		t.Fatalf("failed to write capture metadata: %v", err)
	}

	raw, err := tool.Run(fmt.Sprintf(`{"path":%q,"x":120,"y":40}`, path))
	if err != nil {
		t.Fatalf("translate_image_point_to_screen returned error: %v", err)
	}

	result, ok := raw.(ImagePointTranslationResult)
	if !ok {
		t.Fatalf("expected ImagePointTranslationResult, got %T", raw)
	}
	if result.RequestedDeliveredPoint != (Point{X: 120, Y: 40}) {
		t.Fatalf("unexpected requested delivered point: %+v", result.RequestedDeliveredPoint)
	}
	if result.OriginalImagePoint != (ImagePoint{X: 240, Y: 80}) {
		t.Fatalf("unexpected original image point: %+v", result.OriginalImagePoint)
	}
	if result.AbsoluteScreenPoint != (Point{X: 170, Y: 120}) {
		t.Fatalf("unexpected absolute screen point: %+v", result.AbsoluteScreenPoint)
	}
	if result.Capture.Scale != (Scale{X: 1, Y: 1}) || result.Capture.OriginalScale != (Scale{X: 2, Y: 2}) {
		t.Fatalf("unexpected capture scales in translation result: %+v", result.Capture)
	}
}

func TestAgentFunctionCallOutputAttachesLoadedImageAsImageInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loaded.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	output := marshalToolPayload(AgentImageLoadResult{
		Path:    path,
		Message: "Loaded image into the model context.",
	})

	item := agentFunctionCallOutput("call_load", output)
	payload := marshalJSON(t, item)
	if !strings.Contains(payload, `"type":"function_call_output"`) {
		t.Fatalf("expected function_call_output payload, got %s", payload)
	}
	if !strings.Contains(payload, `"type":"input_image"`) {
		t.Fatalf("expected loaded image output to attach input_image, got %s", payload)
	}
	if !strings.Contains(payload, `"image_url":"data:image/png;base64,`) {
		t.Fatalf("expected loaded image output to use a data URL, got %s", payload)
	}
	if !strings.Contains(payload, `"detail":"high"`) {
		t.Fatalf("expected loaded image output detail to remain high, got %s", payload)
	}
	if !strings.Contains(payload, `Loaded image into the model context.`) {
		t.Fatalf("expected loaded image output text payload to preserve the message, got %s", payload)
	}
}

func TestAgentToolsExposeAnalyzeScreenshot(t *testing.T) {
	service := NewService()
	tool, ok := findAgentTool(service.agentToolsForGOOS("darwin"), "analyze_screenshot")
	if !ok {
		t.Fatal("expected analyze_screenshot to be exposed")
	}
	if !strings.Contains(tool.Description, "structured fallback") {
		t.Fatalf("expected analyze_screenshot description to mention structured fallback, got %q", tool.Description)
	}

	required, ok := tool.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("expected analyze_screenshot required fields to be []string, got %#v", tool.Parameters["required"])
	}
	if len(required) != 5 || required[0] != "detail" || required[1] != "offset" || required[2] != "path" || required[3] != "prompt" || required[4] != "scale" {
		t.Fatalf("expected analyze_screenshot required fields [detail offset path prompt scale], got %#v", required)
	}
}

func TestAgentToolsExposeLoadImageForContext(t *testing.T) {
	service := NewService()
	tool, ok := findAgentTool(service.agentToolsForGOOS("darwin"), "load_image_for_context")
	if !ok {
		t.Fatal("expected load_image_for_context to be exposed")
	}
	if !strings.Contains(tool.Description, "previously saved screenshots or arbitrary image paths") {
		t.Fatalf("expected load_image_for_context description to mention previously saved screenshots or arbitrary image paths, got %q", tool.Description)
	}

	required, ok := tool.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("expected load_image_for_context required fields to be []string, got %#v", tool.Parameters["required"])
	}
	if len(required) != 1 || required[0] != "path" {
		t.Fatalf("expected load_image_for_context required fields [path], got %#v", required)
	}
}

func TestEmptyObjectSchemaMarshalsRequiredAsEmptyArray(t *testing.T) {
	schema := emptyObjectSchema()

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required fields to be []string, got %#v", schema["required"])
	}
	if len(required) != 0 {
		t.Fatalf("expected no required fields, got %#v", required)
	}

	payload := marshalJSON(t, schema)
	if !strings.Contains(payload, `"required":[]`) {
		t.Fatalf("expected required to marshal as an empty array, got %s", payload)
	}
	if strings.Contains(payload, `"required":null`) {
		t.Fatalf("expected required to avoid null, got %s", payload)
	}
}

func TestCaptureRequestSchemaIncludesAllPropertiesInRequired(t *testing.T) {
	schema := captureRequestSchema()

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required fields to be []string, got %#v", schema["required"])
	}
	if len(required) != 3 || required[0] != "file_name" || required[1] != "max_image_height" || required[2] != "max_image_width" {
		t.Fatalf("expected capture request required fields to include every property, got %#v", required)
	}

	payload := marshalJSON(t, schema)
	if !strings.Contains(payload, `"required":["file_name","max_image_height","max_image_width"]`) {
		t.Fatalf("expected required to marshal all properties, got %s", payload)
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
