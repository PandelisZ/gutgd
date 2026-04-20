package backend

import (
	"strings"
	"testing"
)

func TestMarshalToolPayloadUsesSnapshotMarkdownForAccessibilitySnapshot(t *testing.T) {
	payload := marshalToolPayload(WindowAccessibilitySnapshotResult{
		SnapshotID: "axwin-1",
		Markdown:   "# Window accessibility snapshot\n\n- Snapshot ID: `axwin-1`\n- Element count: `2`",
		Message:    "Captured 2 accessible elements.",
	})

	if !strings.Contains(payload, "# Window accessibility snapshot") {
		t.Fatalf("expected snapshot payload to prefer markdown, got %q", payload)
	}
	if strings.Contains(payload, `"elements"`) || strings.Contains(payload, `"snapshot_id"`) {
		t.Fatalf("expected snapshot payload to avoid raw JSON blob fields, got %q", payload)
	}
}

func TestMarshalToolPayloadFormatsSearchAXElementsAsMarkdown(t *testing.T) {
	payload := marshalToolPayload(SearchAXElementsResult{
		Query: SearchAXElementsRequest{
			Scope:        "window_handle",
			WindowHandle: 7,
			Role:         "AXButton",
			Limit:        5,
			MaxDepth:     3,
		},
		Matches: []AXElementMatchResult{
			{
				Ref: AXElementRefResult{
					Scope:        "window_handle",
					WindowHandle: 7,
					Path:         []int{0, 1, 2},
				},
				Metadata: UIElementMetadataResult{
					Role:       "AXButton",
					Title:      "Send",
					Enabled:    true,
					Focused:    false,
					FrameKnown: true,
					Frame:      Region{Left: 420, Top: 180, Width: 90, Height: 24},
					Actions:    []string{"AXPress"},
				},
				Depth:            2,
				ActionPointKnown: true,
				ActionPoint:      Point{X: 465, Y: 192},
			},
		},
		Message: "Found 1 AX element matches.",
	})

	for _, needle := range []string{
		"# AX search results",
		"Found 1 AX element matches.",
		"window handle: `7`",
		"role `AXButton`",
		"`Send`",
		`"path":[0,1,2]`,
		"action point: `(465, 192)`",
	} {
		if !strings.Contains(payload, needle) {
			t.Fatalf("expected search payload to contain %q, got %q", needle, payload)
		}
	}
	if strings.Contains(payload, `"matches"`) || strings.Contains(payload, `"query"`) {
		t.Fatalf("expected search payload to avoid raw JSON blob fields, got %q", payload)
	}
}

func TestMarshalToolPayloadFormatsTranslatedElementMetadataAsMarkdown(t *testing.T) {
	payload := marshalToolPayload(AgentTranslatedPointResult{
		CoordinateSpace: AgentCoordinateSpace{Mode: "window"},
		Requested:       Point{X: 160, Y: 60},
		ScreenPoint:     Point{X: 460, Y: 180},
		Result: UIElementMetadataResult{
			Role:       "AXButton",
			Title:      "Send",
			Enabled:    true,
			Focused:    false,
			FrameKnown: true,
			Frame:      Region{Left: 420, Top: 180, Width: 90, Height: 24},
			Actions:    []string{"AXPress"},
		},
	})

	for _, needle := range []string{
		"# Element at point",
		"requested point: `(160, 60)`",
		"translated absolute screen point: `(460, 180)`",
		"# Resolved accessible element",
		"role: `AXButton`",
		"actions: `AXPress`",
	} {
		if !strings.Contains(payload, needle) {
			t.Fatalf("expected translated point payload to contain %q, got %q", needle, payload)
		}
	}
	if strings.Contains(payload, `"screen_point"`) || strings.Contains(payload, `"result"`) {
		t.Fatalf("expected translated point payload to avoid raw JSON blob fields, got %q", payload)
	}
}
