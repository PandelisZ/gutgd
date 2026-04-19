package backend

import (
	"math"
	"strings"
	"time"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/native/libnutcore"
)

var showAgentCursorOverlay = libnutcore.ShowAgentCursor

func decorateAgentPointerTools(service *Service, tools []agentTool, coordinateState *agentCoordinateState, pointerState *agentPointerState) []agentTool {
	if service == nil {
		return tools
	}
	if pointerState == nil {
		pointerState = newAgentPointerState()
	}

	overlay := &agentPointerOverlay{
		service:         service,
		coordinateState: coordinateState,
		state:           pointerState,
	}
	for index := range tools {
		tools[index] = overlay.decorate(tools[index])
	}
	return tools
}

type agentPointerOverlay struct {
	service         *Service
	coordinateState *agentCoordinateState
	state           *agentPointerState
}

func (o *agentPointerOverlay) decorate(tool agentTool) agentTool {
	switch tool.Name {
	case "search_ax_elements":
		return o.decorateSearchAXElements(tool)
	case "set_mouse_position":
		return o.decorateSetMousePosition(tool)
	case "move_mouse_line":
		return o.decorateMoveMouseLine(tool)
	case "drag_mouse":
		return o.decorateDragMouse(tool)
	case "click_mouse":
		return o.decorateClickMouse(tool, libnutcore.AgentCursorEventClick)
	case "double_click_mouse":
		return o.decorateClickMouse(tool, libnutcore.AgentCursorEventDoubleClick)
	case "mouse_down":
		return o.decorateMouseToggle(tool, libnutcore.AgentCursorEventMouseDown, true)
	case "mouse_up":
		return o.decorateMouseToggle(tool, libnutcore.AgentCursorEventMouseUp, false)
	case "scroll_mouse":
		return o.decorateScrollMouse(tool)
	case "perform_element_action_at_point":
		return o.decoratePerformElementActionAtPoint(tool)
	case "focus_element_at_point":
		return o.decorateFocusElementAtPoint(tool)
	case "act_on_window_accessibility_element":
		return o.decorateActOnWindowAccessibilityElement(tool)
	case "focus_ax_element":
		return o.decorateFocusAXElement(tool)
	case "perform_ax_element_action":
		return o.decoratePerformAXElementAction(tool)
	default:
		return tool
	}
}

func (o *agentPointerOverlay) decorateSearchAXElements(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		result, err := original(raw)
		if err == nil {
			o.cacheAXSearchPoints(result)
		}
		return result, err
	}
	return tool
}

func (o *agentPointerOverlay) decorateSetMousePosition(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		target, ok := o.decodePointTarget(raw)
		if ok {
			o.emitMove(target, false)
		}
		result, err := original(raw)
		if err == nil && ok {
			o.state.rememberLast(target)
			o.state.Pressed = false
		}
		return result, err
	}
	return tool
}

func (o *agentPointerOverlay) decorateMoveMouseLine(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		target, ok := o.decodeLineTarget(raw)
		if ok {
			o.emitMove(target, false)
		}
		result, err := original(raw)
		if err == nil && ok {
			o.state.rememberLast(target)
			o.state.Pressed = false
		}
		return result, err
	}
	return tool
}

func (o *agentPointerOverlay) decorateDragMouse(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		from, to, ok := o.decodeDragTargets(raw)
		if ok {
			duration := agentCursorMotionDuration(from, to, true)
			o.emit(libnutcore.AgentCursorEvent{
				Kind:     libnutcore.AgentCursorEventDragStart,
				Position: common.Point{X: from.X, Y: from.Y},
				Target:   &common.Point{X: to.X, Y: to.Y},
				Button:   common.MouseButtonLeft,
				Pressed:  true,
				Duration: duration,
			})
		}
		result, err := original(raw)
		if ok {
			o.emit(libnutcore.AgentCursorEvent{
				Kind:     libnutcore.AgentCursorEventDragEnd,
				Position: common.Point{X: to.X, Y: to.Y},
				Button:   common.MouseButtonLeft,
			})
		}
		if err == nil && ok {
			o.state.rememberLast(to)
			o.state.Pressed = false
		}
		return result, err
	}
	return tool
}

func (o *agentPointerOverlay) decorateClickMouse(tool agentTool, kind libnutcore.AgentCursorEventKind) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		result, err := original(raw)
		if err != nil {
			return result, err
		}
		point, ok := o.resolveScreenPoint(nil, nil)
		if ok {
			button := o.decodeMouseButton(raw)
			o.emit(libnutcore.AgentCursorEvent{
				Kind:     kind,
				Position: common.Point{X: point.X, Y: point.Y},
				Button:   button,
			})
			o.state.rememberLast(point)
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decorateMouseToggle(tool agentTool, kind libnutcore.AgentCursorEventKind, pressed bool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		result, err := original(raw)
		if err != nil {
			return result, err
		}
		point, ok := o.resolveScreenPoint(nil, nil)
		if ok {
			button := o.decodeMouseButton(raw)
			o.emit(libnutcore.AgentCursorEvent{
				Kind:     kind,
				Position: common.Point{X: point.X, Y: point.Y},
				Button:   button,
				Pressed:  pressed,
			})
			o.state.rememberLast(point)
			o.state.Pressed = pressed
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decorateScrollMouse(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		result, err := original(raw)
		if err != nil {
			return result, err
		}
		point, ok := o.resolveScreenPoint(nil, nil)
		if ok {
			o.emit(libnutcore.AgentCursorEvent{
				Kind:      libnutcore.AgentCursorEventScroll,
				Position:  common.Point{X: point.X, Y: point.Y},
				Direction: o.decodeScrollDirection(raw),
			})
			o.state.rememberLast(point)
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decoratePerformElementActionAtPoint(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		target, ok := o.decodePointTarget(raw)
		result, err := original(raw)
		if err != nil {
			return result, err
		}
		if ok {
			o.emit(libnutcore.AgentCursorEvent{
				Kind:     libnutcore.AgentCursorEventClick,
				Position: common.Point{X: target.X, Y: target.Y},
			})
			o.state.rememberLast(target)
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decorateFocusElementAtPoint(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		target, ok := o.decodePointTarget(raw)
		result, err := original(raw)
		if err != nil {
			return result, err
		}
		if ok {
			o.emitMove(target, false)
			o.state.rememberLast(target)
			o.state.Pressed = false
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decorateActOnWindowAccessibilityElement(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		payload := WindowAccessibilityElementActionRequest{}
		_ = decodeToolArgs(raw, &payload)
		point, ok := o.lookupSnapshotElementPoint(payload.SnapshotID, payload.ElementID)

		result, err := original(raw)
		if err != nil {
			return result, err
		}
		if ok {
			action := strings.ToLower(strings.TrimSpace(payload.Action))
			switch action {
			case "focus":
				o.emitMove(point, false)
				o.state.Pressed = false
			case "double_click":
				o.emit(libnutcore.AgentCursorEvent{
					Kind:     libnutcore.AgentCursorEventDoubleClick,
					Position: common.Point{X: point.X, Y: point.Y},
				})
			default:
				o.emit(libnutcore.AgentCursorEvent{
					Kind:     libnutcore.AgentCursorEventClick,
					Position: common.Point{X: point.X, Y: point.Y},
				})
			}
			o.state.rememberLast(point)
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decorateFocusAXElement(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		payload := FocusAXElementRequest{}
		_ = decodeToolArgs(raw, &payload)
		point, ok := o.resolveScreenPoint(nil, &payload.Ref)

		result, err := original(raw)
		if err != nil {
			return result, err
		}
		if ok {
			o.emitMove(point, false)
			o.state.rememberLast(point)
			o.state.Pressed = false
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) decoratePerformAXElementAction(tool agentTool) agentTool {
	original := tool.Run
	tool.Run = func(raw string) (any, error) {
		payload := PerformAXElementActionOnRefRequest{}
		_ = decodeToolArgs(raw, &payload)
		point, ok := o.resolveScreenPoint(nil, &payload.Ref)

		result, err := original(raw)
		if err != nil {
			return result, err
		}
		if ok {
			o.emit(libnutcore.AgentCursorEvent{
				Kind:     libnutcore.AgentCursorEventClick,
				Position: common.Point{X: point.X, Y: point.Y},
			})
			o.state.rememberLast(point)
			o.state.Pressed = false
		}
		return result, nil
	}
	return tool
}

func (o *agentPointerOverlay) emitMove(target Point, dragging bool) {
	start, ok := o.resolveCurrentOrLastPoint()
	if !ok {
		start = target
	}
	targetPoint := common.Point{X: target.X, Y: target.Y}
	o.emit(libnutcore.AgentCursorEvent{
		Kind:     libnutcore.AgentCursorEventMove,
		Position: common.Point{X: start.X, Y: start.Y},
		Target:   &targetPoint,
		Duration: agentCursorMotionDuration(start, target, dragging),
	})
}

func (o *agentPointerOverlay) emit(event libnutcore.AgentCursorEvent) {
	if strings.TrimSpace(string(event.Kind)) == "" {
		return
	}
	_ = showAgentCursorOverlay(event)
}

func (o *agentPointerOverlay) cacheAXSearchPoints(result any) {
	if o.state == nil {
		return
	}

	switch typed := result.(type) {
	case SearchAXElementsResult:
		for _, match := range typed.Matches {
			if !match.ActionPointKnown {
				continue
			}
			o.state.rememberAXPointForResult(match.Ref, match.ActionPoint)
		}
	case *SearchAXElementsResult:
		if typed == nil {
			return
		}
		o.cacheAXSearchPoints(*typed)
	}
}

func (o *agentPointerOverlay) decodePointTarget(raw string) (Point, bool) {
	payload := PointRequest{}
	if err := decodeToolArgs(raw, &payload); err != nil {
		return Point{}, false
	}
	return o.translatePoint(Point{X: payload.X, Y: payload.Y}), true
}

func (o *agentPointerOverlay) decodeLineTarget(raw string) (Point, bool) {
	payload := MouseLineRequest{}
	if err := decodeToolArgs(raw, &payload); err != nil {
		return Point{}, false
	}
	return o.translatePoint(Point{X: payload.X, Y: payload.Y}), true
}

func (o *agentPointerOverlay) decodeDragTargets(raw string) (Point, Point, bool) {
	payload := MouseDragRequest{}
	if err := decodeToolArgs(raw, &payload); err != nil {
		return Point{}, Point{}, false
	}
	from := o.translatePoint(Point{X: payload.FromX, Y: payload.FromY})
	to := o.translatePoint(Point{X: payload.ToX, Y: payload.ToY})
	return from, to, true
}

func (o *agentPointerOverlay) decodeMouseButton(raw string) common.MouseButton {
	payload := MouseButtonRequest{}
	if err := decodeToolArgs(raw, &payload); err != nil {
		return ""
	}
	return common.MouseButton(strings.ToLower(strings.TrimSpace(payload.Button)))
}

func (o *agentPointerOverlay) decodeScrollDirection(raw string) libnutcore.AgentCursorDirection {
	payload := MouseScrollRequest{}
	if err := decodeToolArgs(raw, &payload); err != nil {
		return ""
	}
	return libnutcore.AgentCursorDirection(strings.ToLower(strings.TrimSpace(payload.Direction)))
}

func (o *agentPointerOverlay) translatePoint(point Point) Point {
	state := agentCoordinateState{Mode: "screen"}
	if o.coordinateState != nil {
		state = *o.coordinateState
	}
	return translatePointToScreenSpace(point, state)
}

func (o *agentPointerOverlay) resolveScreenPoint(explicit *Point, ref *AXElementRefResult) (Point, bool) {
	if explicit != nil {
		return *explicit, true
	}
	if ref != nil && o.state != nil {
		if point, ok := o.state.lookupAXPointForResult(*ref); ok {
			return point, true
		}
	}
	return o.resolveCurrentOrLastPoint()
}

func (o *agentPointerOverlay) resolveCurrentOrLastPoint() (Point, bool) {
	if o.service != nil {
		if point, err := o.service.GetMousePosition(); err == nil {
			return point, true
		}
	}
	if o.state != nil && o.state.LastScreenPoint != nil {
		return *o.state.LastScreenPoint, true
	}
	return Point{}, false
}

func (o *agentPointerOverlay) lookupSnapshotElementPoint(snapshotID string, elementID string) (Point, bool) {
	if o.service == nil {
		return Point{}, false
	}

	o.service.mu.Lock()
	defer o.service.mu.Unlock()

	snapshot, ok := o.service.accessibilitySnapshots[strings.TrimSpace(snapshotID)]
	if !ok {
		return Point{}, false
	}
	element, ok := snapshot.Elements[strings.TrimSpace(elementID)]
	if !ok || element.ScreenRegion == nil {
		return Point{}, false
	}
	return centerPoint(*element.ScreenRegion), true
}

func agentCursorMotionDuration(from Point, to Point, dragging bool) time.Duration {
	distance := math.Hypot(float64(to.X-from.X), float64(to.Y-from.Y))
	if dragging {
		return clampCursorDuration(220.0+distance*0.18, 220, 520)
	}
	return clampCursorDuration(160.0+distance*0.16, 160, 420)
}

func clampCursorDuration(value float64, minMS int, maxMS int) time.Duration {
	if value < float64(minMS) {
		value = float64(minMS)
	}
	if value > float64(maxMS) {
		value = float64(maxMS)
	}
	return time.Duration(value) * time.Millisecond
}
