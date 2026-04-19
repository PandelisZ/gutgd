package backend

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/PandelisZ/gut/shared"
)

var (
	currentRuntimeGOOS   = runtime.GOOS
	runDarwinAppleScript = func(ctx context.Context, script string) (string, error) {
		cmd := exec.CommandContext(ctx, "osascript", "-e", script)
		output, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(output)), err
	}
)

func shouldUseDarwinShortcutPath(keys []string) bool {
	if currentRuntimeGOOS != "darwin" || len(keys) == 0 {
		return false
	}

	nonModifiers := 0
	for _, item := range keys {
		key, err := parseKey(item)
		if err != nil {
			return false
		}
		if !isModifierKey(key) {
			nonModifiers++
		}
	}
	return nonModifiers == 1
}

func isModifierKey(key shared.Key) bool {
	switch key {
	case shared.KeyLeftShift, shared.KeyRightShift,
		shared.KeyLeftControl, shared.KeyRightControl,
		shared.KeyLeftAlt, shared.KeyRightAlt,
		shared.KeyLeftSuper, shared.KeyRightSuper,
		shared.KeyLeftWin, shared.KeyRightWin,
		shared.KeyLeftCmd, shared.KeyRightCmd,
		shared.KeyFn:
		return true
	default:
		return false
	}
}

func executeDarwinAppleScript(ctx context.Context, script string) error {
	output, err := runDarwinAppleScript(ctx, script)
	if err == nil {
		return nil
	}
	if output == "" {
		return err
	}
	return fmt.Errorf("%v: %s", err, output)
}
