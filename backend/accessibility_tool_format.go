package backend

import (
	"encoding/json"
	"fmt"
	"strings"
)

func formatAccessibilityToolPayload(value any) (string, bool) {
	switch typed := value.(type) {
	case WindowAccessibilitySnapshotResult:
		return strings.TrimSpace(typed.Markdown), true
	case *WindowAccessibilitySnapshotResult:
		if typed == nil {
			return "", false
		}
		return strings.TrimSpace(typed.Markdown), true
	case SearchAXElementsResult:
		return formatSearchAXElementsResultMarkdown(typed), true
	case *SearchAXElementsResult:
		if typed == nil {
			return "", false
		}
		return formatSearchAXElementsResultMarkdown(*typed), true
	case FocusedWindowMetadataResult:
		return formatFocusedWindowMetadataMarkdown(typed), true
	case *FocusedWindowMetadataResult:
		if typed == nil {
			return "", false
		}
		return formatFocusedWindowMetadataMarkdown(*typed), true
	case UIElementMetadataResult:
		return formatUIElementMetadataMarkdown("Accessible element", typed), true
	case *UIElementMetadataResult:
		if typed == nil {
			return "", false
		}
		return formatUIElementMetadataMarkdown("Accessible element", *typed), true
	case FocusAXElementResult:
		return formatFocusAXElementMarkdown(typed), true
	case *FocusAXElementResult:
		if typed == nil {
			return "", false
		}
		return formatFocusAXElementMarkdown(*typed), true
	case PerformAXElementActionResult:
		return formatPerformAXElementActionMarkdown(typed), true
	case *PerformAXElementActionResult:
		if typed == nil {
			return "", false
		}
		return formatPerformAXElementActionMarkdown(*typed), true
	case WindowAccessibilityElementActionResult:
		return formatWindowAccessibilityElementActionMarkdown(typed), true
	case *WindowAccessibilityElementActionResult:
		if typed == nil {
			return "", false
		}
		return formatWindowAccessibilityElementActionMarkdown(*typed), true
	case BackgroundMouseResolveResult:
		return formatBackgroundMouseResolveMarkdown(typed), true
	case *BackgroundMouseResolveResult:
		if typed == nil {
			return "", false
		}
		return formatBackgroundMouseResolveMarkdown(*typed), true
	case BackgroundMouseActionResult:
		return formatBackgroundMouseActionMarkdown(typed), true
	case *BackgroundMouseActionResult:
		if typed == nil {
			return "", false
		}
		return formatBackgroundMouseActionMarkdown(*typed), true
	case AgentTranslatedPointResult:
		return formatAgentTranslatedPointMarkdown(typed)
	case *AgentTranslatedPointResult:
		if typed == nil {
			return "", false
		}
		return formatAgentTranslatedPointMarkdown(*typed)
	default:
		return "", false
	}
}

func formatFocusedWindowMetadataMarkdown(result FocusedWindowMetadataResult) string {
	lines := []string{
		"# Focused window",
		fmt.Sprintf("- title: `%s`", markdownInline(firstNonEmpty(result.Title, "(untitled window)"))),
		fmt.Sprintf("- handle: `%d`", result.Handle),
	}
	if result.OwnerName != "" || result.OwnerPID != 0 {
		lines = append(lines, fmt.Sprintf("- owner: `%s` pid `%d`", markdownInline(firstNonEmpty(result.OwnerName, "unknown")), result.OwnerPID))
	}
	if result.BundleID != "" {
		lines = append(lines, fmt.Sprintf("- bundle id: `%s`", markdownInline(result.BundleID)))
	}
	if result.Role != "" || result.Subrole != "" {
		lines = append(lines, fmt.Sprintf("- role: `%s`  subrole: `%s`", markdownInline(firstNonEmpty(result.Role, "unknown")), markdownInline(firstNonEmpty(result.Subrole, "none"))))
	}
	lines = append(lines, fmt.Sprintf("- focused: `%t`  main: `%t`  minimized: `%t`", result.Focused, result.Main, result.Minimized))
	if result.RegionKnown {
		lines = append(lines, fmt.Sprintf("- region: `(%d, %d)` `%dx%d`", result.Region.Left, result.Region.Top, result.Region.Width, result.Region.Height))
	} else {
		lines = append(lines, "- region: `unknown`")
	}
	return strings.Join(lines, "\n")
}

func formatUIElementMetadataMarkdown(title string, result UIElementMetadataResult) string {
	lines := []string{
		fmt.Sprintf("# %s", title),
		fmt.Sprintf("- label: `%s`", markdownInline(firstNonEmpty(result.Title, result.Value, result.Description, result.Role, "unnamed"))),
	}
	if result.Role != "" || result.Subrole != "" {
		lines = append(lines, fmt.Sprintf("- role: `%s`  subrole: `%s`", markdownInline(firstNonEmpty(result.Role, "unknown")), markdownInline(firstNonEmpty(result.Subrole, "none"))))
	}
	if strings.TrimSpace(result.Description) != "" {
		lines = append(lines, fmt.Sprintf("- description: `%s`", markdownInline(result.Description)))
	}
	if strings.TrimSpace(result.Value) != "" && strings.TrimSpace(result.Value) != strings.TrimSpace(result.Title) {
		lines = append(lines, fmt.Sprintf("- value: `%s`", markdownInline(result.Value)))
	}
	lines = append(lines, fmt.Sprintf("- enabled: `%t`  focused: `%t`", result.Enabled, result.Focused))
	if result.FrameKnown {
		lines = append(lines, fmt.Sprintf("- frame: `(%d, %d)` `%dx%d`", result.Frame.Left, result.Frame.Top, result.Frame.Width, result.Frame.Height))
	} else {
		lines = append(lines, "- frame: `unknown`")
	}
	if len(result.Actions) > 0 {
		lines = append(lines, fmt.Sprintf("- actions: %s", backtickJoin(result.Actions)))
	}
	return strings.Join(lines, "\n")
}

func formatSearchAXElementsResultMarkdown(result SearchAXElementsResult) string {
	var builder strings.Builder
	builder.WriteString("# AX search results\n\n")
	if strings.TrimSpace(result.Message) != "" {
		builder.WriteString(result.Message)
		builder.WriteString("\n\n")
	}

	builder.WriteString("## Query\n")
	builder.WriteString(fmt.Sprintf("- scope: `%s`\n", markdownInline(firstNonEmpty(result.Query.Scope, "unknown"))))
	if result.Query.WindowHandle != 0 {
		builder.WriteString(fmt.Sprintf("- window handle: `%d`\n", result.Query.WindowHandle))
	}
	filters := searchAXFilters(result.Query)
	if len(filters) > 0 {
		builder.WriteString(fmt.Sprintf("- filters: %s\n", strings.Join(filters, ", ")))
	}
	builder.WriteString(fmt.Sprintf("- limit: `%d`  max depth: `%d`\n", result.Query.Limit, result.Query.MaxDepth))

	builder.WriteString("\n## Matches\n")
	if len(result.Matches) == 0 {
		builder.WriteString("- No matches found.\n")
		return strings.TrimSpace(builder.String())
	}

	for index, match := range result.Matches {
		label := markdownInline(firstNonEmpty(match.Metadata.Title, match.Metadata.Value, match.Metadata.Description, match.Metadata.Role, "match"))
		builder.WriteString(fmt.Sprintf("%d. `%s`\n", index+1, label))
		builder.WriteString(fmt.Sprintf("   - ref: `%s`\n", compactAXRefJSON(match.Ref)))
		builder.WriteString(fmt.Sprintf("   - depth: `%d`\n", match.Depth))
		builder.WriteString(fmt.Sprintf("   - role: `%s`  subrole: `%s`\n", markdownInline(firstNonEmpty(match.Metadata.Role, "unknown")), markdownInline(firstNonEmpty(match.Metadata.Subrole, "none"))))
		builder.WriteString(fmt.Sprintf("   - enabled: `%t`  focused: `%t`\n", match.Metadata.Enabled, match.Metadata.Focused))
		if match.Metadata.FrameKnown {
			builder.WriteString(fmt.Sprintf("   - frame: `(%d, %d)` `%dx%d`\n", match.Metadata.Frame.Left, match.Metadata.Frame.Top, match.Metadata.Frame.Width, match.Metadata.Frame.Height))
		}
		if match.ActionPointKnown {
			builder.WriteString(fmt.Sprintf("   - action point: `(%d, %d)`\n", match.ActionPoint.X, match.ActionPoint.Y))
		}
		if strings.TrimSpace(match.Metadata.Description) != "" {
			builder.WriteString(fmt.Sprintf("   - description: `%s`\n", markdownInline(match.Metadata.Description)))
		}
		if strings.TrimSpace(match.Metadata.Value) != "" && strings.TrimSpace(match.Metadata.Value) != strings.TrimSpace(match.Metadata.Title) {
			builder.WriteString(fmt.Sprintf("   - value: `%s`\n", markdownInline(match.Metadata.Value)))
		}
		if len(match.Metadata.Actions) > 0 {
			builder.WriteString(fmt.Sprintf("   - actions: %s\n", backtickJoin(match.Metadata.Actions)))
		}
	}

	return strings.TrimSpace(builder.String())
}

func formatFocusAXElementMarkdown(result FocusAXElementResult) string {
	lines := []string{
		"# AX focus result",
		fmt.Sprintf("- ok: `%t`", result.OK),
		fmt.Sprintf("- ref: `%s`", compactAXRefJSON(result.Ref)),
	}
	if result.Message != "" {
		lines = append(lines, fmt.Sprintf("- message: `%s`", markdownInline(result.Message)))
	}
	return strings.Join(lines, "\n")
}

func formatPerformAXElementActionMarkdown(result PerformAXElementActionResult) string {
	lines := []string{
		"# AX action result",
		fmt.Sprintf("- ok: `%t`", result.OK),
		fmt.Sprintf("- action: `%s`", markdownInline(result.Action)),
		fmt.Sprintf("- ref: `%s`", compactAXRefJSON(result.Ref)),
	}
	if result.Message != "" {
		lines = append(lines, fmt.Sprintf("- message: `%s`", markdownInline(result.Message)))
	}
	return strings.Join(lines, "\n")
}

func formatWindowAccessibilityElementActionMarkdown(result WindowAccessibilityElementActionResult) string {
	lines := []string{
		"# AX snapshot action result",
		fmt.Sprintf("- snapshot id: `%s`", markdownInline(result.SnapshotID)),
		fmt.Sprintf("- element id: `%s`", markdownInline(result.ElementID)),
		fmt.Sprintf("- action: `%s`", markdownInline(result.Action)),
		fmt.Sprintf("- mode: `%s`", markdownInline(result.Mode)),
	}
	if result.ScreenPoint != (Point{}) {
		lines = append(lines, fmt.Sprintf("- screen point: `(%d, %d)`", result.ScreenPoint.X, result.ScreenPoint.Y))
	}
	if result.Message != "" {
		lines = append(lines, fmt.Sprintf("- message: `%s`", markdownInline(result.Message)))
	}
	return strings.Join(lines, "\n")
}

func formatBackgroundMouseResolveMarkdown(result BackgroundMouseResolveResult) string {
	lines := []string{
		"# Background window point resolution",
		fmt.Sprintf("- snapshot id: `%s`", markdownInline(result.SnapshotID)),
		fmt.Sprintf("- requested point: `(%d, %d)`", result.RequestedPoint.X, result.RequestedPoint.Y),
		fmt.Sprintf("- resolved screen point: `(%d, %d)`", result.ScreenPoint.X, result.ScreenPoint.Y),
		fmt.Sprintf("- snapped: `%t`", result.Snapped),
		fmt.Sprintf("- element id: `%s`", markdownInline(result.ElementID)),
	}
	lines = append(lines, formatWindowAccessibilityElementSummaryLines("element", result.Element)...)
	if result.Message != "" {
		lines = append(lines, fmt.Sprintf("- message: `%s`", markdownInline(result.Message)))
	}
	return strings.Join(lines, "\n")
}

func formatBackgroundMouseActionMarkdown(result BackgroundMouseActionResult) string {
	lines := []string{
		"# Background window action result",
		fmt.Sprintf("- snapshot id: `%s`", markdownInline(result.SnapshotID)),
		fmt.Sprintf("- action: `%s`", markdownInline(result.Action)),
		fmt.Sprintf("- element id: `%s`", markdownInline(result.ElementID)),
		fmt.Sprintf("- resolved screen point: `(%d, %d)`", result.ScreenPoint.X, result.ScreenPoint.Y),
		fmt.Sprintf("- snapped: `%t`", result.Snapped),
		fmt.Sprintf("- mode: `%s`", markdownInline(result.Mode)),
	}
	if result.RequestedPoint != nil {
		lines = append(lines, fmt.Sprintf("- requested point: `(%d, %d)`", result.RequestedPoint.X, result.RequestedPoint.Y))
	}
	lines = append(lines, formatWindowAccessibilityElementSummaryLines("element", result.Element)...)
	if result.Message != "" {
		lines = append(lines, fmt.Sprintf("- message: `%s`", markdownInline(result.Message)))
	}
	return strings.Join(lines, "\n")
}

func formatAgentTranslatedPointMarkdown(result AgentTranslatedPointResult) (string, bool) {
	switch typed := result.Result.(type) {
	case UIElementMetadataResult:
		return formatTranslatedElementMetadataMarkdown(result, typed), true
	case *UIElementMetadataResult:
		if typed == nil {
			return "", false
		}
		return formatTranslatedElementMetadataMarkdown(result, *typed), true
	case ActionResult:
		return formatTranslatedAXActionMarkdown(result, typed), true
	case *ActionResult:
		if typed == nil {
			return "", false
		}
		return formatTranslatedAXActionMarkdown(result, *typed), true
	default:
		return "", false
	}
}

func formatTranslatedElementMetadataMarkdown(result AgentTranslatedPointResult, metadata UIElementMetadataResult) string {
	lines := []string{
		"# Element at point",
		fmt.Sprintf("- requested point: `(%d, %d)` in current coordinate space", result.Requested.X, result.Requested.Y),
		fmt.Sprintf("- translated absolute screen point: `(%d, %d)`", result.ScreenPoint.X, result.ScreenPoint.Y),
		fmt.Sprintf("- coordinate space: `%s`", markdownInline(result.CoordinateSpace.Mode)),
	}
	elementLines := strings.Split(formatUIElementMetadataMarkdown("Resolved accessible element", metadata), "\n")
	lines = append(lines, elementLines...)
	return strings.Join(lines, "\n")
}

func formatTranslatedAXActionMarkdown(result AgentTranslatedPointResult, action ActionResult) string {
	lines := []string{
		"# AX point action result",
		fmt.Sprintf("- requested point: `(%d, %d)` in current coordinate space", result.Requested.X, result.Requested.Y),
		fmt.Sprintf("- translated absolute screen point: `(%d, %d)`", result.ScreenPoint.X, result.ScreenPoint.Y),
		fmt.Sprintf("- coordinate space: `%s`", markdownInline(result.CoordinateSpace.Mode)),
		fmt.Sprintf("- ok: `%t`", action.OK),
	}
	if action.Message != "" {
		lines = append(lines, fmt.Sprintf("- message: `%s`", markdownInline(action.Message)))
	}
	return strings.Join(lines, "\n")
}

func searchAXFilters(query SearchAXElementsRequest) []string {
	filters := make([]string, 0, 8)
	if query.Role != "" {
		filters = append(filters, fmt.Sprintf("role `%s`", markdownInline(query.Role)))
	}
	if query.Subrole != "" {
		filters = append(filters, fmt.Sprintf("subrole `%s`", markdownInline(query.Subrole)))
	}
	if query.TitleContains != "" {
		filters = append(filters, fmt.Sprintf("title contains `%s`", markdownInline(query.TitleContains)))
	}
	if query.ValueContains != "" {
		filters = append(filters, fmt.Sprintf("value contains `%s`", markdownInline(query.ValueContains)))
	}
	if query.DescriptionContains != "" {
		filters = append(filters, fmt.Sprintf("description contains `%s`", markdownInline(query.DescriptionContains)))
	}
	if query.Action != "" {
		filters = append(filters, fmt.Sprintf("action `%s`", markdownInline(query.Action)))
	}
	if query.Enabled != nil {
		filters = append(filters, fmt.Sprintf("enabled `%t`", *query.Enabled))
	}
	if query.Focused != nil {
		filters = append(filters, fmt.Sprintf("focused `%t`", *query.Focused))
	}
	return filters
}

func compactAXRefJSON(ref AXElementRefResult) string {
	payload := map[string]any{
		"scope": ref.Scope,
		"path":  ref.Path,
	}
	if ref.WindowHandle != 0 {
		payload["window_handle"] = ref.WindowHandle
	}
	if ref.OwnerPID != 0 {
		payload["owner_pid"] = ref.OwnerPID
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return markdownInline(fmt.Sprintf("%+v", ref))
	}
	return string(data)
}

func formatWindowAccessibilityElementSummaryLines(prefix string, element WindowAccessibilityElement) []string {
	lines := []string{
		fmt.Sprintf("- %s label: `%s`", prefix, markdownInline(firstNonEmpty(element.Title, element.Value, element.SelectedText, element.Role, "unnamed"))),
	}
	if element.Role != "" || element.Subrole != "" {
		lines = append(lines, fmt.Sprintf("- %s role: `%s`  subrole: `%s`", prefix, markdownInline(firstNonEmpty(element.Role, "unknown")), markdownInline(firstNonEmpty(element.Subrole, "none"))))
	}
	if element.ScreenRegion != nil {
		lines = append(lines, fmt.Sprintf("- %s region: `(%d, %d)` `%dx%d`", prefix, element.ScreenRegion.Left, element.ScreenRegion.Top, element.ScreenRegion.Width, element.ScreenRegion.Height))
	}
	if element.ActionPointKnown {
		lines = append(lines, fmt.Sprintf("- %s action point: `(%d, %d)`", prefix, element.ActionPoint.X, element.ActionPoint.Y))
	}
	if len(element.AvailableActions) > 0 {
		lines = append(lines, fmt.Sprintf("- %s available actions: %s", prefix, backtickJoin(element.AvailableActions)))
	}
	return lines
}
