package backend

import (
	"errors"
	"image"
	"image/png"
	"os"
	"runtime"
	"strings"
	"testing"

	"gut/native/common"
	"gut/shared"
	guttesting "gut/testing"
)

func TestParseButton(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{input: "left", ok: true},
		{input: "middle", ok: true},
		{input: "right", ok: true},
		{input: "other", ok: false},
	}

	for _, test := range tests {
		_, err := parseButton(test.input)
		if test.ok && err != nil {
			t.Fatalf("parseButton(%q) returned error: %v", test.input, err)
		}
		if !test.ok && err == nil {
			t.Fatalf("parseButton(%q) expected error", test.input)
		}
	}
}

func TestParseKey(t *testing.T) {
	tests := []string{"a", "Z", "1", "enter", "ctrl", "leftwin", "f12"}
	for _, input := range tests {
		if _, err := parseKey(input); err != nil {
			t.Fatalf("parseKey(%q) returned error: %v", input, err)
		}
	}

	if _, err := parseKey("not-a-key"); err == nil {
		t.Fatal("parseKey should reject unknown keys")
	}
}

func TestParseKeyMetaAliasesFollowPlatform(t *testing.T) {
	got, err := parseKey("command")
	if err != nil {
		t.Fatalf("parseKey(%q) returned error: %v", "command", err)
	}

	want := shared.KeyLeftWin
	if runtime.GOOS == "darwin" {
		want = shared.KeyLeftCmd
	}
	if got != want {
		t.Fatalf("parseKey(%q) expected %v, got %v", "command", want, got)
	}
}

func TestBuildWindowQuery(t *testing.T) {
	query, err := buildWindowQuery(WindowQueryRequest{Title: "Calculator"})
	if err != nil {
		t.Fatalf("buildWindowQuery exact returned error: %v", err)
	}
	if !query.By.Title.Match("Calculator") {
		t.Fatal("exact title matcher did not match")
	}

	query, err = buildWindowQuery(WindowQueryRequest{Title: "^Calc", UseRegex: true})
	if err != nil {
		t.Fatalf("buildWindowQuery regex returned error: %v", err)
	}
	if !query.By.Title.Match("Calculator") {
		t.Fatal("regex title matcher did not match")
	}
}

func TestScreencaptureArgs(t *testing.T) {
	full := screencaptureArgs("", "/tmp/full.png")
	if len(full) != 2 || full[0] != "-x" || full[1] != "/tmp/full.png" {
		t.Fatalf("unexpected full-screen args: %#v", full)
	}

	region := screencaptureArgs("10,20,30,40", "/tmp/region.png")
	if len(region) != 4 || region[0] != "-x" || region[1] != "-R" || region[2] != "10,20,30,40" || region[3] != "/tmp/region.png" {
		t.Fatalf("unexpected region args: %#v", region)
	}
}

func TestRequiredCapabilitiesForGOOS(t *testing.T) {
	darwin := requiredCapabilitiesForGOOS("darwin")
	for _, capability := range []common.Capability{
		common.CapabilityKeyboardTap,
		common.CapabilityKeyboardToggle,
		common.CapabilityScreenCapture,
		common.CapabilityScreenHighlight,
		common.CapabilityWindowMinimize,
		common.CapabilityWindowRestore,
	} {
		if hasCapability(darwin, capability) {
			t.Fatalf("expected %s to be excluded from darwin required capabilities", capability)
		}
	}
	for _, capability := range []common.Capability{
		common.CapabilityKeyboardType,
		common.CapabilityScreenSize,
		common.CapabilityWindowList,
	} {
		if !hasCapability(darwin, capability) {
			t.Fatalf("expected %s to remain required on darwin", capability)
		}
	}

	linux := requiredCapabilitiesForGOOS("linux")
	for _, capability := range []common.Capability{
		common.CapabilityKeyboardTap,
		common.CapabilityKeyboardToggle,
		common.CapabilityScreenCapture,
		common.CapabilityScreenHighlight,
	} {
		if !hasCapability(linux, capability) {
			t.Fatalf("expected %s to remain required off darwin", capability)
		}
	}
	for _, capability := range []common.Capability{
		common.CapabilityWindowMinimize,
		common.CapabilityWindowRestore,
	} {
		if hasCapability(linux, capability) {
			t.Fatalf("expected %s to remain non-blocking off darwin", capability)
		}
	}
}

func TestFeatureStatusesForGOOSDarwin(t *testing.T) {
	statuses := featureStatusesForGOOS("darwin", guttesting.Report{
		Capabilities: common.NewCapabilitySet(
			common.CapabilityStatus{Capability: common.CapabilityScreenCapture, Availability: common.AvailabilityUnsupported, Reason: "native fallback hidden"},
			common.CapabilityStatus{Capability: common.CapabilityWindowMinimize, Availability: common.AvailabilityUnsupported, Reason: "minimize unsupported"},
			common.CapabilityStatus{Capability: common.CapabilityWindowRestore, Availability: common.AvailabilityUnsupported, Reason: "restore unsupported"},
		),
	})

	captureScreen := requireFeatureStatus(t, statuses, "capture_screen")
	if captureScreen.Availability != string(common.AvailabilityAvailable) || !strings.Contains(captureScreen.Reason, "safe") {
		t.Fatalf("expected capture_screen to report macOS fallback availability, got %+v", captureScreen)
	}
	pressSpecialKey := requireFeatureStatus(t, statuses, "press_special_key")
	if pressSpecialKey.Availability != string(common.AvailabilityAvailable) {
		t.Fatalf("expected press_special_key to be available on darwin, got %+v", pressSpecialKey)
	}
	for _, id := range []string{"tap_keys", "press_keys", "release_keys", "highlight_region"} {
		status := requireFeatureStatus(t, statuses, id)
		if status.Availability != string(common.AvailabilityUnavailable) || !strings.Contains(status.Reason, "intentionally hidden on macOS 26+") {
			t.Fatalf("expected %s to be hidden on darwin, got %+v", id, status)
		}
	}
	minimize := requireFeatureStatus(t, statuses, "minimize_window")
	if minimize.Availability != string(common.AvailabilityUnavailable) || !strings.Contains(minimize.Reason, "window.minimize") {
		t.Fatalf("expected minimize_window to surface the native unsupported state, got %+v", minimize)
	}
	nativeCapture := requireFeatureStatus(t, statuses, string(common.CapabilityScreenCapture))
	if nativeCapture.Availability != string(common.AvailabilityUnsupported) {
		t.Fatalf("expected native screen.capture capability status to remain visible, got %+v", nativeCapture)
	}
}

func TestFeatureStatusesForGOOSNonDarwin(t *testing.T) {
	statuses := featureStatusesForGOOS("linux", guttesting.Report{
		Capabilities: common.NewCapabilitySet(
			common.CapabilityStatus{Capability: common.CapabilityScreenCapture, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityKeyboardTap, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityKeyboardToggle, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityScreenHighlight, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityWindowMinimize, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityWindowRestore, Availability: common.AvailabilityAvailable},
		),
	})

	for _, id := range []string{"capture_screen", "capture_region", "tap_keys", "press_keys", "release_keys", "highlight_region", "minimize_window", "restore_window"} {
		status := requireFeatureStatus(t, statuses, id)
		if status.Availability != string(common.AvailabilityAvailable) {
			t.Fatalf("expected %s to be available off darwin, got %+v", id, status)
		}
	}
	pressSpecialKey := requireFeatureStatus(t, statuses, "press_special_key")
	if pressSpecialKey.Availability != string(common.AvailabilityUnavailable) || !strings.Contains(pressSpecialKey.Reason, "only exposed on darwin") {
		t.Fatalf("expected press_special_key to be unavailable off darwin, got %+v", pressSpecialKey)
	}
}

func TestCaptureResultIncludesOffset(t *testing.T) {
	result := CaptureResult{
		Path:    "/tmp/region.png",
		Message: "Saved capture to /tmp/region.png",
		Offset:  Point{X: 2860, Y: 1510},
		Scale:   Scale{X: 2, Y: 2},
	}
	if result.Offset.X != 2860 || result.Offset.Y != 1510 {
		t.Fatalf("unexpected offset: %#v", result.Offset)
	}
	if result.Scale.X != 2 || result.Scale.Y != 2 {
		t.Fatalf("unexpected scale: %#v", result.Scale)
	}
}

func TestCaptureScaleUsesImageDimensions(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/capture.png"

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp image: %v", err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 400, 200))); err != nil {
		file.Close()
		t.Fatalf("encode temp image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp image: %v", err)
	}

	scale := captureScale(path, Size{Width: 200, Height: 100})
	if scale.X != 2 || scale.Y != 2 {
		t.Fatalf("unexpected capture scale: %#v", scale)
	}
}

func TestCaptureMetadataRoundTrip(t *testing.T) {
	path := t.TempDir() + "/capture.png"
	want := CaptureResult{
		Path:    path,
		Message: "Saved capture",
		Offset:  Point{X: 120, Y: 340},
		Scale:   Scale{X: 2, Y: 2},
	}

	if err := writeCaptureMetadata(want); err != nil {
		t.Fatalf("write capture metadata: %v", err)
	}

	got, err := readCaptureMetadata(path)
	if err != nil {
		t.Fatalf("read capture metadata: %v", err)
	}

	if got.Path != want.Path || got.Offset != want.Offset || got.Scale != want.Scale {
		t.Fatalf("unexpected capture metadata: got %#v want %#v", got, want)
	}
}

func TestDarwinSpecialKeyHelpers(t *testing.T) {
	keyCode, ok := darwinSpecialKeyCode("enter")
	if !ok || keyCode != 36 {
		t.Fatalf("expected enter key code 36, got code=%d ok=%v", keyCode, ok)
	}
	keyCode, ok = darwinSpecialKeyCode("tab")
	if !ok || keyCode != 48 {
		t.Fatalf("expected tab key code 48, got code=%d ok=%v", keyCode, ok)
	}
	if _, ok := darwinSpecialKeyCode("bogus"); ok {
		t.Fatal("expected bogus special key to be rejected")
	}

	script := darwinSpecialKeyScript(36, 2)
	if script == "" || !strings.Contains(script, "repeat 2 times") || !strings.Contains(script, "key code 36") {
		t.Fatalf("unexpected special key script: %q", script)
	}
}

func TestHighlightRegionUnavailableOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-specific behavior")
	}

	service := NewService()
	_, err := service.HighlightRegion(Region{Left: 1, Top: 2, Width: 3, Height: 4})
	if !errors.Is(err, common.ErrCapabilityUnavailable) {
		t.Fatalf("expected ErrCapabilityUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "intentionally hidden on macOS 26+") {
		t.Fatalf("expected darwin highlight error to mention the macOS 26+ contract, got %v", err)
	}
}

func hasCapability(capabilities []common.Capability, target common.Capability) bool {
	for _, capability := range capabilities {
		if capability == target {
			return true
		}
	}
	return false
}

func requireFeatureStatus(t *testing.T, statuses []FeatureStatus, id string) FeatureStatus {
	t.Helper()

	for _, status := range statuses {
		if status.ID == id {
			return status
		}
	}
	t.Fatalf("missing feature status %q in %#v", id, statuses)
	return FeatureStatus{}
}
