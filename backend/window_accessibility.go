package backend

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PandelisZ/gut"
	"github.com/PandelisZ/gut/backgroundmouse"
	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/shared"
)

func (s *Service) GetWindowAccessibilitySnapshot(req WindowAccessibilitySnapshotRequest) (WindowAccessibilitySnapshotResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	window, err := s.windowSummaryForAccessibilitySnapshot(ctx, req.Handle)
	if err != nil {
		return WindowAccessibilitySnapshotResult{}, err
	}

	snapshot, err := s.nut.BackgroundMouse.SnapshotWindow(ctx, shared.WindowHandle(window.Handle))
	if err != nil {
		return WindowAccessibilitySnapshotResult{}, err
	}

	elements, cache := flattenWindowAccessibilitySnapshot(snapshot)
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
	return s.actOnWindowAccessibilityElement(req, false)
}

func (s *Service) ResolveBackgroundWindowPoint(req BackgroundMouseResolveRequest) (BackgroundMouseResolveResult, error) {
	snapshotID := strings.TrimSpace(req.SnapshotID)
	if snapshotID == "" {
		return BackgroundMouseResolveResult{}, fmt.Errorf("snapshot_id is required")
	}

	requested := shared.Point{X: req.X, Y: req.Y}
	resolution, elementID, element, err := s.resolveBackgroundWindowPoint(snapshotID, requested)
	if err != nil {
		return BackgroundMouseResolveResult{}, err
	}

	return BackgroundMouseResolveResult{
		SnapshotID:     snapshotID,
		RequestedPoint: Point{X: requested.X, Y: requested.Y},
		ScreenPoint:    pointFromSharedPoint(resolution.ScreenPoint),
		Snapped:        resolution.Snapped,
		ElementID:      elementID,
		Element:        element,
		Mode:           "background_virtual",
		Message:        fmt.Sprintf("Resolved background virtual pointer to %s at (%d, %d).", elementID, resolution.ScreenPoint.X, resolution.ScreenPoint.Y),
	}, nil
}

func (s *Service) PerformBackgroundWindowAction(req BackgroundMouseActionRequest) (BackgroundMouseActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snapshotID := strings.TrimSpace(req.SnapshotID)
	if snapshotID == "" {
		return BackgroundMouseActionResult{}, fmt.Errorf("snapshot_id is required")
	}

	kind, err := parseBackgroundMouseActionKind(req.Action)
	if err != nil {
		return BackgroundMouseActionResult{}, normalizeBackgroundMouseError(err)
	}

	s.mu.Lock()
	snapshot, ok := s.accessibilitySnapshots[snapshotID]
	s.mu.Unlock()
	if !ok {
		return BackgroundMouseActionResult{}, fmt.Errorf("unknown snapshot_id %q", snapshotID)
	}

	actionReq := backgroundmouse.SnapshotActionRequest{Kind: kind}
	var requestedPoint *Point
	if req.Point != nil {
		point := shared.Point{X: req.Point.X, Y: req.Point.Y}
		actionReq.Point = &point
		requestedPoint = &Point{X: req.Point.X, Y: req.Point.Y}
	} else if elementID := strings.TrimSpace(req.ElementID); elementID != "" {
		element, ok := snapshot.Elements[elementID]
		if !ok {
			return BackgroundMouseActionResult{}, fmt.Errorf("unresolved_background_action: unknown element_id %q for snapshot %q", elementID, snapshotID)
		}
		ref, ok := windowAccessibilityElementRef(element)
		if !ok {
			return BackgroundMouseActionResult{}, fmt.Errorf("unresolved_background_action: element %q does not expose an AX ref", elementID)
		}
		actionReq.Ref = &ref
	} else {
		return BackgroundMouseActionResult{}, fmt.Errorf("unresolved_background_action: point or element_id is required")
	}

	result, err := s.performStrictBackgroundWindowAction(ctx, snapshot.BackgroundSnapshot, actionReq)
	if err != nil {
		return BackgroundMouseActionResult{}, normalizeBackgroundMouseError(err)
	}

	screenPoint := pointFromSharedPoint(result.ScreenPoint)
	if screenPoint == (Point{}) {
		if elementID := strings.TrimSpace(req.ElementID); elementID != "" {
			element := snapshot.Elements[elementID]
			if virtualPoint, ok := windowAccessibilityVirtualPoint(element); ok {
				screenPoint = virtualPoint
			}
		}
	}

	s.mu.Lock()
	snapshot, ok = s.accessibilitySnapshots[snapshotID]
	if !ok {
		s.mu.Unlock()
		return BackgroundMouseActionResult{}, fmt.Errorf("unknown snapshot_id %q", snapshotID)
	}
	elementID, ok := snapshot.ElementIDsByRef[agentAXRefKey(result.MatchedRef)]
	if !ok {
		s.mu.Unlock()
		return BackgroundMouseActionResult{}, fmt.Errorf("unresolved_background_action: resolved background ref %+v is not cached in snapshot %q", result.MatchedRef, snapshotID)
	}
	element := snapshot.Elements[elementID]
	previous := copyPointPointer(snapshot.LastVirtualPoint)
	if screenPoint != (Point{}) {
		next := screenPoint
		snapshot.LastVirtualPoint = &next
	}
	s.accessibilitySnapshots[snapshotID] = snapshot
	s.mu.Unlock()

	if screenPoint != (Point{}) {
		emitBackgroundVirtualAction(previous, screenPoint, kind)
	}

	message := fmt.Sprintf("Executed %s on %s via background_virtual.", req.Action, elementID)
	if screenPoint != (Point{}) {
		message = fmt.Sprintf("Executed %s on %s at (%d, %d) via background_virtual.", req.Action, elementID, screenPoint.X, screenPoint.Y)
	}

	return BackgroundMouseActionResult{
		SnapshotID:     snapshotID,
		Action:         string(kind),
		RequestedPoint: requestedPoint,
		ScreenPoint:    screenPoint,
		Snapped:        result.Snapped,
		ElementID:      elementID,
		Element:        element,
		Mode:           "background_virtual",
		Result: ActionResult{
			OK:      true,
			Message: message,
		},
		Message: message,
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

func flattenWindowAccessibilitySnapshot(snapshot backgroundmouse.WindowSnapshot) ([]WindowAccessibilityElement, windowAccessibilitySnapshotCache) {
	elements := make([]WindowAccessibilityElement, 0, len(snapshot.Elements))
	cache := windowAccessibilitySnapshotCache{
		Elements:           make(map[string]WindowAccessibilityElement, len(snapshot.Elements)),
		ElementIDsByRef:    make(map[string]string, len(snapshot.Elements)),
		BackgroundSnapshot: snapshot,
	}
	for _, match := range snapshot.Elements {
		id := fmt.Sprintf("el-%03d", len(elements)+1)
		element := windowAccessibilityElementFromBackgroundElement(id, match)
		elements = append(elements, element)
		cache.Elements[id] = element
		cache.ElementIDsByRef[agentAXRefKey(match.Ref)] = id
	}
	return elements, cache
}

func windowAccessibilityElementFromBackgroundElement(id string, match backgroundmouse.SnapshotElement) WindowAccessibilityElement {
	result := WindowAccessibilityElement{
		ID:                    id,
		Path:                  windowAccessibilityPath(match.Ref.Path),
		Depth:                 match.Depth,
		Role:                  strings.TrimSpace(match.Metadata.Role),
		Subrole:               strings.TrimSpace(match.Metadata.Subrole),
		Title:                 strings.TrimSpace(match.Metadata.Title),
		Value:                 strings.TrimSpace(match.Metadata.Value),
		Enabled:               match.Metadata.Enabled,
		Focused:               match.Metadata.Focused,
		AXActions:             append([]string(nil), match.Metadata.Actions...),
		ActionPoint:           pointFromSharedPoint(match.ActionPoint),
		ActionPointKnown:      match.ActionPointKnown,
		AXRef:                 axElementRefPointer(match.Ref),
		BackgroundSafeActions: backgroundMouseActionNames(match.BackgroundSafeActions),
		EnabledKnown:          true,
		FocusedKnown:          true,
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
	if element.ActionPointKnown {
		return element.ActionPoint, nil
	}
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
		if element.ActionPointKnown {
			builder.WriteString(fmt.Sprintf("%s  - action point `(%d, %d)`\n", indent, element.ActionPoint.X, element.ActionPoint.Y))
		}
		if len(element.BackgroundSafeActions) > 0 {
			builder.WriteString(fmt.Sprintf("%s  - background-safe actions: %s\n", indent, backtickJoin(element.BackgroundSafeActions)))
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

func (s *Service) resolveBackgroundWindowPoint(snapshotID string, requested shared.Point) (backgroundmouse.PointResolution, string, WindowAccessibilityElement, error) {
	s.mu.Lock()
	snapshot, ok := s.accessibilitySnapshots[snapshotID]
	s.mu.Unlock()
	if !ok {
		return backgroundmouse.PointResolution{}, "", WindowAccessibilityElement{}, fmt.Errorf("unknown snapshot_id %q", snapshotID)
	}

	resolution, err := s.nut.BackgroundMouse.ResolveInSnapshot(snapshot.BackgroundSnapshot, requested)
	if err != nil {
		return backgroundmouse.PointResolution{}, "", WindowAccessibilityElement{}, normalizeBackgroundMouseError(err)
	}

	screenPoint := pointFromSharedPoint(resolution.ScreenPoint)

	s.mu.Lock()
	snapshot, ok = s.accessibilitySnapshots[snapshotID]
	if !ok {
		s.mu.Unlock()
		return backgroundmouse.PointResolution{}, "", WindowAccessibilityElement{}, fmt.Errorf("unknown snapshot_id %q", snapshotID)
	}
	elementID, ok := snapshot.ElementIDsByRef[agentAXRefKey(resolution.MatchedRef)]
	if !ok {
		s.mu.Unlock()
		return backgroundmouse.PointResolution{}, "", WindowAccessibilityElement{}, fmt.Errorf("unresolved_background_action: resolved background ref %+v is not cached in snapshot %q", resolution.MatchedRef, snapshotID)
	}
	element := snapshot.Elements[elementID]
	previous := copyPointPointer(snapshot.LastVirtualPoint)
	next := screenPoint
	snapshot.LastVirtualPoint = &next
	s.accessibilitySnapshots[snapshotID] = snapshot
	s.mu.Unlock()

	emitBackgroundVirtualMove(previous, screenPoint)
	return resolution, elementID, element, nil
}

func windowAccessibilityElementRef(element WindowAccessibilityElement) (common.AXElementRef, bool) {
	if element.AXRef == nil {
		return common.AXElementRef{}, false
	}
	ref, err := parseAXElementRef(*element.AXRef)
	if err != nil {
		return common.AXElementRef{}, false
	}
	return ref, true
}
