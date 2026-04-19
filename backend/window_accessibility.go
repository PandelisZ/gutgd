package backend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PandelisZ/gut"
	"github.com/PandelisZ/gut/native/common"
)

const defaultMaxWindowAccessibilityElements = 400

func (s *Service) GetWindowAccessibilitySnapshot(req WindowAccessibilitySnapshotRequest) (WindowAccessibilitySnapshotResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	window, err := s.windowSummaryForAccessibilitySnapshot(ctx, req.Handle)
	if err != nil {
		return WindowAccessibilitySnapshotResult{}, err
	}

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return WindowAccessibilitySnapshotResult{}, err
	}
	matches, err := accessibility.SearchAXElements(ctx, common.AXElementSearchQuery{
		Scope:        common.AXSearchScopeWindowHandle,
		WindowHandle: common.WindowHandle(window.Handle),
		Limit:        defaultMaxWindowAccessibilityElements,
		MaxDepth:     defaultMaxWindowAccessibilityElements,
	})
	if err != nil {
		return WindowAccessibilitySnapshotResult{}, err
	}

	elements, cache := flattenWindowAccessibilityMatches(matches)
	snapshotID := fmt.Sprintf("axwin-%d-%d", window.Handle, time.Now().UnixNano())
	cache.Window = window
	s.accessibilitySnapshots[snapshotID] = cache

	return WindowAccessibilitySnapshotResult{
		SnapshotID:   snapshotID,
		Window:       window,
		ElementCount: len(elements),
		Elements:     elements,
		Markdown:     formatWindowAccessibilitySnapshotMarkdown(snapshotID, window, elements),
		Message:      fmt.Sprintf("Captured %d accessible elements for window %q.", len(elements), window.Title),
	}, nil
}

func (s *Service) ActOnWindowAccessibilityElement(req WindowAccessibilityElementActionRequest) (WindowAccessibilityElementActionResult, error) {
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

	screenPoint := Point{}
	if element.ScreenRegion != nil {
		screenPoint = centerPoint(*element.ScreenRegion)
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

func (s *Service) windowSummaryForAccessibilitySnapshot(ctx context.Context, handle uint64) (WindowSummary, error) {
	if handle == 0 {
		window, err := gut.GetActiveWindow(ctx, s.nut.Registry)
		if err != nil {
			return WindowSummary{}, err
		}
		return s.windowSummary(ctx, window)
	}

	window, err := s.windowByHandle(ctx, handle)
	if err != nil {
		return WindowSummary{}, err
	}
	return s.windowSummary(ctx, window)
}

func (s *Service) performWindowAccessibilityAction(window WindowSummary, element WindowAccessibilityElement, action string) (ActionResult, string, error) {
	switch action {
	case "focus":
		if element.AXRef != nil {
			result, err := s.FocusAXElement(FocusAXElementRequest{Ref: *element.AXRef})
			if err == nil {
				return ActionResult{OK: result.OK, Message: result.Message}, "background_ax", nil
			}
		}
		return s.focusWindowAccessibilityElement(window.Handle, element)
	case "click":
		if element.AXRef != nil {
			if axAction, ok := windowAccessibilityClickAXAction(element.AXActions); ok {
				result, err := s.PerformAXElementAction(PerformAXElementActionOnRefRequest{
					Ref:    *element.AXRef,
					Action: string(axAction),
				})
				if err == nil {
					return ActionResult{OK: result.OK, Message: result.Message}, "background_ax", nil
				}
			}
		}
		return s.clickWindowAccessibilityElement(window.Handle, element, "left", false)
	case "double_click":
		return s.clickWindowAccessibilityElement(window.Handle, element, "left", true)
	case "right_click":
		return s.clickWindowAccessibilityElement(window.Handle, element, "right", false)
	case "show_menu":
		if element.AXRef != nil && windowAccessibilityHasAXAction(element.AXActions, common.AXShowMenu) {
			result, err := s.PerformAXElementAction(PerformAXElementActionOnRefRequest{
				Ref:    *element.AXRef,
				Action: string(common.AXShowMenu),
			})
			if err == nil {
				return ActionResult{OK: result.OK, Message: result.Message}, "background_ax", nil
			}
		}
		return s.clickWindowAccessibilityElement(window.Handle, element, "right", false)
	default:
		return ActionResult{}, "", fmt.Errorf("unsupported action %q", action)
	}
}

func (s *Service) focusWindowAccessibilityElement(windowHandle uint64, element WindowAccessibilityElement) (ActionResult, string, error) {
	screenPoint, err := requireWindowAccessibilityScreenPoint(element)
	if err != nil {
		return ActionResult{}, "", err
	}

	return s.runWithFocusedWindowForRawInput(windowHandle, func() (ActionResult, error) {
		return s.FocusElementAtPoint(PointRequest{X: screenPoint.X, Y: screenPoint.Y})
	})
}

func (s *Service) clickWindowAccessibilityElement(windowHandle uint64, element WindowAccessibilityElement, button string, double bool) (ActionResult, string, error) {
	screenPoint, err := requireWindowAccessibilityScreenPoint(element)
	if err != nil {
		return ActionResult{}, "", err
	}

	return s.runWithFocusedWindowForRawInput(windowHandle, func() (ActionResult, error) {
		if _, err := s.SetMousePosition(MouseMoveRequest{X: screenPoint.X, Y: screenPoint.Y}); err != nil {
			return ActionResult{}, err
		}
		if double {
			return s.DoubleClickMouse(MouseButtonRequest{Button: button})
		}
		return s.ClickMouse(MouseButtonRequest{Button: button})
	})
}

func flattenWindowAccessibilityMatches(matches []common.AXElementMatch) ([]WindowAccessibilityElement, windowAccessibilitySnapshotCache) {
	elements := make([]WindowAccessibilityElement, 0, len(matches))
	cache := windowAccessibilitySnapshotCache{
		Elements: make(map[string]WindowAccessibilityElement, len(matches)),
	}
	for _, match := range matches {
		id := fmt.Sprintf("el-%03d", len(elements)+1)
		element := windowAccessibilityElementFromAXMatch(id, match)
		elements = append(elements, element)
		cache.Elements[id] = element
	}
	return elements, cache
}

func windowAccessibilityElementFromAXMatch(id string, match common.AXElementMatch) WindowAccessibilityElement {
	result := WindowAccessibilityElement{
		ID:           id,
		Path:         windowAccessibilityPath(match.Ref.Path),
		Depth:        match.Depth,
		Role:         strings.TrimSpace(match.Metadata.Role),
		Subrole:      strings.TrimSpace(match.Metadata.Subrole),
		Title:        strings.TrimSpace(match.Metadata.Title),
		Value:        strings.TrimSpace(match.Metadata.Value),
		Enabled:      match.Metadata.Enabled,
		Focused:      match.Metadata.Focused,
		AXActions:    append([]string(nil), match.Metadata.Actions...),
		AXRef:        axElementRefPointer(match.Ref),
		EnabledKnown: true,
		FocusedKnown: true,
	}
	if match.Metadata.FrameKnown {
		region := regionFromRect(match.Metadata.Frame)
		result.ScreenRegion = &region
	}
	result.AvailableActions = windowAccessibilityAvailableActions(result)
	return result
}

func windowAccessibilityAvailableActions(element WindowAccessibilityElement) []string {
	actions := make([]string, 0, 5)
	if element.AXRef != nil || element.ScreenRegion != nil {
		actions = append(actions, "focus")
	}
	if _, ok := windowAccessibilityClickAXAction(element.AXActions); ok || element.ScreenRegion != nil {
		actions = append(actions, "click")
	}
	if element.ScreenRegion != nil {
		actions = append(actions, "double_click", "right_click")
	}
	role := strings.ToLower(strings.TrimSpace(element.Role))
	subrole := strings.ToLower(strings.TrimSpace(element.Subrole))
	if windowAccessibilityHasAXAction(element.AXActions, common.AXShowMenu) || element.ScreenRegion != nil || strings.Contains(role, "menu") || strings.Contains(subrole, "menu") {
		actions = append(actions, "show_menu")
	}
	return uniqueStrings(actions)
}

func windowAccessibilityClickAXAction(actions []string) (common.AXAction, bool) {
	for _, candidate := range []common.AXAction{common.AXPress, common.AXConfirm, common.AXPick} {
		if windowAccessibilityHasAXAction(actions, candidate) {
			return candidate, true
		}
	}
	return "", false
}

func windowAccessibilityHasAXAction(actions []string, target common.AXAction) bool {
	for _, action := range actions {
		if strings.TrimSpace(action) == string(target) {
			return true
		}
	}
	return false
}

func requireWindowAccessibilityScreenPoint(element WindowAccessibilityElement) (Point, error) {
	if element.ScreenRegion == nil {
		return Point{}, fmt.Errorf("element %q does not expose a known screen region", element.ID)
	}
	return centerPoint(*element.ScreenRegion), nil
}

func windowAccessibilityPath(path []int) string {
	if len(path) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(path)+1)
	parts = append(parts, "0")
	for _, entry := range path {
		parts = append(parts, fmt.Sprintf("%d", entry+1))
	}
	return strings.Join(parts, ".")
}

func formatWindowAccessibilitySnapshotMarkdown(snapshotID string, window WindowSummary, elements []WindowAccessibilityElement) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("# Window accessibility snapshot\n\n- Snapshot ID: `%s`\n- Window: `%s` (`%d`)\n- Screen region: `(%d, %d)` `%dx%d`\n- Element count: `%d`\n\n## Elements\n",
		snapshotID,
		window.Title,
		window.Handle,
		window.Region.Left,
		window.Region.Top,
		window.Region.Width,
		window.Region.Height,
		len(elements),
	))

	for _, element := range elements {
		indent := strings.Repeat("  ", element.Depth)
		label := firstNonEmpty(element.Title, element.Value, element.SelectedText, element.Role, element.Type, "element")
		builder.WriteString(fmt.Sprintf("%s- `%s` `%s` — %s\n", indent, element.ID, element.Path, markdownInline(label)))
		meta := make([]string, 0, 7)
		if element.Role != "" {
			meta = append(meta, fmt.Sprintf("role `%s`", element.Role))
		}
		if element.Subrole != "" {
			meta = append(meta, fmt.Sprintf("subrole `%s`", element.Subrole))
		}
		if element.Type != "" {
			meta = append(meta, fmt.Sprintf("type `%s`", element.Type))
		}
		if element.EnabledKnown {
			meta = append(meta, fmt.Sprintf("enabled `%t`", element.Enabled))
		}
		if element.FocusedKnown {
			meta = append(meta, fmt.Sprintf("focused `%t`", element.Focused))
		}
		if len(meta) > 0 {
			builder.WriteString(fmt.Sprintf("%s  - %s\n", indent, strings.Join(meta, ", ")))
		}
		if element.AXRef != nil {
			builder.WriteString(fmt.Sprintf("%s  - ax ref scope `%s`, window `%d`\n", indent, element.AXRef.Scope, element.AXRef.WindowHandle))
		}
		if len(element.AXActions) > 0 {
			builder.WriteString(fmt.Sprintf("%s  - ax actions: %s\n", indent, backtickJoin(element.AXActions)))
		}
		if element.ScreenRegion != nil {
			builder.WriteString(fmt.Sprintf("%s  - screen region `(%d, %d)` `%dx%d`\n", indent, element.ScreenRegion.Left, element.ScreenRegion.Top, element.ScreenRegion.Width, element.ScreenRegion.Height))
		}
		if len(element.AvailableActions) > 0 {
			builder.WriteString(fmt.Sprintf("%s  - available actions: %s\n", indent, backtickJoin(element.AvailableActions)))
		}
	}

	builder.WriteString("\nUse `snapshot_id` plus one of the element IDs with `act_on_window_accessibility_element` to act on a known control without re-guessing coordinates.")
	return builder.String()
}

func centerPoint(region Region) Point {
	return Point{
		X: region.Left + region.Width/2,
		Y: region.Top + region.Height/2,
	}
}

func axElementRefPointer(ref common.AXElementRef) *AXElementRefResult {
	result := axElementRefResultFromCommon(ref)
	return &result
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
