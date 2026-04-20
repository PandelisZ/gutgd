package backend

import (
	"slices"
	"strings"
)

const strictBackgroundOnlyInstructions = `Strict background-only mouse mode is enabled.

- Raw pointer tools are intentionally unavailable. Do not try to move, click, drag, or scroll the real system pointer.
- Prefer get_window_accessibility_snapshot, act_on_window_accessibility_element, search_ax_elements, focus_ax_element, and perform_ax_element_action for UI interaction.
- When act_on_window_accessibility_element or background-safe actions report unsupported_background_action, stop and tell the user instead of falling back to focused raw input.`

var strictBackgroundOnlyBlockedTools = []string{
	"get_mouse_position",
	"set_mouse_position",
	"move_mouse_line",
	"click_mouse",
	"mouse_down",
	"mouse_up",
	"double_click_mouse",
	"scroll_mouse",
	"drag_mouse",
}

func combineAgentInstructions(settings AgentSettings) string {
	instructions := combineInstructions(settings.SystemPrompt)
	if !settings.StrictBackgroundOnly {
		return instructions
	}
	return instructions + "\n\n" + strictBackgroundOnlyInstructions
}

func filterAgentToolsForSettings(tools []agentTool, settings AgentSettings) []agentTool {
	if !settings.StrictBackgroundOnly {
		return tools
	}
	return slices.DeleteFunc(tools, func(tool agentTool) bool {
		return slices.Contains(strictBackgroundOnlyBlockedTools, strings.TrimSpace(tool.Name))
	})
}

func windowAccessibilityActionToolDescription(strictBackgroundOnly bool) string {
	if strictBackgroundOnly {
		return "Act on one element returned by get_window_accessibility_snapshot using its stable element ID. In strict background-only mouse mode this uses only cached background-safe AX actions and fails closed instead of focusing a window or falling back to raw cursor input."
	}
	return "Act on one element returned by get_window_accessibility_snapshot using its stable element ID. This prefers cached AX refs for background-safe actions and falls back to focused raw input only when needed, so the model does not have to guess coordinates again."
}
