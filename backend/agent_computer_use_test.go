package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentToolParamsIncludeBuiltInComputerTool(t *testing.T) {
	service := NewService()
	params := service.agentToolParams(service.agentToolsForGOOS("linux"))
	payload := marshalJSON(t, params)

	if !strings.Contains(payload, `"type":"computer"`) {
		t.Fatalf("expected tool params to include the built-in computer tool, got %s", payload)
	}
}

func TestAgentComputerCallOutputUsesComputerCallOutputAndOriginalDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "computer-capture.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	item, err := agentComputerCallOutput("call_computer_1", CaptureResult{
		Path:          path,
		Message:       "Saved capture",
		Offset:        Point{X: 0, Y: 0},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 1200, Height: 900},
		DeliveredSize: Size{Width: 1200, Height: 900},
		OriginalScale: Scale{X: 1, Y: 1},
	})
	if err != nil {
		t.Fatalf("agentComputerCallOutput returned error: %v", err)
	}

	payload := marshalJSON(t, item)
	for _, needle := range []string{
		`"type":"computer_call_output"`,
		`"call_id":"call_computer_1"`,
		`"type":"computer_screenshot"`,
		`"detail":"original"`,
		`"image_url":"data:image/png;base64,`,
	} {
		if !strings.Contains(payload, needle) {
			t.Fatalf("expected computer call output payload to contain %q, got %s", needle, payload)
		}
	}
}

func TestAgentComputerActionsFallsBackToLegacySingleAction(t *testing.T) {
	item := mustResponseOutputItem(t, `{
		"type":"computer_call",
		"id":"cc_legacy",
		"call_id":"call_legacy",
		"pending_safety_checks":[],
		"status":"completed",
		"action":{"type":"click","button":"left","x":42,"y":84,"keys":[]}
	}`)

	actions := agentComputerActions(item.AsComputerCall())
	if len(actions) != 1 {
		t.Fatalf("expected exactly one legacy computer action, got %+v", actions)
	}
	if actions[0].Type != "click" || actions[0].Button != "left" || actions[0].X != 42 || actions[0].Y != 84 {
		t.Fatalf("unexpected legacy computer action: %+v", actions[0])
	}
}

func TestAgentDeveloperPromptMentionsBuiltInComputerTool(t *testing.T) {
	prompt := agentDeveloperPromptForGOOS("darwin")
	if !strings.Contains(prompt, "built-in computer tool") {
		t.Fatalf("expected agent prompt to mention the built-in computer tool, got %q", prompt)
	}
}

func TestTranscriptItemsFromResponseOutputSkipsComputerCalls(t *testing.T) {
	item := mustResponseOutputItem(t, `{
		"type":"computer_call",
		"id":"cc_1",
		"call_id":"call_1",
		"pending_safety_checks":[],
		"status":"completed",
		"actions":[{"type":"screenshot"}]
	}`)

	if got := transcriptItemsFromResponseOutput(item); len(got) != 0 {
		t.Fatalf("expected computer calls to be handled only by the explicit computer-call path, got %+v", got)
	}
}

func TestAgentComputerStatePersistsByResponseID(t *testing.T) {
	service := NewService()
	state := &agentComputerState{
		LastCapture: &CaptureResult{
			Path:          "/tmp/capture.png",
			Message:       "Saved capture",
			Offset:        Point{X: 40, Y: 80},
			Scale:         Scale{X: 1, Y: 1},
			OriginalSize:  Size{Width: 800, Height: 600},
			DeliveredSize: Size{Width: 800, Height: 600},
			OriginalScale: Scale{X: 1, Y: 1},
		},
	}

	service.saveAgentComputerState("resp_computer", state)
	got := service.loadAgentComputerState("resp_computer")
	if got.LastCapture == nil {
		t.Fatal("expected persisted computer state to include a capture")
	}
	if got.LastCapture.Path != "/tmp/capture.png" || got.LastCapture.Offset.X != 40 || got.LastCapture.Offset.Y != 80 {
		t.Fatalf("unexpected persisted computer state: %+v", got.LastCapture)
	}

	got.LastCapture.Path = "mutated"
	reloaded := service.loadAgentComputerState("resp_computer")
	if reloaded.LastCapture == nil || reloaded.LastCapture.Path != "/tmp/capture.png" {
		t.Fatalf("expected computer state to be copied defensively, got %+v", reloaded.LastCapture)
	}
}

func TestComputerCaptureRequestUsesLogicalScreenSizeBounds(t *testing.T) {
	req := computerCaptureRequest(Size{Width: 1800, Height: 1169})
	if req.MaxImageWidth != 1800 || req.MaxImageHeight != 1169 {
		t.Fatalf("expected computer capture request to preserve logical screen size, got %+v", req)
	}
}

func TestTranslateDeliveredImagePointToScreenPreservesLogicalCoordinatesForRetinaDownscaledCapture(t *testing.T) {
	result, err := translateDeliveredImagePointToScreen(CaptureResult{
		Path:          "/tmp/capture.png",
		Offset:        Point{X: 0, Y: 0},
		OriginalSize:  Size{Width: 3600, Height: 2338},
		DeliveredSize: Size{Width: 1800, Height: 1169},
		OriginalScale: Scale{X: 2, Y: 2},
	}, Point{X: 822, Y: 532})
	if err != nil {
		t.Fatalf("translateDeliveredImagePointToScreen returned error: %v", err)
	}
	if result.AbsoluteScreenPoint != (Point{X: 822, Y: 532}) {
		t.Fatalf("expected logical coordinates to survive retina translation, got %+v", result.AbsoluteScreenPoint)
	}
}

func TestRunAgentComputerCallReturnsStableSafetyAckCode(t *testing.T) {
	service := NewService()
	_, event, output, err := service.runAgentComputerCall(
		mustResponseOutputItem(t, `{
			"type":"computer_call",
			"id":"cc_safety",
			"call_id":"call_safety",
			"pending_safety_checks":[{"id":"psc_1","code":"confirm_navigation","message":"Needs approval"}],
			"status":"completed",
			"actions":[{"type":"screenshot"}]
		}`).AsComputerCall(),
		&agentComputerState{},
	)
	if err == nil {
		t.Fatal("expected pending safety checks to stop the computer loop")
	}
	if !strings.Contains(output, agentComputerSafetyAckCode) {
		t.Fatalf("expected output to contain stable safety code %q, got %s", agentComputerSafetyAckCode, output)
	}
	if !strings.Contains(event.Output, `"pending_safety_checks"`) {
		t.Fatalf("expected event output to preserve pending safety check payload, got %s", event.Output)
	}
}
