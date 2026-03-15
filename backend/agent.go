package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

const (
	defaultAgentModel = "gpt-5.4"
	maxAgentSteps     = 8
)

type AgentSettings struct {
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	SystemPrompt    string `json:"system_prompt"`
}

type AgentChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AgentChatRequest struct {
	Messages []AgentChatMessage `json:"messages"`
}

type AgentToolEvent struct {
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Output    string `json:"output"`
	Error     string `json:"error"`
}

type AgentUsage struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

type AgentChatResponse struct {
	Message    AgentChatMessage `json:"message"`
	ToolEvents []AgentToolEvent `json:"tool_events"`
	ResponseID string           `json:"response_id"`
	Usage      AgentUsage       `json:"usage"`
}

type AgentModelOption struct {
	ID string `json:"id"`
}

type agentTool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Run         func(string) (any, error)
}

func (s *Service) GetAgentSettings() (AgentSettings, error) {
	return s.loadAgentSettings()
}

func (s *Service) SaveAgentSettings(settings AgentSettings) (AgentSettings, error) {
	settings = normalizeAgentSettings(settings)
	if err := s.saveAgentSettings(settings); err != nil {
		return AgentSettings{}, err
	}
	return settings, nil
}

func (s *Service) ListAgentModels() ([]AgentModelOption, error) {
	settings, err := s.loadAgentSettings()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return nil, fmt.Errorf("openai api key is required")
	}

	client := openai.NewClient(option.WithAPIKey(settings.APIKey))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.Models.List(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]AgentModelOption, 0, len(page.Data))
	for _, item := range page.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		items = append(items, AgentModelOption{ID: item.ID})
	}

	slices.SortFunc(items, func(a, b AgentModelOption) int {
		return strings.Compare(a.ID, b.ID)
	})
	return items, nil
}

func (s *Service) ChatWithAgent(req AgentChatRequest) (AgentChatResponse, error) {
	settings, err := s.loadAgentSettings()
	if err != nil {
		return AgentChatResponse{}, err
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return AgentChatResponse{}, fmt.Errorf("openai api key is required")
	}
	if len(req.Messages) == 0 {
		return AgentChatResponse{}, fmt.Errorf("at least one chat message is required")
	}

	client := openai.NewClient(option.WithAPIKey(settings.APIKey))
	tools := s.agentTools()
	instructions := combineInstructions(settings.SystemPrompt)
	inputItems := make([]responses.ResponseInputItemUnionParam, 0, len(req.Messages))
	for _, message := range req.Messages {
		role := normalizeAgentRole(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" || content == "" {
			continue
		}
		inputItems = append(inputItems, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRole(role)))
	}
	if len(inputItems) == 0 {
		return AgentChatResponse{}, fmt.Errorf("at least one non-empty user or assistant message is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Instructions: openai.String(instructions),
		Model:        openai.ChatModel(settings.Model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: inputItems,
		},
		ParallelToolCalls: openai.Bool(true),
		Reasoning:         reasoningParam(settings.ReasoningEffort),
		Tools:             s.agentToolParams(tools),
	})
	if err != nil {
		return AgentChatResponse{}, err
	}

	toolEvents := make([]AgentToolEvent, 0)
	for step := 0; step < maxAgentSteps; step++ {
		outputs := make([]responses.ResponseInputItemUnionParam, 0)
		functionCalls := 0

		for _, item := range response.Output {
			if item.Type != "function_call" {
				continue
			}

			functionCalls++
			call := item.AsFunctionCall()
			output, event := runAgentTool(tools, call.Name, call.Arguments, call.CallID)
			toolEvents = append(toolEvents, event)
			outputs = append(outputs, responses.ResponseInputItemParamOfFunctionCallOutput(call.CallID, output))
		}

		if functionCalls == 0 {
			return AgentChatResponse{
				Message: AgentChatMessage{
					Role:    "assistant",
					Content: strings.TrimSpace(response.OutputText()),
				},
				ToolEvents: toolEvents,
				ResponseID: response.ID,
				Usage: AgentUsage{
					InputTokens:     response.Usage.InputTokens,
					OutputTokens:    response.Usage.OutputTokens,
					ReasoningTokens: response.Usage.OutputTokensDetails.ReasoningTokens,
					TotalTokens:     response.Usage.TotalTokens,
				},
			}, nil
		}

		response, err = client.Responses.New(ctx, responses.ResponseNewParams{
			Instructions:       openai.String(instructions),
			Model:              openai.ChatModel(settings.Model),
			PreviousResponseID: openai.String(response.ID),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: outputs,
			},
			Reasoning: reasoningParam(settings.ReasoningEffort),
		})
		if err != nil {
			return AgentChatResponse{}, err
		}
	}

	return AgentChatResponse{}, fmt.Errorf("agent exceeded the maximum tool-call depth")
}

func (s *Service) agentToolParams(tools []agentTool) []responses.ToolUnionParam {
	result := make([]responses.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		result = append(result, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  tool.Parameters,
				Strict:      openai.Bool(true),
			},
		})
	}
	return result
}

func (s *Service) agentTools() []agentTool {
	return []agentTool{
		{
			Name:        "get_diagnostics",
			Description: "Inspect backend readiness, platform details, and capability availability.",
			Parameters:  objectSchema(map[string]any{"mutable": boolSchema("Whether to request mutable evaluation gates.")}),
			Run: func(raw string) (any, error) {
				var payload struct {
					Mutable bool `json:"mutable"`
				}
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.GetDiagnostics(payload.Mutable)
			},
		},
		{
			Name:        "type_text",
			Description: "Type plain text into the currently focused application.",
			Parameters: objectSchema(map[string]any{
				"text":          stringSchema("Text to type into the active application."),
				"auto_delay_ms": integerSchema("Optional key delay in milliseconds."),
			}, "text"),
			Run: func(raw string) (any, error) {
				var payload KeyboardTextRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.TypeText(payload)
			},
		},
		{
			Name:        "tap_keys",
			Description: "Tap one or more keys, optionally as a key chord.",
			Parameters:  keyboardKeysSchema(),
			Run: func(raw string) (any, error) {
				var payload KeyboardKeysRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.TapKeys(payload)
			},
		},
		{
			Name:        "press_keys",
			Description: "Press one or more keys without releasing them.",
			Parameters:  keyboardKeysSchema(),
			Run: func(raw string) (any, error) {
				var payload KeyboardKeysRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.PressKeys(payload)
			},
		},
		{
			Name:        "release_keys",
			Description: "Release one or more keys that were previously pressed.",
			Parameters:  keyboardKeysSchema(),
			Run: func(raw string) (any, error) {
				var payload KeyboardKeysRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.ReleaseKeys(payload)
			},
		},
		{
			Name:        "get_mouse_position",
			Description: "Read the current pointer position.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.GetMousePosition()
			},
		},
		{
			Name:        "set_mouse_position",
			Description: "Move the pointer directly to a screen coordinate.",
			Parameters: objectSchema(map[string]any{
				"x":             integerSchema("Target x-coordinate."),
				"y":             integerSchema("Target y-coordinate."),
				"auto_delay_ms": integerSchema("Optional mouse delay in milliseconds."),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				var payload MouseMoveRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.SetMousePosition(payload)
			},
		},
		{
			Name:        "move_mouse_line",
			Description: "Move the pointer along a straight path to a coordinate.",
			Parameters: objectSchema(map[string]any{
				"x":             integerSchema("Target x-coordinate."),
				"y":             integerSchema("Target y-coordinate."),
				"speed":         integerSchema("Optional pointer speed."),
				"auto_delay_ms": integerSchema("Optional mouse delay in milliseconds."),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				var payload MouseLineRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.MoveMouseLine(payload)
			},
		},
		{
			Name:        "click_mouse",
			Description: "Click a mouse button.",
			Parameters:  mouseButtonSchema(),
			Run: func(raw string) (any, error) {
				var payload MouseButtonRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.ClickMouse(payload)
			},
		},
		{
			Name:        "double_click_mouse",
			Description: "Double-click a mouse button.",
			Parameters:  mouseButtonSchema(),
			Run: func(raw string) (any, error) {
				var payload MouseButtonRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.DoubleClickMouse(payload)
			},
		},
		{
			Name:        "scroll_mouse",
			Description: "Scroll the mouse in one of four directions.",
			Parameters: objectSchema(map[string]any{
				"direction":     enumSchema("Scroll direction.", "up", "down", "left", "right"),
				"amount":        integerSchema("Scroll amount."),
				"auto_delay_ms": integerSchema("Optional mouse delay in milliseconds."),
			}, "direction", "amount"),
			Run: func(raw string) (any, error) {
				var payload MouseScrollRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.ScrollMouse(payload)
			},
		},
		{
			Name:        "drag_mouse",
			Description: "Drag the mouse from one point to another with the left button.",
			Parameters: objectSchema(map[string]any{
				"from_x":        integerSchema("Start x-coordinate."),
				"from_y":        integerSchema("Start y-coordinate."),
				"to_x":          integerSchema("Destination x-coordinate."),
				"to_y":          integerSchema("Destination y-coordinate."),
				"speed":         integerSchema("Optional pointer speed."),
				"auto_delay_ms": integerSchema("Optional mouse delay in milliseconds."),
			}, "from_x", "from_y", "to_x", "to_y"),
			Run: func(raw string) (any, error) {
				var payload MouseDragRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.DragMouse(payload)
			},
		},
		{
			Name:        "get_screen_size",
			Description: "Read the current primary screen size.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.GetScreenSize()
			},
		},
		{
			Name:        "capture_screen",
			Description: "Capture the full screen to a file under the artifacts directory.",
			Parameters: objectSchema(map[string]any{
				"file_name": stringSchema("Optional file name for the capture."),
			}),
			Run: func(raw string) (any, error) {
				var payload CaptureRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.CaptureScreen(payload)
			},
		},
		{
			Name:        "capture_region",
			Description: "Capture a screen region to a file under the artifacts directory.",
			Parameters: objectSchema(map[string]any{
				"file_name": stringSchema("Optional file name for the capture."),
				"region":    regionSchema("Region to capture."),
			}, "region"),
			Run: func(raw string) (any, error) {
				var payload CaptureRegionRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.CaptureRegion(payload)
			},
		},
		{
			Name:        "color_at",
			Description: "Read the pixel color at a screen coordinate.",
			Parameters: objectSchema(map[string]any{
				"x": integerSchema("Target x-coordinate."),
				"y": integerSchema("Target y-coordinate."),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				var payload PointRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.ColorAt(payload)
			},
		},
		{
			Name:        "highlight_region",
			Description: "Highlight a region on the screen.",
			Parameters:  regionSchema("Region to highlight."),
			Run: func(raw string) (any, error) {
				var payload Region
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.HighlightRegion(payload)
			},
		},
		{
			Name:        "list_windows",
			Description: "Enumerate visible windows through the native window provider.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.ListWindows()
			},
		},
		{
			Name:        "get_active_window",
			Description: "Read the current active window.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.GetActiveWindow()
			},
		},
		{
			Name:        "focus_window",
			Description: "Focus a window by handle.",
			Parameters:  windowHandleSchema("Window handle to focus."),
			Run: func(raw string) (any, error) {
				var payload WindowHandleRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.FocusWindow(payload)
			},
		},
		{
			Name:        "minimize_window",
			Description: "Attempt to minimize a window by handle.",
			Parameters:  windowHandleSchema("Window handle to minimize."),
			Run: func(raw string) (any, error) {
				var payload WindowHandleRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.MinimizeWindow(payload)
			},
		},
		{
			Name:        "restore_window",
			Description: "Attempt to restore a window by handle.",
			Parameters:  windowHandleSchema("Window handle to restore."),
			Run: func(raw string) (any, error) {
				var payload WindowHandleRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.RestoreWindow(payload)
			},
		},
		{
			Name:        "move_window",
			Description: "Move a window to a new screen coordinate.",
			Parameters: objectSchema(map[string]any{
				"handle": integerSchema("Window handle."),
				"x":      integerSchema("Target x-coordinate."),
				"y":      integerSchema("Target y-coordinate."),
			}, "handle", "x", "y"),
			Run: func(raw string) (any, error) {
				var payload WindowMoveRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.MoveWindow(payload)
			},
		},
		{
			Name:        "resize_window",
			Description: "Resize a window by handle.",
			Parameters: objectSchema(map[string]any{
				"handle": integerSchema("Window handle."),
				"width":  integerSchema("Window width."),
				"height": integerSchema("Window height."),
			}, "handle", "width", "height"),
			Run: func(raw string) (any, error) {
				var payload WindowResizeRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.ResizeWindow(payload)
			},
		},
		{
			Name:        "find_color",
			Description: "Find a color on screen, optionally constrained to a region.",
			Parameters:  colorQuerySchema(),
			Run: func(raw string) (any, error) {
				var payload ColorQueryRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.FindColor(payload)
			},
		},
		{
			Name:        "wait_for_color",
			Description: "Wait until a color appears on screen.",
			Parameters:  colorQuerySchema(),
			Run: func(raw string) (any, error) {
				var payload ColorQueryRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.WaitForColor(payload)
			},
		},
		{
			Name:        "assert_color_visible",
			Description: "Assert that a color is currently visible on screen.",
			Parameters:  colorQuerySchema(),
			Run: func(raw string) (any, error) {
				var payload ColorQueryRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.AssertColorVisible(payload)
			},
		},
		{
			Name:        "find_window_by_title",
			Description: "Find a window by exact title or regular expression.",
			Parameters:  windowQuerySchema(),
			Run: func(raw string) (any, error) {
				var payload WindowQueryRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.FindWindowByTitle(payload)
			},
		},
		{
			Name:        "wait_for_window_by_title",
			Description: "Wait until a window title is visible.",
			Parameters:  windowQuerySchema(),
			Run: func(raw string) (any, error) {
				var payload WindowQueryRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.WaitForWindowByTitle(payload)
			},
		},
		{
			Name:        "assert_window_visible",
			Description: "Assert that a window title is currently visible.",
			Parameters:  windowQuerySchema(),
			Run: func(raw string) (any, error) {
				var payload WindowQueryRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.AssertWindowVisible(payload)
			},
		},
		{
			Name:        "clipboard_copy",
			Description: "Copy text into the system clipboard.",
			Parameters: objectSchema(map[string]any{
				"text": stringSchema("Text to copy."),
			}, "text"),
			Run: func(raw string) (any, error) {
				var payload ClipboardCopyRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.ClipboardCopy(payload)
			},
		},
		{
			Name:        "clipboard_paste",
			Description: "Paste text from the system clipboard.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.ClipboardPaste()
			},
		},
		{
			Name:        "clipboard_clear",
			Description: "Clear clipboard text if supported by the current backend.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.ClipboardClear()
			},
		},
		{
			Name:        "clipboard_has_text",
			Description: "Check whether the clipboard currently contains text.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return s.ClipboardHasText()
			},
		},
	}
}

func runAgentTool(tools []agentTool, name string, raw string, callID string) (string, AgentToolEvent) {
	event := AgentToolEvent{
		CallID:    callID,
		Name:      name,
		Arguments: raw,
	}

	for _, tool := range tools {
		if tool.Name != name {
			continue
		}

		result, err := tool.Run(raw)
		if err != nil {
			event.Error = err.Error()
			event.Output = marshalToolPayload(map[string]any{"error": err.Error()})
			return event.Output, event
		}

		event.Output = marshalToolPayload(result)
		return event.Output, event
	}

	event.Error = fmt.Sprintf("unknown tool %q", name)
	event.Output = marshalToolPayload(map[string]any{"error": event.Error})
	return event.Output, event
}

func agentDeveloperPrompt() string {
	return strings.TrimSpace(`You are a desktop automation assistant inside gutgd.
Use the provided tools whenever the user asks you to inspect or control the desktop environment.
Prefer the smallest number of tool calls needed to satisfy the request.
Summarize what you did, include notable tool results, and be explicit when the native backend reports an unsupported capability.
Do not invent tool results.`)
}

func combineInstructions(systemPrompt string) string {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		return agentDeveloperPrompt()
	}
	return agentDeveloperPrompt() + "\n\nAdditional system prompt:\n" + systemPrompt
}

func reasoningParam(value string) shared.ReasoningParam {
	switch normalizeReasoningEffort(value) {
	case "none":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortNone}
	case "minimal":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortMinimal}
	case "low":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortLow}
	case "high":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortHigh}
	case "xhigh":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortXhigh}
	default:
		return shared.ReasoningParam{Effort: shared.ReasoningEffortMedium}
	}
}

func normalizeAgentRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	case "developer":
		return "developer"
	case "system":
		return "system"
	default:
		return ""
	}
}

func decodeToolArgs(raw string, target any) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = "{}"
	}
	return json.Unmarshal([]byte(value), target)
}

func marshalToolPayload(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\":%q}", err.Error())
	}
	return string(data)
}

func emptyObjectSchema() map[string]any {
	return objectSchema(map[string]any{})
}

func keyboardKeysSchema() map[string]any {
	return objectSchema(map[string]any{
		"keys": arraySchema("List of keys such as [\"ctrl\", \"c\"] or [\"enter\"].", map[string]any{
			"type": "string",
		}),
		"auto_delay_ms": integerSchema("Optional key delay in milliseconds."),
	}, "keys")
}

func mouseButtonSchema() map[string]any {
	return objectSchema(map[string]any{
		"button":        enumSchema("Mouse button token.", "left", "middle", "right"),
		"auto_delay_ms": integerSchema("Optional mouse delay in milliseconds."),
	}, "button")
}

func windowHandleSchema(description string) map[string]any {
	return objectSchema(map[string]any{
		"handle": integerSchema(description),
	}, "handle")
}

func colorQuerySchema() map[string]any {
	return objectSchema(map[string]any{
		"r":           integerSchema("Red component 0-255."),
		"g":           integerSchema("Green component 0-255."),
		"b":           integerSchema("Blue component 0-255."),
		"a":           integerSchema("Alpha component 0-255."),
		"region":      regionSchema("Optional search region."),
		"timeout_ms":  integerSchema("Optional timeout in milliseconds."),
		"interval_ms": integerSchema("Optional polling interval in milliseconds."),
	}, "r", "g", "b", "a")
}

func windowQuerySchema() map[string]any {
	return objectSchema(map[string]any{
		"title":       stringSchema("Window title or regular expression."),
		"use_regex":   boolSchema("Whether title should be treated as a regular expression."),
		"timeout_ms":  integerSchema("Optional timeout in milliseconds."),
		"interval_ms": integerSchema("Optional polling interval in milliseconds."),
	}, "title")
}

func regionSchema(description string) map[string]any {
	schema := objectSchema(map[string]any{
		"left":   integerSchema("Left coordinate."),
		"top":    integerSchema("Top coordinate."),
		"width":  integerSchema("Region width."),
		"height": integerSchema("Region height."),
	}, "left", "top", "width", "height")
	if description != "" {
		schema["description"] = description
	}
	return schema
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	required = make([]string, 0, len(properties))
	for key := range properties {
		required = append(required, key)
	}
	slices.Sort(required)

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	schema["required"] = required
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func integerSchema(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

func boolSchema(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

func arraySchema(description string, items map[string]any) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       items,
	}
}

func enumSchema(description string, values ...string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        values,
	}
}
