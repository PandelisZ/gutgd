package backend

import (
	"context"
	"strings"
	"testing"
)

func TestTapKeysUsesSafeShortcutPathOnDarwin(t *testing.T) {
	previousGOOS := currentRuntimeGOOS
	previousRunner := runDarwinAppleScript
	currentRuntimeGOOS = "darwin"
	t.Cleanup(func() {
		currentRuntimeGOOS = previousGOOS
		runDarwinAppleScript = previousRunner
	})

	var script string
	runDarwinAppleScript = func(_ context.Context, nextScript string) (string, error) {
		script = nextScript
		return "", nil
	}

	service := &Service{}
	result, err := service.TapKeys(KeyboardKeysRequest{Keys: []string{"cmd", "space"}})
	if err != nil {
		t.Fatalf("TapKeys returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK result, got %+v", result)
	}
	if !strings.Contains(script, "command down") || !strings.Contains(script, "key code 49") {
		t.Fatalf("expected safe AppleScript shortcut, got %q", script)
	}
}
