package backend

import (
	"fmt"
	"strings"
)

func (s *Service) actOnWindowAccessibilityElement(req WindowAccessibilityElementActionRequest, strictBackgroundOnly bool) (WindowAccessibilityElementActionResult, error) {
	snapshotID := strings.TrimSpace(req.SnapshotID)
	elementID := strings.TrimSpace(req.ElementID)
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if snapshotID == "" {
		return WindowAccessibilityElementActionResult{}, fmt.Errorf("snapshot_id is required")
	}
	if elementID == "" {
		return WindowAccessibilityElementActionResult{}, fmt.Errorf("element_id is required")
	}
	if action == "" {
		return WindowAccessibilityElementActionResult{}, fmt.Errorf("action is required")
	}

	s.mu.Lock()
	snapshot, ok := s.accessibilitySnapshots[snapshotID]
	if !ok {
		s.mu.Unlock()
		return WindowAccessibilityElementActionResult{}, fmt.Errorf("unknown snapshot_id %q", snapshotID)
	}
	element, ok := snapshot.Elements[elementID]
	s.mu.Unlock()
	if !ok {
		return WindowAccessibilityElementActionResult{}, fmt.Errorf("unknown element_id %q for snapshot %q", elementID, snapshotID)
	}

	if strictBackgroundOnly {
		backgroundResult, err := s.PerformBackgroundWindowAction(BackgroundMouseActionRequest{
			SnapshotID: snapshotID,
			ElementID:  elementID,
			Action:     action,
		})
		if err != nil {
			return WindowAccessibilityElementActionResult{}, err
		}
		return WindowAccessibilityElementActionResult{
			SnapshotID:  snapshotID,
			ElementID:   elementID,
			Action:      action,
			ScreenPoint: backgroundResult.ScreenPoint,
			Mode:        backgroundResult.Mode,
			Result:      backgroundResult.Result,
			Message:     backgroundResult.Message,
		}, nil
	}

	screenPoint := Point{}
	if point, err := requireWindowAccessibilityScreenPoint(element); err == nil {
		screenPoint = point
	}

	result, mode, err := s.performWindowAccessibilityAction(snapshot.Window, element, action)
	if err != nil {
		return WindowAccessibilityElementActionResult{}, err
	}

	message := fmt.Sprintf("Executed %s on %s via %s.", action, elementID, mode)
	if element.ScreenRegion != nil {
		message = fmt.Sprintf("Executed %s on %s at (%d, %d) via %s.", action, elementID, screenPoint.X, screenPoint.Y, mode)
	}

	return WindowAccessibilityElementActionResult{
		SnapshotID:  snapshotID,
		ElementID:   elementID,
		Action:      action,
		ScreenPoint: screenPoint,
		Mode:        mode,
		Result:      result,
		Message:     message,
	}, nil
}
