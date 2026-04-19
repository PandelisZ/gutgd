package backend

import (
	"errors"
	"testing"
	"time"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/native/libnutcore"
	"github.com/PandelisZ/gut/shared"
)

func TestPreviewAgentCursorEmitsOverlaySequence(t *testing.T) {
	service := newTestServiceWithWindows()
	setFakeMousePosition(t, service, shared.Point{X: 120, Y: 160})

	var events []recordedAgentCursorEvent
	var hidden bool

	previousShow := showPreviewAgentCursorOverlay
	previousHide := hidePreviewAgentCursorOverlay
	previousRun := runPreviewAgentCursorAsync
	previousSleep := sleepPreviewAgentCursor
	t.Cleanup(func() {
		showPreviewAgentCursorOverlay = previousShow
		hidePreviewAgentCursorOverlay = previousHide
		runPreviewAgentCursorAsync = previousRun
		sleepPreviewAgentCursor = previousSleep
	})

	showPreviewAgentCursorOverlay = func(event libnutcore.AgentCursorEvent) error {
		recorded := recordedAgentCursorEvent{
			Kind:      event.Kind,
			Position:  event.Position,
			Button:    event.Button,
			Direction: event.Direction,
			Pressed:   event.Pressed,
			Duration:  event.Duration,
		}
		if event.Target != nil {
			target := *event.Target
			recorded.Target = &target
		}
		events = append(events, recorded)
		return nil
	}
	hidePreviewAgentCursorOverlay = func() error {
		hidden = true
		return nil
	}
	runPreviewAgentCursorAsync = func(fn func()) { fn() }
	sleepPreviewAgentCursor = func(time.Duration) {}

	result, err := service.PreviewAgentCursor()
	if err != nil {
		t.Fatalf("PreviewAgentCursor returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK result, got %+v", result)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 overlay events, got %+v", events)
	}
	if events[0].Kind != libnutcore.AgentCursorEventMove || events[0].Target == nil {
		t.Fatalf("expected initial move with target, got %+v", events[0])
	}
	if events[1].Kind != libnutcore.AgentCursorEventClick {
		t.Fatalf("expected click event, got %+v", events[1])
	}
	if events[2].Kind != libnutcore.AgentCursorEventScroll || events[2].Direction != libnutcore.AgentCursorDirectionDown {
		t.Fatalf("expected scroll event, got %+v", events[2])
	}
	if !hidden {
		t.Fatal("expected preview to hide the cursor at the end")
	}
}

func TestPreviewAgentCursorUsesFallbackWhenMousePositionUnavailable(t *testing.T) {
	service := newTestServiceWithWindows()
	mouseProvider, err := service.nut.Registry.Mouse()
	if err != nil {
		t.Fatalf("expected mouse provider: %v", err)
	}
	fakeMouse, ok := mouseProvider.(*fakeBackendMouseProvider)
	if !ok {
		t.Fatalf("expected fake backend mouse provider, got %T", mouseProvider)
	}
	fakeMouse.positionErr = errors.New("mouse unavailable")

	previousShow := showPreviewAgentCursorOverlay
	previousHide := hidePreviewAgentCursorOverlay
	previousRun := runPreviewAgentCursorAsync
	previousSleep := sleepPreviewAgentCursor
	t.Cleanup(func() {
		showPreviewAgentCursorOverlay = previousShow
		hidePreviewAgentCursorOverlay = previousHide
		runPreviewAgentCursorAsync = previousRun
		sleepPreviewAgentCursor = previousSleep
	})

	var firstEvent libnutcore.AgentCursorEvent
	showPreviewAgentCursorOverlay = func(event libnutcore.AgentCursorEvent) error {
		if firstEvent.Kind == "" {
			firstEvent = event
		}
		return nil
	}
	hidePreviewAgentCursorOverlay = func() error { return nil }
	runPreviewAgentCursorAsync = func(fn func()) { fn() }
	sleepPreviewAgentCursor = func(time.Duration) {}

	if _, err := service.PreviewAgentCursor(); err != nil {
		t.Fatalf("PreviewAgentCursor returned error: %v", err)
	}
	if firstEvent.Position != (common.Point{X: 720, Y: 480}) {
		t.Fatalf("expected fallback start point, got %+v", firstEvent.Position)
	}
}
