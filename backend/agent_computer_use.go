package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

const (
	agentComputerToolName      = "computer"
	agentComputerWait          = 750 * time.Millisecond
	agentComputerSafetyAckCode = "unsupported_safety_acknowledgement"
)

type agentComputerState struct {
	LastCapture *CaptureResult
}

type agentPendingAction struct {
	Kind         string
	FunctionCall responses.ResponseFunctionToolCall
	ComputerCall responses.ResponseComputerToolCall
}

type agentComputerActionResult struct {
	Summary             string
	RequestedScreenshot bool
}

func cloneAgentComputerState(state *agentComputerState) *agentComputerState {
	if state == nil {
		return &agentComputerState{}
	}
	clone := &agentComputerState{}
	if state.LastCapture != nil {
		capture := normalizeCaptureResultMetadata(*state.LastCapture)
		clone.LastCapture = &capture
	}
	return clone
}

func computerCaptureRequest(screenSize Size) CaptureRequest {
	return CaptureRequest{
		MaxImageWidth:  screenSize.Width,
		MaxImageHeight: screenSize.Height,
	}
}

func (s *Service) captureComputerScreen() (CaptureResult, error) {
	screenSize, err := s.GetScreenSize()
	if err != nil {
		return CaptureResult{}, err
	}
	return s.CaptureScreen(computerCaptureRequest(Size{Width: screenSize.Width, Height: screenSize.Height}))
}

func (s *Service) loadAgentComputerState(previousResponseID string) *agentComputerState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(previousResponseID) == "" {
		return &agentComputerState{}
	}
	state, ok := s.agentComputerStates[previousResponseID]
	if !ok {
		return &agentComputerState{}
	}
	return cloneAgentComputerState(&state)
}

func (s *Service) saveAgentComputerState(responseID string, state *agentComputerState) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" || state == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentComputerStates[responseID] = *cloneAgentComputerState(state)
}

func agentActionStepSignature(actions []agentPendingAction) string {
	if len(actions) == 0 {
		return ""
	}

	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		switch action.Kind {
		case "function_call":
			parts = append(parts, action.FunctionCall.Name+"|"+normalizeAgentToolArguments(action.FunctionCall.Arguments))
		case "computer_call":
			parts = append(parts, agentComputerCallSignature(action.ComputerCall))
		}
	}
	return strings.Join(parts, "\n")
}

func agentComputerCallSignature(call responses.ResponseComputerToolCall) string {
	payload := map[string]any{
		"type":                  call.Type,
		"pending_safety_checks": call.PendingSafetyChecks,
	}
	if actions := agentComputerActions(call); len(actions) > 0 {
		payload["actions"] = actions
	}
	return agentComputerToolName + "|" + normalizeAgentToolArguments(marshalToolPayload(payload))
}

func agentComputerToolParam() responses.ToolUnionParam {
	tool := responses.NewComputerToolParam()
	return responses.ToolUnionParam{OfComputer: &tool}
}

func agentComputerActions(call responses.ResponseComputerToolCall) responses.ComputerActionList {
	if len(call.Actions) > 0 {
		return call.Actions
	}
	if call.Action.Type == "" {
		return nil
	}

	var legacy responses.ComputerActionUnion
	if err := json.Unmarshal([]byte(call.Action.RawJSON()), &legacy); err != nil {
		return nil
	}
	return responses.ComputerActionList{legacy}
}

func agentComputerCallOutput(callID string, capture CaptureResult) (responses.ResponseInputItemUnionParam, error) {
	dataURL, err := imageDataURL(capture.Path)
	if err != nil {
		return responses.ResponseInputItemUnionParam{}, err
	}

	payload, err := json.Marshal(map[string]any{
		"type":    "computer_call_output",
		"call_id": callID,
		"output": map[string]any{
			"type":      "computer_screenshot",
			"image_url": dataURL,
			"detail":    "original",
		},
	})
	if err != nil {
		return responses.ResponseInputItemUnionParam{}, err
	}

	return param.Override[responses.ResponseInputItemUnionParam](json.RawMessage(payload)), nil
}

func (s *Service) runAgentComputerCall(call responses.ResponseComputerToolCall, state *agentComputerState) (responses.ResponseInputItemUnionParam, AgentToolEvent, string, error) {
	event := AgentToolEvent{
		CallID:    call.CallID,
		Name:      agentComputerToolName,
		Arguments: normalizeAgentToolArguments(call.RawJSON()),
	}

	if checks := computerSafetyCheckSummary(call.PendingSafetyChecks); checks != "" {
		event.Error = fmt.Sprintf("pending computer safety checks require manual handling: %s", checks)
		event.Output = marshalToolPayload(map[string]any{
			"code":                  agentComputerSafetyAckCode,
			"error":                 event.Error,
			"pending_safety_checks": call.PendingSafetyChecks,
		})
		return responses.ResponseInputItemUnionParam{}, event, event.Output, errors.New(event.Error)
	}

	actions := agentComputerActions(call)
	if len(actions) == 0 {
		event.Error = "computer call did not include any actions"
		event.Output = marshalToolPayload(map[string]any{"error": event.Error})
		return responses.ResponseInputItemUnionParam{}, event, event.Output, errors.New(event.Error)
	}

	summaries := make([]string, 0, len(actions))
	for _, action := range actions {
		result, err := s.runAgentComputerAction(action, state)
		if err != nil {
			event.Error = err.Error()
			event.Output = marshalToolPayload(map[string]any{"error": err.Error()})
			return responses.ResponseInputItemUnionParam{}, event, event.Output, err
		}
		if summary := strings.TrimSpace(result.Summary); summary != "" {
			summaries = append(summaries, summary)
		}
		if result.RequestedScreenshot {
			break
		}
	}

	capture, err := s.captureComputerScreen()
	if err != nil {
		event.Error = err.Error()
		event.Output = marshalToolPayload(map[string]any{"error": err.Error()})
		return responses.ResponseInputItemUnionParam{}, event, event.Output, err
	}
	capture = normalizeCaptureResultMetadata(capture)
	state.LastCapture = &capture

	output, err := agentComputerCallOutput(call.CallID, capture)
	if err != nil {
		event.Error = err.Error()
		event.Output = marshalToolPayload(map[string]any{"error": err.Error()})
		return responses.ResponseInputItemUnionParam{}, event, event.Output, err
	}

	event.Output = marshalToolPayload(map[string]any{
		"actions": summaries,
		"capture": capture,
	})
	return output, event, event.Output, nil
}

func computerSafetyCheckSummary(checks []responses.ResponseComputerToolCallPendingSafetyCheck) string {
	if len(checks) == 0 {
		return ""
	}

	parts := make([]string, 0, len(checks))
	for _, check := range checks {
		switch {
		case strings.TrimSpace(check.Message) != "":
			parts = append(parts, strings.TrimSpace(check.Message))
		case strings.TrimSpace(check.Code) != "":
			parts = append(parts, strings.TrimSpace(check.Code))
		case strings.TrimSpace(check.ID) != "":
			parts = append(parts, strings.TrimSpace(check.ID))
		}
	}
	return strings.Join(parts, "; ")
}

func (s *Service) runAgentComputerAction(action responses.ComputerActionUnion, state *agentComputerState) (agentComputerActionResult, error) {
	switch action.Type {
	case "click":
		point, err := agentComputerPoint(state, action.X, action.Y)
		if err != nil {
			return agentComputerActionResult{}, err
		}
		button := agentComputerMouseButton(action.Button)
		if err := s.runComputerActionWithHeldKeys(action.Keys, func() error {
			if _, err := s.SetMousePosition(MouseMoveRequest{X: point.X, Y: point.Y}); err != nil {
				return err
			}
			_, err := s.ClickMouse(MouseButtonRequest{Button: button})
			return err
		}); err != nil {
			return agentComputerActionResult{}, err
		}
		return agentComputerActionResult{Summary: fmt.Sprintf("Clicked %s at (%d, %d).", button, point.X, point.Y)}, nil
	case "double_click":
		point, err := agentComputerPoint(state, action.X, action.Y)
		if err != nil {
			return agentComputerActionResult{}, err
		}
		if err := s.runComputerActionWithHeldKeys(action.Keys, func() error {
			if _, err := s.SetMousePosition(MouseMoveRequest{X: point.X, Y: point.Y}); err != nil {
				return err
			}
			_, err := s.DoubleClickMouse(MouseButtonRequest{Button: "left"})
			return err
		}); err != nil {
			return agentComputerActionResult{}, err
		}
		return agentComputerActionResult{Summary: fmt.Sprintf("Double-clicked at (%d, %d).", point.X, point.Y)}, nil
	case "move":
		point, err := agentComputerPoint(state, action.X, action.Y)
		if err != nil {
			return agentComputerActionResult{}, err
		}
		if err := s.runComputerActionWithHeldKeys(action.Keys, func() error {
			_, err := s.SetMousePosition(MouseMoveRequest{X: point.X, Y: point.Y})
			return err
		}); err != nil {
			return agentComputerActionResult{}, err
		}
		return agentComputerActionResult{Summary: fmt.Sprintf("Moved pointer to (%d, %d).", point.X, point.Y)}, nil
	case "scroll":
		point, err := agentComputerPoint(state, action.X, action.Y)
		if err != nil {
			return agentComputerActionResult{}, err
		}
		if err := s.runComputerActionWithHeldKeys(action.Keys, func() error {
			if _, err := s.SetMousePosition(MouseMoveRequest{X: point.X, Y: point.Y}); err != nil {
				return err
			}
			return s.runAgentComputerScroll(action.ScrollX, action.ScrollY)
		}); err != nil {
			return agentComputerActionResult{}, err
		}
		return agentComputerActionResult{
			Summary: fmt.Sprintf("Scrolled at (%d, %d) by (%d, %d).", point.X, point.Y, action.ScrollX, action.ScrollY),
		}, nil
	case "type":
		if _, err := s.TypeText(KeyboardTextRequest{Text: action.Text}); err != nil {
			return agentComputerActionResult{}, err
		}
		return agentComputerActionResult{Summary: fmt.Sprintf("Typed %d characters.", len(action.Text))}, nil
	case "keypress":
		if _, err := s.TapKeys(KeyboardKeysRequest{Keys: normalizeComputerKeys(action.Keys)}); err != nil {
			return agentComputerActionResult{}, err
		}
		return agentComputerActionResult{Summary: fmt.Sprintf("Pressed keys %s.", strings.Join(normalizeComputerKeys(action.Keys), "+"))}, nil
	case "drag":
		points, err := agentComputerPath(state, action.Path)
		if err != nil {
			return agentComputerActionResult{}, err
		}
		if err := s.runComputerActionWithHeldKeys(action.Keys, func() error {
			return s.runAgentComputerDrag(points)
		}); err != nil {
			return agentComputerActionResult{}, err
		}
		start := points[0]
		end := points[len(points)-1]
		return agentComputerActionResult{
			Summary: fmt.Sprintf("Dragged from (%d, %d) to (%d, %d).", start.X, start.Y, end.X, end.Y),
		}, nil
	case "wait":
		time.Sleep(agentComputerWait)
		return agentComputerActionResult{Summary: fmt.Sprintf("Waited %s.", agentComputerWait)}, nil
	case "screenshot":
		return agentComputerActionResult{
			Summary:             "Captured a fresh desktop screenshot for the next model step.",
			RequestedScreenshot: true,
		}, nil
	default:
		return agentComputerActionResult{}, fmt.Errorf("unsupported computer action %q", action.Type)
	}
}

func (s *Service) runComputerActionWithHeldKeys(keys []string, run func() error) error {
	keys = normalizeComputerKeys(keys)
	if len(keys) == 0 {
		return run()
	}

	if _, err := s.PressKeys(KeyboardKeysRequest{Keys: keys}); err != nil {
		return err
	}

	runErr := run()
	_, releaseErr := s.ReleaseKeys(KeyboardKeysRequest{Keys: keys})
	if runErr != nil {
		if releaseErr != nil {
			return fmt.Errorf("%w (plus failed to release keys: %v)", runErr, releaseErr)
		}
		return runErr
	}
	return releaseErr
}

func normalizeComputerKeys(keys []string) []string {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func agentComputerMouseButton(button string) string {
	switch strings.ToLower(strings.TrimSpace(button)) {
	case "", "left":
		return "left"
	case "wheel":
		return "middle"
	default:
		return strings.ToLower(strings.TrimSpace(button))
	}
}

func agentComputerPoint(state *agentComputerState, x int64, y int64) (Point, error) {
	if state == nil || state.LastCapture == nil {
		return Point{}, fmt.Errorf("computer action coordinates require a prior screenshot")
	}
	translated, err := translateDeliveredImagePointToScreen(*state.LastCapture, Point{X: int(x), Y: int(y)})
	if err != nil {
		return Point{}, err
	}
	return translated.AbsoluteScreenPoint, nil
}

func agentComputerPath(state *agentComputerState, path []responses.ComputerActionDragPath) ([]Point, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("computer drag action path is empty")
	}

	points := make([]Point, 0, len(path))
	for _, item := range path {
		point, err := agentComputerPoint(state, item.X, item.Y)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

func (s *Service) runAgentComputerScroll(scrollX int64, scrollY int64) error {
	if scrollY < 0 {
		if _, err := s.ScrollMouse(MouseScrollRequest{Direction: "up", Amount: int(-scrollY)}); err != nil {
			return err
		}
	}
	if scrollY > 0 {
		if _, err := s.ScrollMouse(MouseScrollRequest{Direction: "down", Amount: int(scrollY)}); err != nil {
			return err
		}
	}
	if scrollX < 0 {
		if _, err := s.ScrollMouse(MouseScrollRequest{Direction: "left", Amount: int(-scrollX)}); err != nil {
			return err
		}
	}
	if scrollX > 0 {
		if _, err := s.ScrollMouse(MouseScrollRequest{Direction: "right", Amount: int(scrollX)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runAgentComputerDrag(points []Point) error {
	if len(points) == 0 {
		return fmt.Errorf("computer drag action path is empty")
	}
	if _, err := s.SetMousePosition(MouseMoveRequest{X: points[0].X, Y: points[0].Y}); err != nil {
		return err
	}
	if _, err := s.MouseDown(MouseButtonRequest{Button: "left"}); err != nil {
		return err
	}

	for _, point := range points[1:] {
		if _, err := s.MoveMouseLine(MouseLineRequest{
			X:     point.X,
			Y:     point.Y,
			Speed: agentMouseSpeed,
		}); err != nil {
			_, _ = s.MouseUp(MouseButtonRequest{Button: "left"})
			return err
		}
	}

	_, err := s.MouseUp(MouseButtonRequest{Button: "left"})
	return err
}
