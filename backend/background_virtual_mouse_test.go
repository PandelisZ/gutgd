package backend

import (
	"testing"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/shared"
)

func TestPerformBackgroundWindowActionDoesNotTouchPrimaryMouseProvider(t *testing.T) {
	service, _, snapshot, mouse := newBackgroundVirtualActionFixture(t)
	mouse.position = shared.Point{X: 44, Y: 61}

	result, err := service.PerformBackgroundWindowAction(BackgroundMouseActionRequest{
		SnapshotID: snapshot.SnapshotID,
		Action:     "click",
		Point:      &PointRequest{X: 716, Y: 592},
	})
	if err != nil {
		t.Fatalf("PerformBackgroundWindowAction returned error: %v", err)
	}
	if result.ScreenPoint != (Point{X: 1016, Y: 712}) {
		t.Fatalf("unexpected strict background screen point: %+v", result)
	}
	if mouse.position != (shared.Point{X: 44, Y: 61}) {
		t.Fatalf("expected primary mouse position to remain unchanged, got %+v", mouse.position)
	}
	if len(mouse.setPositions) != 0 {
		t.Fatalf("expected no primary mouse restore or move calls, got %+v", mouse.setPositions)
	}
	if len(mouse.clicks) != 0 || len(mouse.doubleClicks) != 0 || len(mouse.pressedButtons) != 0 || len(mouse.releasedButtons) != 0 {
		t.Fatalf("expected strict background action to avoid real mouse input, got clicks=%+v double=%+v down=%+v up=%+v", mouse.clicks, mouse.doubleClicks, mouse.pressedButtons, mouse.releasedButtons)
	}
}

func TestPerformBackgroundWindowActionRestoresPrimaryMousePositionWhenUnderlyingLayerMovesIt(t *testing.T) {
	service, accessibility, snapshot, mouse := newBackgroundVirtualActionFixture(t)
	start := shared.Point{X: 44, Y: 61}
	mouse.position = start
	accessibility.performAXElementActionHook = func(common.AXElementRef, common.AXAction) {
		mouse.position = shared.Point{X: 1016, Y: 712}
	}

	if _, err := service.PerformBackgroundWindowAction(BackgroundMouseActionRequest{
		SnapshotID: snapshot.SnapshotID,
		Action:     "click",
		Point:      &PointRequest{X: 716, Y: 592},
	}); err != nil {
		t.Fatalf("PerformBackgroundWindowAction returned error: %v", err)
	}

	if mouse.position != start {
		t.Fatalf("expected primary mouse position to be restored, got %+v want %+v", mouse.position, start)
	}
	if len(mouse.setPositions) != 1 || mouse.setPositions[0] != start {
		t.Fatalf("expected one restore call back to the starting pointer position, got %+v", mouse.setPositions)
	}
	if len(mouse.clicks) != 0 || len(mouse.doubleClicks) != 0 {
		t.Fatalf("expected strict background action to avoid real click calls, got clicks=%+v double=%+v", mouse.clicks, mouse.doubleClicks)
	}
}

func newBackgroundVirtualActionFixture(t *testing.T) (*Service, *fakeBackendAccessibilityProvider, WindowAccessibilitySnapshotResult, *fakeBackendMouseProvider) {
	t.Helper()

	accessibility := &fakeBackendAccessibilityProvider{
		searchAXElementsMatches: []common.AXElementMatch{
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{}},
				Metadata: common.UIElementMetadata{
					Role:       "AXWindow",
					Title:      "Slack",
					Enabled:    true,
					Frame:      common.Rect{X: 300, Y: 120, Width: 800, Height: 600},
					FrameKnown: true,
				},
			},
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{0}},
				Metadata: common.UIElementMetadata{
					Role:       "AXButton",
					Title:      "Send",
					Enabled:    true,
					Frame:      common.Rect{X: 1000, Y: 700, Width: 32, Height: 24},
					FrameKnown: true,
					Actions:    []string{string(common.AXPress)},
				},
				Depth:            1,
				ActionPoint:      common.Point{X: 1016, Y: 712},
				ActionPointKnown: true,
			},
		},
	}

	service := newTestServiceWithWindowsAndAccessibility(accessibility, WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})

	snapshot, err := service.GetWindowAccessibilitySnapshot(WindowAccessibilitySnapshotRequest{Handle: 7})
	if err != nil {
		t.Fatalf("GetWindowAccessibilitySnapshot returned error: %v", err)
	}

	mouseProvider, err := service.nut.Registry.Mouse()
	if err != nil {
		t.Fatalf("expected mouse provider: %v", err)
	}
	mouse, ok := mouseProvider.(*fakeBackendMouseProvider)
	if !ok {
		t.Fatalf("expected fake backend mouse provider, got %T", mouseProvider)
	}

	return service, accessibility, snapshot, mouse
}
