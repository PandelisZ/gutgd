package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/PandelisZ/gut/backgroundmouse"
	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/native/libnutcore"
	"github.com/PandelisZ/gut/shared"
)

func (s *Service) performStrictBackgroundWindowAction(
	ctx context.Context,
	snapshot backgroundmouse.WindowSnapshot,
	req backgroundmouse.SnapshotActionRequest,
) (backgroundmouse.ActionResult, error) {
	mouseProvider, err := s.nut.Registry.Mouse()
	if err != nil {
		return s.nut.BackgroundMouse.PerformInSnapshot(ctx, snapshot, req)
	}

	start, err := mouseProvider.CurrentMousePosition(ctx)
	if err != nil {
		return s.nut.BackgroundMouse.PerformInSnapshot(ctx, snapshot, req)
	}

	result, actionErr := s.nut.BackgroundMouse.PerformInSnapshot(ctx, snapshot, req)
	end, endErr := mouseProvider.CurrentMousePosition(ctx)
	if endErr != nil || end == start {
		return result, actionErr
	}

	restoreErr := mouseProvider.SetMousePosition(ctx, start)
	if actionErr != nil {
		if restoreErr != nil {
			return result, fmt.Errorf("%w; additionally failed to restore primary mouse position: %v", actionErr, restoreErr)
		}
		return result, actionErr
	}
	if restoreErr != nil {
		return backgroundmouse.ActionResult{}, fmt.Errorf("background_virtual_primary_mouse_restore_failed: %w", restoreErr)
	}
	return result, nil
}

func parseBackgroundMouseActionKind(value string) (backgroundmouse.ActionKind, error) {
	switch kind := backgroundmouse.ActionKind(strings.ToLower(strings.TrimSpace(value))); kind {
	case backgroundmouse.ActionClick, backgroundmouse.ActionDoubleClick, backgroundmouse.ActionFocus, backgroundmouse.ActionRightClick, backgroundmouse.ActionShowMenu:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: unsupported background mouse action %q", backgroundmouse.ErrActionUnsupported, value)
	}
}

func normalizeBackgroundMouseError(err error) error {
	switch {
	case errors.Is(err, backgroundmouse.ErrUnresolved):
		return fmt.Errorf("unresolved_background_action: %w", err)
	case errors.Is(err, backgroundmouse.ErrActionUnsupported), errors.Is(err, backgroundmouse.ErrUnsupportedPlatform):
		return fmt.Errorf("unsupported_background_action: %w", err)
	default:
		return err
	}
}

func backgroundMouseActionNames(actions []backgroundmouse.ActionKind) []string {
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		result = append(result, string(action))
	}
	return result
}

func windowAccessibilityVirtualPoint(element WindowAccessibilityElement) (Point, bool) {
	if element.ActionPointKnown {
		return element.ActionPoint, true
	}
	if element.ScreenRegion != nil {
		return centerPoint(*element.ScreenRegion), true
	}
	return Point{}, false
}

func emitBackgroundVirtualMove(previous *Point, next Point) {
	start := next
	if previous != nil {
		start = *previous
	}
	target := common.Point{X: next.X, Y: next.Y}
	_ = showAgentCursorOverlay(libnutcore.AgentCursorEvent{
		Kind:     libnutcore.AgentCursorEventMove,
		Position: common.Point{X: start.X, Y: start.Y},
		Target:   &target,
		Duration: agentCursorMotionDuration(start, next, false),
	})
}

func emitBackgroundVirtualAction(previous *Point, next Point, kind backgroundmouse.ActionKind) {
	if previous == nil || *previous != next {
		emitBackgroundVirtualMove(previous, next)
	}

	event := libnutcore.AgentCursorEvent{
		Position: common.Point{X: next.X, Y: next.Y},
	}
	switch kind {
	case backgroundmouse.ActionFocus:
		return
	case backgroundmouse.ActionDoubleClick:
		event.Kind = libnutcore.AgentCursorEventDoubleClick
		event.Button = common.MouseButtonLeft
	case backgroundmouse.ActionRightClick, backgroundmouse.ActionShowMenu:
		event.Kind = libnutcore.AgentCursorEventClick
		event.Button = common.MouseButtonRight
	default:
		event.Kind = libnutcore.AgentCursorEventClick
		event.Button = common.MouseButtonLeft
	}
	_ = showAgentCursorOverlay(event)
}

func copyPointPointer(point *Point) *Point {
	if point == nil {
		return nil
	}
	copy := *point
	return &copy
}

func pointFromSharedPoint(point shared.Point) Point {
	return Point{X: point.X, Y: point.Y}
}
