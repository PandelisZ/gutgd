package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	maxAgentContinues = 3
	continueMessage   = "continue"
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
	Messages           []AgentChatMessage `json:"messages"`
	PreviousResponseID string             `json:"previous_response_id"`
	ClientRunID        string             `json:"client_run_id"`
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
	Message    AgentChatMessage      `json:"message"`
	ToolEvents []AgentToolEvent      `json:"tool_events"`
	Items      []AgentTranscriptItem `json:"items"`
	ResponseID string                `json:"response_id"`
	Usage      AgentUsage            `json:"usage"`
}

type AgentTranscriptItem struct {
	Kind      string `json:"kind"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	CallID    string `json:"call_id"`
	Content   string `json:"content"`
	Arguments string `json:"arguments"`
	Output    string `json:"output"`
	Error     string `json:"error"`
}

type AgentProgressEvent struct {
	RunID      string               `json:"run_id"`
	Kind       string               `json:"kind"`
	Status     string               `json:"status"`
	Item       *AgentTranscriptItem `json:"item"`
	ToolEvent  *AgentToolEvent      `json:"tool_event"`
	ResponseID string               `json:"response_id"`
}

type AgentModelOption struct {
	ID string `json:"id"`
}

type AnalyzeScreenshotRequest struct {
	Path   string `json:"path"`
	Prompt string `json:"prompt"`
	Offset Point  `json:"offset"`
	Scale  Scale  `json:"scale"`
	Detail string `json:"detail"`
}

type AnalyzeScreenshotResult struct {
	Path     string `json:"path"`
	Offset   Point  `json:"offset"`
	Scale    Scale  `json:"scale"`
	Analysis string `json:"analysis"`
}

type AgentCoordinateSpace struct {
	Mode    string         `json:"mode"`
	Origin  Point          `json:"origin"`
	Window  *WindowSummary `json:"window,omitempty"`
	Message string         `json:"message"`
}

type AgentTranslatedPointResult struct {
	CoordinateSpace AgentCoordinateSpace `json:"coordinate_space"`
	Requested       Point                `json:"requested"`
	ScreenPoint     Point                `json:"screen_point"`
	Result          any                  `json:"result"`
}

type AgentTranslatedRegionResult struct {
	CoordinateSpace AgentCoordinateSpace `json:"coordinate_space"`
	Requested       Region               `json:"requested"`
	ScreenRegion    Region               `json:"screen_region"`
	Result          any                  `json:"result"`
}

type agentCoordinateState struct {
	Mode   string
	Window *WindowSummary
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
	coordinateState := s.loadAgentCoordinateState(req.PreviousResponseID)
	tools := s.agentToolsWithState(coordinateState)
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

	ctx := context.Background()
	runID := strings.TrimSpace(req.ClientRunID)
	if runID == "" {
		runID = fmt.Sprintf("agent-%d", time.Now().UnixNano())
	}
	s.emitAgentProgress(runID, AgentProgressEvent{
		RunID:  runID,
		Kind:   "status",
		Status: "Running agent…",
	})

	params := responses.ResponseNewParams{
		Instructions: openai.String(instructions),
		Model:        openai.ChatModel(settings.Model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: inputItems,
		},
		ParallelToolCalls: openai.Bool(true),
		Include:           []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent},
		Reasoning:         reasoningParam(settings.ReasoningEffort),
		Tools:             s.agentToolParams(tools),
	}
	if previousResponseID := strings.TrimSpace(req.PreviousResponseID); previousResponseID != "" {
		params.PreviousResponseID = openai.String(previousResponseID)
	}

	response, err := client.Responses.New(ctx, params)
	if err != nil {
		return AgentChatResponse{}, err
	}
	s.emitAgentProgress(runID, AgentProgressEvent{
		RunID:  runID,
		Kind:   "status",
		Status: "Model responded. Processing tool calls…",
	})

	toolEvents := make([]AgentToolEvent, 0)
	transcriptItems := make([]AgentTranscriptItem, 0)
	totalUsage := AgentUsage{}
	accumulateAgentUsage(&totalUsage, response)
	lastToolStepSignature := ""
	continueCount := 0
	for step := 0; ; step++ {
		outputs := make([]responses.ResponseInputItemUnionParam, 0)
		functionCalls := make([]responses.ResponseFunctionToolCall, 0)

		for _, item := range response.Output {
			nextItems := transcriptItemsFromResponseOutput(item)
			transcriptItems = append(transcriptItems, nextItems...)
			for _, transcriptItem := range nextItems {
				itemCopy := transcriptItem
				s.emitAgentProgress(runID, AgentProgressEvent{
					RunID:  runID,
					Kind:   "item",
					Item:   &itemCopy,
					Status: "Updated transcript.",
				})
			}

			if item.Type != "function_call" {
				continue
			}

			functionCalls = append(functionCalls, item.AsFunctionCall())
		}

		if len(functionCalls) == 0 {
			finalMessage := strings.TrimSpace(response.OutputText())
			if finalMessage == "" {
				finalMessage = latestAssistantMessage(transcriptItems)
			}
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:      runID,
				Kind:       "complete",
				Status:     "Agent response ready.",
				ResponseID: response.ID,
			})
			s.saveAgentCoordinateState(response.ID, coordinateState)
			return AgentChatResponse{
				Message: AgentChatMessage{
					Role:    "assistant",
					Content: finalMessage,
				},
				ToolEvents: toolEvents,
				Items:      transcriptItems,
				ResponseID: response.ID,
				Usage:      totalUsage,
			}, nil
		}

		signature := agentToolStepSignature(functionCalls)
		if signature != "" && signature == lastToolStepSignature {
			message := repeatedToolCallMessage(functionCalls)
			transcriptItems = append(transcriptItems, AgentTranscriptItem{
				Kind:    "message",
				Role:    "assistant",
				Content: message,
			})
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:  runID,
				Kind:   "complete",
				Status: message,
			})
			return AgentChatResponse{
				Message: AgentChatMessage{
					Role:    "assistant",
					Content: message,
				},
				ToolEvents: toolEvents,
				Items:      transcriptItems,
				ResponseID: "",
				Usage:      totalUsage,
			}, nil
		}
		lastToolStepSignature = signature

		for _, call := range functionCalls {
			output, event := runAgentTool(tools, call.Name, call.Arguments, call.CallID)
			toolEvents = append(toolEvents, event)
			callItem := AgentTranscriptItem{
				Kind:      "tool_call",
				Name:      call.Name,
				CallID:    call.CallID,
				Arguments: call.Arguments,
			}
			outputItem := AgentTranscriptItem{
				Kind:   "tool_output",
				Name:   call.Name,
				CallID: call.CallID,
				Output: output,
				Error:  event.Error,
			}
			transcriptItems = append(transcriptItems, callItem, outputItem)
			callItemCopy := callItem
			outputItemCopy := outputItem
			eventCopy := event
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:  runID,
				Kind:   "item",
				Item:   &callItemCopy,
				Status: "Calling tool…",
			})
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:  runID,
				Kind:   "item",
				Item:   &outputItemCopy,
				Status: "Tool call completed.",
			})
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:     runID,
				Kind:      "tool_event",
				ToolEvent: &eventCopy,
				Status:    "Tool event received.",
			})
			outputs = append(outputs, agentFunctionCallOutput(call.CallID, output))
		}

		if step+1 >= maxAgentSteps {
			if continueCount >= maxAgentContinues {
				message := "The agent reached the maximum tool-call depth before producing a final answer. Review the tool activity above and refine the request if needed."
				transcriptItems = appendAssistantAfterTrailingContinue(transcriptItems, AgentTranscriptItem{
					Kind:    "message",
					Role:    "assistant",
					Content: message,
				})
				s.emitAgentProgress(runID, AgentProgressEvent{
					RunID:  runID,
					Kind:   "complete",
					Status: message,
				})
				return AgentChatResponse{
					Message: AgentChatMessage{
						Role:    "assistant",
						Content: message,
					},
					ToolEvents: toolEvents,
					Items:      transcriptItems,
					ResponseID: "",
					Usage:      totalUsage,
				}, nil
			}

			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:  runID,
				Kind:   "status",
				Status: "Deep tool chain reached. Sending continue.",
			})

			response, err = client.Responses.New(ctx, responses.ResponseNewParams{
				Instructions:       openai.String(instructions),
				Model:              openai.ChatModel(settings.Model),
				PreviousResponseID: openai.String(response.ID),
				Input: responses.ResponseNewParamsInputUnion{
					OfInputItemList: continueInputItems(outputs),
				},
				ParallelToolCalls: openai.Bool(true),
				Include:           []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent},
				Reasoning:         reasoningParam(settings.ReasoningEffort),
				Tools:             s.agentToolParams(tools),
			})
			if err != nil {
				return AgentChatResponse{}, err
			}
			accumulateAgentUsage(&totalUsage, response)
			continueItem := AgentTranscriptItem{
				Kind:    "message",
				Role:    "user",
				Content: continueMessage,
			}
			transcriptItems = append(transcriptItems, continueItem)
			continueItemCopy := continueItem
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:  runID,
				Kind:   "item",
				Item:   &continueItemCopy,
				Status: "Continue prompt sent.",
			})
			s.emitAgentProgress(runID, AgentProgressEvent{
				RunID:  runID,
				Kind:   "status",
				Status: "Continue response received. Processing next step…",
			})
			continueCount++
			step = -1
			continue
		}

		nextParams := responses.ResponseNewParams{
			Instructions:       openai.String(instructions),
			Model:              openai.ChatModel(settings.Model),
			PreviousResponseID: openai.String(response.ID),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: outputs,
			},
			ParallelToolCalls: openai.Bool(true),
			Include:           []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent},
			Reasoning:         reasoningParam(settings.ReasoningEffort),
			Tools:             s.agentToolParams(tools),
		}

		response, err = client.Responses.New(ctx, nextParams)
		if err != nil {
			return AgentChatResponse{}, err
		}
		accumulateAgentUsage(&totalUsage, response)
		s.emitAgentProgress(runID, AgentProgressEvent{
			RunID:  runID,
			Kind:   "status",
			Status: "Tool outputs submitted. Waiting for model…",
		})
	}
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

func (s *Service) emitAgentProgress(runID string, event AgentProgressEvent) {
	if s == nil || s.emitEvent == nil {
		return
	}
	if strings.TrimSpace(event.RunID) == "" {
		event.RunID = runID
	}
	s.emitEvent("agent_progress", event)
}

func (s *Service) agentTools() []agentTool {
	return s.agentToolsForGOOS(runtime.GOOS)
}

func (s *Service) agentToolsForGOOS(goos string) []agentTool {
	return s.agentToolsForGOOSWithState(goos, newAgentCoordinateState())
}

func (s *Service) agentToolsWithState(state *agentCoordinateState) []agentTool {
	if state == nil {
		state = newAgentCoordinateState()
	}
	return s.agentToolsForGOOSWithState(runtime.GOOS, state)
}

func (s *Service) agentToolsForGOOSWithState(goos string, state *agentCoordinateState) []agentTool {
	if state == nil {
		state = newAgentCoordinateState()
	}
	captureScreenDescription := "Capture the full screen to a file under the artifacts directory."
	captureRegionDescription := "Capture a screen region to a file under the artifacts directory."
	pressSpecialKeyDescription := "Press a non-text special key such as enter, return, tab, escape, space, backspace, delete, or the arrow keys. Use this after typing when you need to submit a text field or move focus without relying on low-level key chord tools."
	if goos == "darwin" {
		captureScreenDescription = "Capture the full screen to a file under the artifacts directory. On macOS 26+, this uses the safe OS screenshot fallback instead of the hidden native screen.capture path."
		captureRegionDescription = "Capture a screen region to a file under the artifacts directory. On macOS 26+, this uses the safe OS screenshot fallback instead of the hidden native screen.capture path."
		pressSpecialKeyDescription = "Press a non-text special key such as enter, return, tab, escape, space, backspace, delete, or the arrow keys. On macOS 26+, this uses the safe special-key path instead of the hidden low-level key chord tools."
	}

	tools := []agentTool{
		{
			Name:        "get_coordinate_space",
			Description: "Read the current coordinate space. Screen space uses absolute screen coordinates. Window space uses coordinates relative to the selected window's top-left corner.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				return coordinateSpaceView(*state, "Current coordinate space."), nil
			},
		},
		{
			Name:        "switch_to_active_window_space",
			Description: "Switch the coordinate space to the current active window. After this, x/y inputs for pointer and capture-region tools are interpreted relative to that window's top-left corner.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				window, err := s.GetActiveWindow()
				if err != nil {
					return nil, err
				}
				state.Mode = "window"
				state.Window = cloneWindowSummary(window)
				return coordinateSpaceView(*state, fmt.Sprintf("Switched to window space for %q.", window.Title)), nil
			},
		},
		{
			Name:        "switch_to_window_space",
			Description: "Switch the coordinate space to a specific window by handle. After this, x/y inputs for pointer and capture-region tools are interpreted relative to that window's top-left corner.",
			Parameters:  windowHandleSchema("Window handle to use as the window-space origin."),
			Run: func(raw string) (any, error) {
				var payload WindowHandleRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				window, err := s.FindWindowByHandle(payload)
				if err != nil {
					return nil, err
				}
				state.Mode = "window"
				state.Window = cloneWindowSummary(window)
				return coordinateSpaceView(*state, fmt.Sprintf("Switched to window space for %q.", window.Title)), nil
			},
		},
		{
			Name:        "switch_to_screen_space",
			Description: "Exit window space and return to absolute screen coordinates.",
			Parameters:  emptyObjectSchema(),
			Run: func(raw string) (any, error) {
				state.Mode = "screen"
				state.Window = nil
				return coordinateSpaceView(*state, "Switched to screen space."), nil
			},
		},
		{
			Name:        "type_text",
			Description: "Type plain text into the currently focused application. Use this for words, sentences, paragraphs, or any multi-character text input instead of spelling characters out with tap_keys.",
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
			Name:        "type_text_block",
			Description: "Type a full sentence, multiple sentences, or a paragraph into the currently focused application in a single tool call. Prefer this over tap_keys when entering natural language text.",
			Parameters: objectSchema(map[string]any{
				"text":          stringSchema("Full text to type into the active application."),
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
			Name:        "press_special_key",
			Description: pressSpecialKeyDescription,
			Parameters: objectSchema(map[string]any{
				"key":          enumSchema("Special key to press.", "enter", "return", "tab", "escape", "esc", "space", "backspace", "delete", "forward_delete", "up", "down", "left", "right"),
				"repeat_count": integerSchema("Optional number of times to press the key."),
			}, "key"),
			Run: func(raw string) (any, error) {
				var payload KeyboardSpecialKeyRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				return s.PressSpecialKey(payload)
			},
		},
		{
			Name:        "analyze_screenshot",
			Description: "Analyze a previously captured screenshot or region and explain what is visible relative to the provided screen offset and scale. For visually precise clicks, prefer capturing the active window or a smaller region first, then use this tool to convert image-local targets back into screen coordinates.",
			Parameters: objectSchema(map[string]any{
				"path":   stringSchema("Path returned by capture_screen or capture_region."),
				"prompt": stringSchema("What to analyze in the screenshot, including the target you need to locate."),
				"offset": objectSchema(map[string]any{
					"x": integerSchema("Screen x-coordinate of the screenshot's top-left corner."),
					"y": integerSchema("Screen y-coordinate of the screenshot's top-left corner."),
				}, "x", "y"),
				"scale": objectSchema(map[string]any{
					"x": numberSchema("Horizontal screenshot pixel scale relative to screen coordinates. Use the scale returned by capture_screen or capture_region."),
					"y": numberSchema("Vertical screenshot pixel scale relative to screen coordinates. Use the scale returned by capture_screen or capture_region."),
				}, "x", "y"),
				"detail": enumSchema("Optional vision detail level.", "low", "high", "auto", "original"),
			}, "path", "prompt", "offset"),
			Run: func(raw string) (any, error) {
				var payload AnalyzeScreenshotRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				settings, err := s.loadAgentSettings()
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(settings.APIKey) == "" {
					return nil, fmt.Errorf("openai api key is required")
				}
				client := openai.NewClient(option.WithAPIKey(settings.APIKey))
				return analyzeScreenshot(context.Background(), client, settings.Model, payload)
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
			Description: "Move the pointer directly to a coordinate in the current coordinate space. In window space, x/y are relative to the selected window's top-left corner.",
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
				requested := Point{X: payload.X, Y: payload.Y}
				screenPoint := translatePointToScreenSpace(requested, *state)
				payload.X = screenPoint.X
				payload.Y = screenPoint.Y
				result, err := s.SetMousePosition(payload)
				if err != nil {
					return nil, err
				}
				return AgentTranslatedPointResult{
					CoordinateSpace: coordinateSpaceView(*state, "Pointer target translated to screen space."),
					Requested:       requested,
					ScreenPoint:     screenPoint,
					Result:          result,
				}, nil
			},
		},
		{
			Name:        "move_mouse_line",
			Description: "Move the pointer along a straight path to a coordinate in the current coordinate space. In window space, x/y are relative to the selected window's top-left corner.",
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
				requested := Point{X: payload.X, Y: payload.Y}
				screenPoint := translatePointToScreenSpace(requested, *state)
				payload.X = screenPoint.X
				payload.Y = screenPoint.Y
				result, err := s.MoveMouseLine(payload)
				if err != nil {
					return nil, err
				}
				return AgentTranslatedPointResult{
					CoordinateSpace: coordinateSpaceView(*state, "Pointer path target translated to screen space."),
					Requested:       requested,
					ScreenPoint:     screenPoint,
					Result:          result,
				}, nil
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
			Description: "Drag the mouse from one point to another with the left button using the current coordinate space. In window space, both endpoints are relative to the selected window's top-left corner.",
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
				requestedFrom := Point{X: payload.FromX, Y: payload.FromY}
				requestedTo := Point{X: payload.ToX, Y: payload.ToY}
				screenFrom := translatePointToScreenSpace(requestedFrom, *state)
				screenTo := translatePointToScreenSpace(requestedTo, *state)
				payload.FromX = screenFrom.X
				payload.FromY = screenFrom.Y
				payload.ToX = screenTo.X
				payload.ToY = screenTo.Y
				result, err := s.DragMouse(payload)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"coordinate_space": coordinateSpaceView(*state, "Drag path translated to screen space."),
					"requested": map[string]Point{
						"from": requestedFrom,
						"to":   requestedTo,
					},
					"screen_points": map[string]Point{
						"from": screenFrom,
						"to":   screenTo,
					},
					"result": result,
				}, nil
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
			Description: captureScreenDescription,
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
			Description: captureRegionDescription + " In window space, the region is relative to the selected window's top-left corner.",
			Parameters: objectSchema(map[string]any{
				"file_name": stringSchema("Optional file name for the capture."),
				"region":    regionSchema("Region to capture."),
			}, "region"),
			Run: func(raw string) (any, error) {
				var payload CaptureRegionRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				requested := payload.Region
				payload.Region = translateRegionToScreenSpace(payload.Region, *state)
				result, err := s.CaptureRegion(payload)
				if err != nil {
					return nil, err
				}
				return AgentTranslatedRegionResult{
					CoordinateSpace: coordinateSpaceView(*state, "Capture region translated to screen space."),
					Requested:       requested,
					ScreenRegion:    payload.Region,
					Result:          result,
				}, nil
			},
		},
		{
			Name:        "color_at",
			Description: "Read the pixel color at a coordinate in the current coordinate space. In window space, x/y are relative to the selected window's top-left corner.",
			Parameters: objectSchema(map[string]any{
				"x": integerSchema("Target x-coordinate."),
				"y": integerSchema("Target y-coordinate."),
			}, "x", "y"),
			Run: func(raw string) (any, error) {
				var payload PointRequest
				if err := decodeToolArgs(raw, &payload); err != nil {
					return nil, err
				}
				requested := Point{X: payload.X, Y: payload.Y}
				screenPoint := translatePointToScreenSpace(requested, *state)
				payload.X = screenPoint.X
				payload.Y = screenPoint.Y
				result, err := s.ColorAt(payload)
				if err != nil {
					return nil, err
				}
				return AgentTranslatedPointResult{
					CoordinateSpace: coordinateSpaceView(*state, "Color sample translated to screen space."),
					Requested:       requested,
					ScreenPoint:     screenPoint,
					Result:          result,
				}, nil
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

	if goos != "darwin" {
		tools = slices.DeleteFunc(tools, func(tool agentTool) bool {
			return tool.Name == "press_special_key"
		})
		tools = append(tools,
			agentTool{
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
			agentTool{
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
			agentTool{
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
			agentTool{
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
		)
	}

	return tools
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
	return agentDeveloperPromptForGOOS(runtime.GOOS)
}

func agentDeveloperPromptForGOOS(goos string) string {
	prompt := `You are a desktop automation assistant inside gutgd.
Use the provided tools whenever the user asks you to inspect or control the desktop environment.
Prefer the smallest number of tool calls needed to satisfy the request.
Summarize what you did, include notable tool results, and be explicit when the native backend reports an unsupported capability.
Do not treat GUT_ENABLE_LIVE_TESTS or the diagnostics field live_enabled as a blocker for normal gutgd actions. Those fields belong to the separate gut live-test harness. In gutgd, use the actual tool call result and the feature_status/capability availability to decide whether an action is possible.
When entering plain language text, sentences, or paragraphs into an application, prefer type_text or type_text_block. When you need to submit a text field or move focus with a non-text key, prefer press_special_key for enter, return, tab, escape, space, backspace, delete, or arrow keys. Use tap_keys only for non-text keys, shortcuts, or isolated single-key actions when that tool is available.
After a tool returns a concrete result or structured error, do not repeat the same tool call with identical arguments unless the user explicitly asked for a retry or the environment changed.
For visually precise clicks, do not guess repeatedly. First identify the relevant window with get_active_window or list_windows, then capture that window or a smaller region with capture_region, inspect it, choose a target, move the mouse, and if the target is small or ambiguous capture again to verify before clicking. If a click does not clearly change the UI, reassess with another capture instead of clicking nearby coordinates repeatedly.
Do not use full-screen capture for a precise click when the target is inside a single app window unless window discovery failed. Prefer smaller verification regions around the intended target after moving the pointer. Use get_active_window, list_windows, and capture_region to narrow the search area before clicking.
On macOS, screenshots may be retina-scaled, so image pixels are often 2x screen coordinates. The macOS menu bar is included in full-screen captures; do not add a separate menu-bar offset. Use the capture metadata to convert image coordinates back to screen coordinates.
capture_region returns the screen offset of the captured image and may include screenshot scale metadata. When you need to identify or click a visual target inside a screenshot, prefer analyze_screenshot with that path so the backend can recover the capture metadata and answer in screen coordinates. For small controls, rerun capture_region on a tighter area before the final click instead of reasoning from a broad capture.
When continuing an existing conversation through previous_response_id, rely on the server-side conversation state rather than asking the user to resend old turns.
Never print pseudo tool-call syntax such as "to=functions.*", JSON envelopes, or planning text in place of an actual tool call. If you need a tool, call it through the Responses function tool interface.
If multiple independent tool calls are needed in the same step, issue real parallel function calls rather than describing them in text.
Use common key aliases naturally: meta, super, cmd, and command all mean the platform meta key in this environment.
Do not narrate that you are about to call a tool. Call the tool first, then describe the result after the tool outputs are available.
Do not invent tool results.`
	if goos == "darwin" {
		prompt += `
On macOS 26+, capture_screen and capture_region use the safe OS screenshot fallback instead of the hidden native screen.capture capability. Prefer press_special_key for enter, return, tab, escape, space, backspace, delete, and arrow keys. Low-level tap_keys, press_keys, release_keys, and highlight_region are intentionally unavailable on macOS 26+.`
	}
	return strings.TrimSpace(prompt)
}

func latestAssistantMessage(items []AgentTranscriptItem) string {
	for index := len(items) - 1; index >= 0; index-- {
		if items[index].Kind == "message" && items[index].Role == "assistant" {
			return items[index].Content
		}
	}
	return ""
}

func agentToolStepSignature(calls []responses.ResponseFunctionToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	parts := make([]string, 0, len(calls))
	for _, call := range calls {
		parts = append(parts, call.Name+"|"+normalizeAgentToolArguments(call.Arguments))
	}
	return strings.Join(parts, "\n")
}

func normalizeAgentToolArguments(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "{}"
	}
	buffer := &bytes.Buffer{}
	if err := json.Compact(buffer, []byte(trimmed)); err == nil {
		return buffer.String()
	}
	return trimmed
}

func repeatedToolCallMessage(calls []responses.ResponseFunctionToolCall) string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return fmt.Sprintf("The agent stopped because it started repeating the same tool call pattern without making progress: %s.", strings.Join(names, ", "))
}

func agentFunctionCallOutput(callID string, output string) responses.ResponseInputItemUnionParam {
	if items, ok := imageFunctionCallOutputItems(output); ok {
		return responses.ResponseInputItemParamOfFunctionCallOutput(callID, items)
	}
	return responses.ResponseInputItemParamOfFunctionCallOutput(callID, output)
}

func imageFunctionCallOutputItems(output string) (responses.ResponseFunctionCallOutputItemListParam, bool) {
	capture, ok := captureResultFromOutput(output)
	if !ok {
		return nil, false
	}
	dataURL, err := imageDataURL(capture.Path)
	if err != nil {
		return nil, false
	}
	return responses.ResponseFunctionCallOutputItemListParam{
		{
			OfInputText: &responses.ResponseInputTextContentParam{
				Text: captureInstructionText(capture),
			},
		},
		{
			OfInputImage: &responses.ResponseInputImageContentParam{
				ImageURL: openai.String(dataURL),
				Detail:   responses.ResponseInputImageContentDetailHigh,
			},
		},
	}, true
}

func captureResultFromOutput(output string) (CaptureResult, bool) {
	var payload CaptureResult
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return CaptureResult{}, false
	}
	path := strings.TrimSpace(payload.Path)
	if path == "" {
		return CaptureResult{}, false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		payload.Path = path
		if payload.Scale.X <= 0 {
			payload.Scale.X = 1
		}
		if payload.Scale.Y <= 0 {
			payload.Scale.Y = 1
		}
		return payload, true
	default:
		return CaptureResult{}, false
	}
}

func captureInstructionText(capture CaptureResult) string {
	return strings.TrimSpace(fmt.Sprintf(
		`%s

Capture metadata:
- path: %s
- screen offset: (%d, %d)
- screenshot scale: x=%.4f, y=%.4f image pixels per screen coordinate

If you identify a target at image coordinates (image_x, image_y), convert it back to screen coordinates with:
- screen_x = %d + image_x / %.4f
- screen_y = %d + image_y / %.4f

On macOS, full-screen captures already include the menu bar. Do not add a separate menu-bar offset.`,
		capture.Message,
		capture.Path,
		capture.Offset.X,
		capture.Offset.Y,
		capture.Scale.X,
		capture.Scale.Y,
		capture.Offset.X,
		capture.Scale.X,
		capture.Offset.Y,
		capture.Scale.Y,
	))
}

func imageDataURL(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + imageMediaType(path) + ";base64," + base64.StdEncoding.EncodeToString(bytes), nil
}

func imageMediaType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func analyzeScreenshot(ctx context.Context, client openai.Client, model string, req AnalyzeScreenshotRequest) (AnalyzeScreenshotResult, error) {
	path := strings.TrimSpace(req.Path)
	prompt := strings.TrimSpace(req.Prompt)
	if path == "" {
		return AnalyzeScreenshotResult{}, fmt.Errorf("path is required")
	}
	if prompt == "" {
		return AnalyzeScreenshotResult{}, fmt.Errorf("prompt is required")
	}
	if metadata, err := readCaptureMetadata(path); err == nil {
		req.Offset = metadata.Offset
		req.Scale = metadata.Scale
	}
	dataURL, err := imageDataURL(path)
	if err != nil {
		return AnalyzeScreenshotResult{}, err
	}
	detail := normalizeImageDetailParam(req.Detail)
	scaleX := req.Scale.X
	if scaleX <= 0 {
		scaleX = 1
	}
	scaleY := req.Scale.Y
	if scaleY <= 0 {
		scaleY = 1
	}
	offsetText := fmt.Sprintf(
		"The screenshot top-left corner is at screen coordinates (%d, %d). The screenshot scale is x=%.4f and y=%.4f image pixels per screen coordinate. Convert image-local targets back into screen coordinates using screen_x = %d + image_x / %.4f and screen_y = %d + image_y / %.4f. Answer using screen coordinates when suggesting cursor targets.",
		req.Offset.X,
		req.Offset.Y,
		scaleX,
		scaleY,
		req.Offset.X,
		scaleX,
		req.Offset.Y,
		scaleY,
	)
	response, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: openai.ChatModel(model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: []responses.ResponseInputItemUnionParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						{
							OfInputText: &responses.ResponseInputTextParam{
								Text: offsetText + "\n\n" + prompt,
							},
						},
						{
							OfInputImage: &responses.ResponseInputImageParam{
								ImageURL: openai.String(dataURL),
								Detail:   detail,
							},
						},
					},
					responses.EasyInputMessageRoleUser,
				),
			},
		},
	})
	if err != nil {
		return AnalyzeScreenshotResult{}, err
	}
	return AnalyzeScreenshotResult{
		Path:     path,
		Offset:   req.Offset,
		Scale:    Scale{X: scaleX, Y: scaleY},
		Analysis: strings.TrimSpace(response.OutputText()),
	}, nil
}

func normalizeImageDetailParam(value string) responses.ResponseInputImageDetail {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return responses.ResponseInputImageDetailLow
	case "high":
		return responses.ResponseInputImageDetailHigh
	case "original":
		return responses.ResponseInputImageDetailOriginal
	default:
		return responses.ResponseInputImageDetailAuto
	}
}

func continueInputItems(outputs []responses.ResponseInputItemUnionParam) []responses.ResponseInputItemUnionParam {
	items := append(make([]responses.ResponseInputItemUnionParam, 0, len(outputs)+1), outputs...)
	items = append(items, responses.ResponseInputItemParamOfMessage(continueMessage, responses.EasyInputMessageRoleUser))
	return items
}

func appendAssistantAfterTrailingContinue(items []AgentTranscriptItem, assistant AgentTranscriptItem) []AgentTranscriptItem {
	if len(items) == 0 {
		return append(items, assistant)
	}

	last := items[len(items)-1]
	if last.Kind != "message" || last.Role != "user" || last.Content != continueMessage {
		return append(items, assistant)
	}

	next := append(make([]AgentTranscriptItem, 0, len(items)+1), items[:len(items)-1]...)
	next = append(next, assistant, last)
	return next
}

func transcriptItemsFromResponseOutput(item responses.ResponseOutputItemUnion) []AgentTranscriptItem {
	switch item.Type {
	case "message":
		message := item.AsMessage()
		text := strings.TrimSpace(outputMessageText(message))
		if text == "" {
			return nil
		}
		return []AgentTranscriptItem{{
			Kind:    "message",
			Role:    "assistant",
			Content: text,
		}}
	case "reasoning":
		reasoning := item.AsReasoning()
		text := strings.TrimSpace(reasoningItemText(reasoning))
		if text == "" {
			return nil
		}
		return []AgentTranscriptItem{{
			Kind:    "reasoning",
			Role:    "assistant",
			Content: text,
		}}
	default:
		return nil
	}
}

func responseOutputToInputItem(item responses.ResponseOutputItemUnion) (responses.ResponseInputItemUnionParam, bool) {
	switch item.Type {
	case "message":
		message := item.AsMessage().ToParam()
		return responses.ResponseInputItemUnionParam{OfOutputMessage: &message}, true
	case "reasoning":
		reasoning := item.AsReasoning().ToParam()
		return responses.ResponseInputItemUnionParam{OfReasoning: &reasoning}, true
	case "function_call":
		call := item.AsFunctionCall().ToParam()
		return responses.ResponseInputItemUnionParam{OfFunctionCall: &call}, true
	case "compaction":
		compaction := item.AsCompaction()
		return responses.ResponseInputItemParamOfCompaction(compaction.EncryptedContent), true
	default:
		return responses.ResponseInputItemUnionParam{}, false
	}
}

func outputMessageText(message responses.ResponseOutputMessage) string {
	parts := make([]string, 0, len(message.Content))
	for _, content := range message.Content {
		switch content.Type {
		case "output_text":
			if text := strings.TrimSpace(content.Text); text != "" {
				parts = append(parts, text)
			}
		case "refusal":
			if refusal := strings.TrimSpace(content.Refusal); refusal != "" {
				parts = append(parts, refusal)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func reasoningItemText(item responses.ResponseReasoningItem) string {
	parts := make([]string, 0, len(item.Summary)+len(item.Content))
	for _, summary := range item.Summary {
		if text := strings.TrimSpace(summary.Text); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	for _, content := range item.Content {
		if text := strings.TrimSpace(content.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func combineInstructions(systemPrompt string) string {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		return agentDeveloperPrompt()
	}
	return agentDeveloperPrompt() + "\n\nAdditional system prompt:\n" + systemPrompt
}

func accumulateAgentUsage(total *AgentUsage, response *responses.Response) {
	if total == nil || response == nil {
		return
	}
	total.InputTokens += response.Usage.InputTokens
	total.OutputTokens += response.Usage.OutputTokens
	total.ReasoningTokens += response.Usage.OutputTokensDetails.ReasoningTokens
	total.TotalTokens += response.Usage.TotalTokens
}

func reasoningParam(value string) shared.ReasoningParam {
	switch normalizeReasoningEffort(value) {
	case "none":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortNone, Summary: shared.ReasoningSummaryAuto}
	case "minimal":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortMinimal, Summary: shared.ReasoningSummaryAuto}
	case "low":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortLow, Summary: shared.ReasoningSummaryAuto}
	case "high":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortHigh, Summary: shared.ReasoningSummaryAuto}
	case "xhigh":
		return shared.ReasoningParam{Effort: shared.ReasoningEffortXhigh, Summary: shared.ReasoningSummaryAuto}
	default:
		return shared.ReasoningParam{Effort: shared.ReasoningEffortMedium, Summary: shared.ReasoningSummaryAuto}
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

func numberSchema(description string) map[string]any {
	return map[string]any{
		"type":        "number",
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
