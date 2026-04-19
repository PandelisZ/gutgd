package backend

import (
	"testing"
	"time"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/native/libnutcore"
	"github.com/PandelisZ/gut/shared"
)

type recordedAgentCursorEvent struct {
	Kind      libnutcore.AgentCursorEventKind
	Position  common.Point
	Target    *common.Point
	Button    common.MouseButton
	Direction libnutcore.AgentCursorDirection
	Pressed   bool
	Duration  time.Duration
}

func TestDecorateSetMousePositionUsesTranslatedPointOverCachedAndCurrent(t *testing.T) {
	service := newTestServiceWithWindows(
		WindowSummary{Handle: 9, Title: "Slack", Region: Region{Left: 100, Top: 200, Width: 800, Height: 600}},
	)
	setFakeMousePosition(t, service, sharedPoint(3, 4))

	pointerState := newAgentPointerState()
	pointerState.rememberAXPointForResult(AXElementRefResult{Scope: "focused_window", Path: []int{0}}, Point{X: 900, Y: 901})

	coordinateState := &agentCoordinateState{
		Mode: "window",
		Window: &WindowSummary{
			Handle: 9,
			Title:  "Slack",
			Region: Region{Left: 100, Top: 200, Width: 800, Height: 600},
		},
	}

	var events []recordedAgentCursorEvent
	restore := interceptAgentCursorEvents(&events)
	defer restore()

	tools := decorateAgentPointerTools(service, []agentTool{{
		Name: "set_mouse_position",
		Run: func(string) (any, error) {
			return ActionResult{OK: true}, nil
		},
	}}, coordinateState, pointerState)

	if _, err := tools[0].Run(`{"x":5,"y":7}`); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 cursor event, got %d", len(events))
	}
	if events[0].Kind != libnutcore.AgentCursorEventMove {
		t.Fatalf("expected move event, got %+v", events[0])
	}
	if events[0].Target == nil || *events[0].Target != (common.Point{X: 105, Y: 207}) {
		t.Fatalf("expected translated target (105, 207), got %+v", events[0].Target)
	}
}

func TestSearchAXElementsCachesPointAndEmitsNoCursorEvent(t *testing.T) {
	service := newTestServiceWithWindows()
	pointerState := newAgentPointerState()

	var events []recordedAgentCursorEvent
	restore := interceptAgentCursorEvents(&events)
	defer restore()

	matchRef := AXElementRefResult{Scope: "focused_window", OwnerPID: 42, WindowHandle: 0, Path: []int{1, 2}}
	searchResult := SearchAXElementsResult{
		Matches: []AXElementMatchResult{{
			Ref:              matchRef,
			ActionPoint:      Point{X: 640, Y: 360},
			ActionPointKnown: true,
		}},
	}

	tools := decorateAgentPointerTools(service, []agentTool{{
		Name: "search_ax_elements",
		Run: func(string) (any, error) {
			return searchResult, nil
		},
	}}, newAgentCoordinateState(), pointerState)

	if _, err := tools[0].Run(`{"scope":"focused_window","limit":1,"max_depth":0}`); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no cursor events for search-only tool, got %+v", events)
	}
	if point, ok := pointerState.lookupAXPointForResult(matchRef); !ok || point != (Point{X: 640, Y: 360}) {
		t.Fatalf("expected cached AX point, got point=%+v ok=%v", point, ok)
	}
}

func TestDecorateFocusAXElementUsesCachedPointBeforeCurrentMouse(t *testing.T) {
	service := newTestServiceWithWindows()
	setFakeMousePosition(t, service, sharedPoint(12, 18))

	pointerState := newAgentPointerState()
	ref := AXElementRefResult{Scope: "focused_window", OwnerPID: 7, WindowHandle: 0, Path: []int{4}}
	pointerState.rememberAXPointForResult(ref, Point{X: 700, Y: 480})

	var events []recordedAgentCursorEvent
	restore := interceptAgentCursorEvents(&events)
	defer restore()

	tools := decorateAgentPointerTools(service, []agentTool{{
		Name: "focus_ax_element",
		Run: func(string) (any, error) {
			return FocusAXElementResult{OK: true, Ref: ref}, nil
		},
	}}, newAgentCoordinateState(), pointerState)

	if _, err := tools[0].Run(`{"ref":{"scope":"focused_window","owner_pid":7,"window_handle":0,"path":[4]}}`); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 cursor event, got %d", len(events))
	}
	if events[0].Target == nil || *events[0].Target != (common.Point{X: 700, Y: 480}) {
		t.Fatalf("expected cached AX target, got %+v", events[0].Target)
	}
}

func TestClickMouseFallsBackToCurrentMousePosition(t *testing.T) {
	service := newTestServiceWithWindows()
	setFakeMousePosition(t, service, sharedPoint(91, 44))

	var events []recordedAgentCursorEvent
	restore := interceptAgentCursorEvents(&events)
	defer restore()

	tools := decorateAgentPointerTools(service, []agentTool{{
		Name: "click_mouse",
		Run: func(string) (any, error) {
			return ActionResult{OK: true}, nil
		},
	}}, newAgentCoordinateState(), newAgentPointerState())

	if _, err := tools[0].Run(`{"button":"left"}`); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 cursor event, got %d", len(events))
	}
	if events[0].Position != (common.Point{X: 91, Y: 44}) {
		t.Fatalf("expected current mouse fallback position, got %+v", events[0])
	}
}

func interceptAgentCursorEvents(events *[]recordedAgentCursorEvent) func() {
	previous := showAgentCursorOverlay
	showAgentCursorOverlay = func(event libnutcore.AgentCursorEvent) error {
		recorded := recordedAgentCursorEvent{
			Kind:      event.Kind,
			Position:  event.Position,
			Button:    event.Button,
			Direction: event.Direction,
			Pressed:   event.Pressed,
			Duration:  event.Duration,
		}
		if event.Target != nil {
			point := *event.Target
			recorded.Target = &point
		}
		*events = append(*events, recorded)
		return nil
	}
	return func() {
		showAgentCursorOverlay = previous
	}
}

func setFakeMousePosition(t *testing.T, service *Service, point shared.Point) {
	t.Helper()

	mouseProvider, err := service.nut.Registry.Mouse()
	if err != nil {
		t.Fatalf("expected mouse provider: %v", err)
	}
	fakeMouse, ok := mouseProvider.(*fakeBackendMouseProvider)
	if !ok {
		t.Fatalf("expected fake backend mouse provider, got %T", mouseProvider)
	}
	fakeMouse.position = point
}

func sharedPoint(x int, y int) shared.Point {
	return shared.Point{X: x, Y: y}
}
