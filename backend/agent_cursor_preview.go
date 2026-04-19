package backend

import (
	"time"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/native/libnutcore"
)

var (
	showPreviewAgentCursorOverlay = libnutcore.ShowAgentCursor
	hidePreviewAgentCursorOverlay = libnutcore.HideAgentCursor
	runPreviewAgentCursorAsync    = func(fn func()) { go fn() }
	sleepPreviewAgentCursor       = time.Sleep
)

func (s *Service) PreviewAgentCursor() (ActionResult, error) {
	start := Point{X: 720, Y: 480}
	if point, err := s.GetMousePosition(); err == nil {
		start = point
	}
	target := Point{X: start.X + 140, Y: start.Y + 180}
	if size, err := s.GetScreenSize(); err == nil {
		target = Point{
			X: clampInt(target.X, 80, max(80, size.Width-80)),
			Y: clampInt(target.Y, 80, max(80, size.Height-80)),
		}
	}

	runPreviewAgentCursorAsync(func() {
		targetPoint := common.Point{X: target.X, Y: target.Y}
		_ = showPreviewAgentCursorOverlay(libnutcore.AgentCursorEvent{
			Kind:     libnutcore.AgentCursorEventMove,
			Position: common.Point{X: start.X, Y: start.Y},
			Target:   &targetPoint,
			Duration: 950 * time.Millisecond,
		})
		sleepPreviewAgentCursor(1100 * time.Millisecond)

		_ = showPreviewAgentCursorOverlay(libnutcore.AgentCursorEvent{
			Kind:     libnutcore.AgentCursorEventClick,
			Position: targetPoint,
			Button:   common.MouseButtonLeft,
		})
		sleepPreviewAgentCursor(250 * time.Millisecond)

		_ = showPreviewAgentCursorOverlay(libnutcore.AgentCursorEvent{
			Kind:      libnutcore.AgentCursorEventScroll,
			Position:  targetPoint,
			Direction: libnutcore.AgentCursorDirectionDown,
		})
		sleepPreviewAgentCursor(1400 * time.Millisecond)

		_ = hidePreviewAgentCursorOverlay()
	})

	return ActionResult{
		OK:      true,
		Message: "Previewing the pink agent cursor overlay on the desktop.",
	}, nil
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
