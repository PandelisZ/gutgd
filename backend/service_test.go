package backend

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/shared"
	guttesting "github.com/PandelisZ/gut/testing"
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
			common.CapabilityStatus{Capability: common.CapabilityPermissionReadiness, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedWindowMetadata, Availability: common.AvailabilityPermissionBlocked, Reason: "Accessibility permission has not been granted"},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedElementMetadata, Availability: common.AvailabilityUnsupported, Reason: "provider unavailable"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementAtPointMetadata, Availability: common.AvailabilityUnavailable, Reason: "temporarily disabled"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementSearch, Availability: common.AvailabilityPermissionBlocked, Reason: "Accessibility permission has not been granted"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementFocusMatch, Availability: common.AvailabilityUnsupported, Reason: "provider unavailable"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementActionMatch, Availability: common.AvailabilityUnavailable, Reason: "temporarily disabled"},
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
	for _, id := range []string{"capture_active_window", "capture_window", "tap_keys", "press_keys", "release_keys"} {
		status := requireFeatureStatus(t, statuses, id)
		if status.Availability != string(common.AvailabilityAvailable) {
			t.Fatalf("expected %s to be available on darwin, got %+v", id, status)
		}
	}
	highlightRegion := requireFeatureStatus(t, statuses, "highlight_region")
	if highlightRegion.Availability != string(common.AvailabilityUnavailable) || !strings.Contains(highlightRegion.Reason, "intentionally hidden on macOS 26+") {
		t.Fatalf("expected highlight_region to stay hidden on darwin, got %+v", highlightRegion)
	}
	for _, test := range []struct {
		id           string
		availability common.Availability
	}{
		{id: "get_permission_readiness", availability: common.AvailabilityAvailable},
		{id: "get_focused_window_metadata", availability: common.AvailabilityPermissionBlocked},
		{id: "get_focused_element_metadata", availability: common.AvailabilityUnsupported},
		{id: "get_element_at_point_metadata", availability: common.AvailabilityUnavailable},
		{id: "search_ax_elements", availability: common.AvailabilityPermissionBlocked},
		{id: "focus_ax_element", availability: common.AvailabilityUnsupported},
		{id: "perform_ax_element_action", availability: common.AvailabilityUnavailable},
	} {
		status := requireFeatureStatus(t, statuses, test.id)
		if status.Availability != string(test.availability) {
			t.Fatalf("expected %s to preserve %s, got %+v", test.id, test.availability, status)
		}
	}
	minimize := requireFeatureStatus(t, statuses, "minimize_window")
	if minimize.Availability != string(common.AvailabilityUnsupported) || !strings.Contains(minimize.Reason, "minimize unsupported") {
		t.Fatalf("expected minimize_window to surface the native unsupported state, got %+v", minimize)
	}
	nativeFocusedWindow := requireFeatureStatus(t, statuses, string(common.CapabilityAXFocusedWindowMetadata))
	if nativeFocusedWindow.Availability != string(common.AvailabilityPermissionBlocked) {
		t.Fatalf("expected native focused-window capability status to remain visible, got %+v", nativeFocusedWindow)
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
			common.CapabilityStatus{Capability: common.CapabilityPermissionReadiness, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedWindowMetadata, Availability: common.AvailabilityPermissionBlocked, Reason: "Accessibility permission has not been granted"},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedElementMetadata, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementAtPointMetadata, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedWindowRaise, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedElementAction, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementActionAtPoint, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementFocusAtPoint, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementSearch, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementFocusMatch, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementActionMatch, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityWindowMinimize, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityWindowRestore, Availability: common.AvailabilityAvailable},
		),
	})

	for _, id := range []string{"capture_screen", "capture_region", "tap_keys", "press_keys", "release_keys", "highlight_region", "get_permission_readiness", "get_focused_element_metadata", "get_element_at_point_metadata", "raise_focused_window", "perform_focused_element_action", "perform_element_action_at_point", "focus_element_at_point", "search_ax_elements", "focus_ax_element", "perform_ax_element_action", "minimize_window", "restore_window"} {
		status := requireFeatureStatus(t, statuses, id)
		if status.Availability != string(common.AvailabilityAvailable) {
			t.Fatalf("expected %s to be available off darwin, got %+v", id, status)
		}
	}
	focusedWindow := requireFeatureStatus(t, statuses, "get_focused_window_metadata")
	if focusedWindow.Availability != string(common.AvailabilityPermissionBlocked) {
		t.Fatalf("expected get_focused_window_metadata to preserve permission_blocked, got %+v", focusedWindow)
	}
	pressSpecialKey := requireFeatureStatus(t, statuses, "press_special_key")
	if pressSpecialKey.Availability != string(common.AvailabilityUnavailable) || !strings.Contains(pressSpecialKey.Reason, "only exposed on darwin") {
		t.Fatalf("expected press_special_key to be unavailable off darwin, got %+v", pressSpecialKey)
	}
}

func TestGetPermissionReadinessPreservesCapabilityAvailability(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{
		permissionSnapshot: common.PermissionSnapshot{
			Accessibility:   common.PermissionStatus{Granted: false, Supported: true, Reason: "Accessibility permission has not been granted"},
			ScreenRecording: common.PermissionStatus{Granted: true, Supported: true, Reason: "already granted"},
		},
		capabilities: common.NewCapabilitySet(
			common.CapabilityStatus{Capability: common.CapabilityPermissionReadiness, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedWindowMetadata, Availability: common.AvailabilityPermissionBlocked, Reason: "Accessibility permission has not been granted"},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedElementMetadata, Availability: common.AvailabilityUnsupported, Reason: "provider unavailable"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementAtPointMetadata, Availability: common.AvailabilityUnavailable, Reason: "temporarily disabled"},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedWindowRaise, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXFocusedElementAction, Availability: common.AvailabilityPermissionBlocked, Reason: "Accessibility permission has not been granted"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementActionAtPoint, Availability: common.AvailabilityUnsupported, Reason: "provider unavailable"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementFocusAtPoint, Availability: common.AvailabilityUnavailable, Reason: "temporarily disabled"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementSearch, Availability: common.AvailabilityAvailable},
			common.CapabilityStatus{Capability: common.CapabilityAXElementFocusMatch, Availability: common.AvailabilityPermissionBlocked, Reason: "Accessibility permission has not been granted"},
			common.CapabilityStatus{Capability: common.CapabilityAXElementActionMatch, Availability: common.AvailabilityUnsupported, Reason: "provider unavailable"},
		),
	}
	service := newTestServiceWithWindowsAndAccessibility(accessibility)

	result, err := service.GetPermissionReadiness()
	if err != nil {
		t.Fatalf("GetPermissionReadiness returned error: %v", err)
	}
	if !strings.Contains(result.Message, "AX metadata and action tools") {
		t.Fatalf("expected readiness message to mention AX metadata and action tools, got %q", result.Message)
	}
	if result.Permissions.Accessibility.Granted || !result.Permissions.Accessibility.Supported {
		t.Fatalf("unexpected accessibility permission snapshot: %+v", result.Permissions.Accessibility)
	}
	for _, test := range []struct {
		id           string
		availability common.Availability
	}{
		{id: string(common.CapabilityPermissionReadiness), availability: common.AvailabilityAvailable},
		{id: string(common.CapabilityAXFocusedWindowMetadata), availability: common.AvailabilityPermissionBlocked},
		{id: string(common.CapabilityAXFocusedElementMetadata), availability: common.AvailabilityUnsupported},
		{id: string(common.CapabilityAXElementAtPointMetadata), availability: common.AvailabilityUnavailable},
		{id: string(common.CapabilityAXElementSearch), availability: common.AvailabilityAvailable},
		{id: string(common.CapabilityAXElementFocusMatch), availability: common.AvailabilityPermissionBlocked},
		{id: string(common.CapabilityAXElementActionMatch), availability: common.AvailabilityUnsupported},
	} {
		status := requireFeatureStatus(t, result.Capabilities, test.id)
		if status.Availability != string(test.availability) {
			t.Fatalf("expected %s to preserve %s, got %+v", test.id, test.availability, status)
		}
	}
}

func TestParseAXAction(t *testing.T) {
	for _, action := range []string{"AXPress", "AXRaise", "AXShowMenu", "AXConfirm", "AXPick"} {
		got, err := parseAXAction(action)
		if err != nil {
			t.Fatalf("parseAXAction(%q) returned error: %v", action, err)
		}
		if got != common.AXAction(action) {
			t.Fatalf("parseAXAction(%q) = %q", action, got)
		}
	}

	if _, err := parseAXAction("AXDefinitelyUnsupportedSyntheticAction"); err == nil {
		t.Fatal("expected invalid AX action to be rejected")
	}
}

func TestAccessibilityActionMethods(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{}
	service := newTestServiceWithWindowsAndAccessibility(accessibility)

	raiseResult, err := service.RaiseFocusedWindow()
	if err != nil {
		t.Fatalf("RaiseFocusedWindow returned error: %v", err)
	}
	if !raiseResult.OK || accessibility.raiseFocusedWindowCalls != 1 {
		t.Fatalf("unexpected raise-focused-window result: %+v calls=%d", raiseResult, accessibility.raiseFocusedWindowCalls)
	}

	focusedResult, err := service.PerformFocusedElementAction(AXActionRequest{Action: "AXConfirm"})
	if err != nil {
		t.Fatalf("PerformFocusedElementAction returned error: %v", err)
	}
	if !focusedResult.OK || accessibility.lastFocusedElementAction != common.AXConfirm {
		t.Fatalf("unexpected focused-element action result: %+v action=%q", focusedResult, accessibility.lastFocusedElementAction)
	}

	pointActionResult, err := service.PerformElementActionAtPoint(AXActionAtPointRequest{X: 40, Y: 50, Action: "AXShowMenu"})
	if err != nil {
		t.Fatalf("PerformElementActionAtPoint returned error: %v", err)
	}
	if !pointActionResult.OK || accessibility.lastElementActionAtPoint != (shared.Point{X: 40, Y: 50}) || accessibility.lastElementActionAtPointAction != common.AXShowMenu {
		t.Fatalf("unexpected element-at-point action result: %+v point=%+v action=%q", pointActionResult, accessibility.lastElementActionAtPoint, accessibility.lastElementActionAtPointAction)
	}

	focusResult, err := service.FocusElementAtPoint(PointRequest{X: 60, Y: 70})
	if err != nil {
		t.Fatalf("FocusElementAtPoint returned error: %v", err)
	}
	if !focusResult.OK || accessibility.lastFocusElementAtPoint != (shared.Point{X: 60, Y: 70}) {
		t.Fatalf("unexpected focus-element-at-point result: %+v point=%+v", focusResult, accessibility.lastFocusElementAtPoint)
	}
}

func TestAccessibilityActionMethodsRejectInvalidAction(t *testing.T) {
	service := newTestServiceWithWindowsAndAccessibility(&fakeBackendAccessibilityProvider{})

	if _, err := service.PerformFocusedElementAction(AXActionRequest{Action: "AXDefinitelyUnsupportedSyntheticAction"}); err == nil {
		t.Fatal("expected focused-element action validation error")
	}
	if _, err := service.PerformElementActionAtPoint(AXActionAtPointRequest{X: 1, Y: 2, Action: "AXDefinitelyUnsupportedSyntheticAction"}); err == nil {
		t.Fatal("expected element-at-point action validation error")
	}
}

func TestSearchAXElementMethods(t *testing.T) {
	enabled := true
	focused := false
	accessibility := &fakeBackendAccessibilityProvider{
		searchAXElementsMatches: []common.AXElementMatch{{
			Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 42, Path: []int{1, 2}},
			Metadata: common.UIElementMetadata{
				Role:       "AXButton",
				Title:      "Save",
				Enabled:    true,
				Frame:      common.Rect{X: 12, Y: 34, Width: 56, Height: 20},
				FrameKnown: true,
				Actions:    []string{"AXPress"},
			},
			Depth:            2,
			ActionPoint:      common.Point{X: 20, Y: 40},
			ActionPointKnown: true,
		}},
	}
	service := newTestServiceWithWindowsAndAccessibility(accessibility)

	searchResult, err := service.SearchAXElements(SearchAXElementsRequest{
		Scope:               string(common.AXSearchScopeWindowHandle),
		WindowHandle:        42,
		Role:                "AXButton",
		Subrole:             "",
		TitleContains:       "Save",
		ValueContains:       "",
		DescriptionContains: "",
		Action:              string(common.AXPress),
		Enabled:             &enabled,
		Focused:             &focused,
		Limit:               3,
		MaxDepth:            4,
	})
	if err != nil {
		t.Fatalf("SearchAXElements returned error: %v", err)
	}
	if searchResult.Query.Scope != string(common.AXSearchScopeWindowHandle) || searchResult.Query.WindowHandle != 42 || searchResult.Query.Limit != 3 || searchResult.Query.MaxDepth != 4 {
		t.Fatalf("unexpected search result query echo: %+v", searchResult.Query)
	}
	if accessibility.lastSearchAXQuery.Scope != common.AXSearchScopeWindowHandle || accessibility.lastSearchAXQuery.WindowHandle != 42 || accessibility.lastSearchAXQuery.Role != "AXButton" || accessibility.lastSearchAXQuery.Action != string(common.AXPress) {
		t.Fatalf("unexpected forwarded search query: %+v", accessibility.lastSearchAXQuery)
	}
	if len(searchResult.Matches) != 1 {
		t.Fatalf("expected one AX search match, got %+v", searchResult.Matches)
	}
	match := searchResult.Matches[0]
	if match.Ref.Scope != string(common.AXSearchScopeWindowHandle) || match.Ref.OwnerPID != 77 || match.Ref.WindowHandle != 42 {
		t.Fatalf("unexpected AX ref result: %+v", match.Ref)
	}
	if len(match.Ref.Path) != 2 || match.Ref.Path[0] != 1 || match.Ref.Path[1] != 2 {
		t.Fatalf("unexpected AX ref path: %+v", match.Ref.Path)
	}
	if match.Metadata.Title != "Save" || match.Metadata.Frame.Left != 12 || !match.ActionPointKnown || match.ActionPoint != (Point{X: 20, Y: 40}) {
		t.Fatalf("unexpected AX match conversion: %+v", match)
	}

	focusResult, err := service.FocusAXElement(FocusAXElementRequest{Ref: match.Ref})
	if err != nil {
		t.Fatalf("FocusAXElement returned error: %v", err)
	}
	if !focusResult.OK || focusResult.Ref.WindowHandle != 42 || accessibility.lastFocusAXRef.WindowHandle != 42 {
		t.Fatalf("unexpected focus-ax-element result/ref forwarding: result=%+v ref=%+v", focusResult, accessibility.lastFocusAXRef)
	}

	actionResult, err := service.PerformAXElementAction(PerformAXElementActionOnRefRequest{Ref: match.Ref, Action: "AXPress"})
	if err != nil {
		t.Fatalf("PerformAXElementAction returned error: %v", err)
	}
	if !actionResult.OK || actionResult.Action != "AXPress" || accessibility.lastPerformAXRef.WindowHandle != 42 || accessibility.lastPerformAXAction != common.AXPress {
		t.Fatalf("unexpected perform-ax-element-action result/ref forwarding: result=%+v ref=%+v action=%q", actionResult, accessibility.lastPerformAXRef, accessibility.lastPerformAXAction)
	}
}

func TestSearchAXElementValidation(t *testing.T) {
	service := newTestServiceWithWindowsAndAccessibility(&fakeBackendAccessibilityProvider{})

	if _, err := service.SearchAXElements(SearchAXElementsRequest{Scope: "bogus", Limit: 1, MaxDepth: 0}); err == nil {
		t.Fatal("expected invalid AX search scope error")
	}
	if _, err := service.SearchAXElements(SearchAXElementsRequest{Scope: string(common.AXSearchScopeFocusedWindow), Limit: 0, MaxDepth: 0}); err == nil {
		t.Fatal("expected limit validation error")
	}
	if _, err := service.SearchAXElements(SearchAXElementsRequest{Scope: string(common.AXSearchScopeFocusedWindow), Limit: 1, MaxDepth: -1}); err == nil {
		t.Fatal("expected max_depth validation error")
	}
	if _, err := service.SearchAXElements(SearchAXElementsRequest{Scope: string(common.AXSearchScopeFocusedWindow), Action: "AXDefinitelyUnsupportedSyntheticAction", Limit: 1, MaxDepth: 0}); err == nil {
		t.Fatal("expected action validation error")
	}
	if _, err := service.SearchAXElements(SearchAXElementsRequest{Scope: string(common.AXSearchScopeWindowHandle), Limit: 1, MaxDepth: 0}); err == nil {
		t.Fatal("expected window_handle validation error")
	}
	if _, err := service.FocusAXElement(FocusAXElementRequest{Ref: AXElementRefResult{Scope: string(common.AXSearchScopeFocusedWindow), Path: []int{-1}}}); err == nil {
		t.Fatal("expected ref.path validation error for focus_ax_element")
	}
	if _, err := service.PerformAXElementAction(PerformAXElementActionOnRefRequest{Ref: AXElementRefResult{Scope: string(common.AXSearchScopeFocusedWindow), Path: []int{-1}}, Action: "AXPress"}); err == nil {
		t.Fatal("expected ref.path validation error for perform_ax_element_action")
	}
	if _, err := service.FocusAXElement(FocusAXElementRequest{Ref: AXElementRefResult{Scope: string(common.AXSearchScopeWindowHandle)}}); err == nil {
		t.Fatal("expected window_handle validation error for focus_ax_element")
	}
}

func TestFocusedWindowMetadataResultFromCommon(t *testing.T) {
	result := focusedWindowMetadataResultFromCommon(common.FocusedWindowMetadata{
		Handle:    42,
		Title:     "Terminal",
		Role:      "AXWindow",
		Subrole:   "AXStandardWindow",
		Rect:      common.Rect{X: 10, Y: 20, Width: 1280, Height: 720},
		RectKnown: true,
		Focused:   true,
		Main:      true,
		Minimized: false,
		OwnerPID:  123,
		OwnerName: "Terminal",
		BundleID:  "com.apple.Terminal",
	})

	if result.Handle != 42 || result.Region.Left != 10 || result.Region.Top != 20 || result.Region.Width != 1280 || result.Region.Height != 720 {
		t.Fatalf("unexpected focused window conversion: %+v", result)
	}
	if !result.RegionKnown || !result.Focused || !result.Main {
		t.Fatalf("expected focused window booleans to be preserved, got %+v", result)
	}
}

func TestUIElementMetadataResultFromCommon(t *testing.T) {
	result := uiElementMetadataResultFromCommon(common.UIElementMetadata{
		Role:        "AXButton",
		Subrole:     "",
		Title:       "Send",
		Description: "Sends the message",
		Value:       "",
		Enabled:     true,
		Focused:     false,
		Frame:       common.Rect{X: 30, Y: 40, Width: 90, Height: 24},
		FrameKnown:  true,
		Actions:     []string{"AXPress", "AXShowMenu"},
	})

	if result.Role != "AXButton" || result.Title != "Send" || result.Frame.Left != 30 || result.Frame.Top != 40 {
		t.Fatalf("unexpected ui element conversion: %+v", result)
	}
	if !result.FrameKnown || len(result.Actions) != 2 || result.Actions[0] != "AXPress" {
		t.Fatalf("expected ui element metadata to preserve frame and actions, got %+v", result)
	}
}

func TestCaptureResultIncludesOffset(t *testing.T) {
	result := CaptureResult{
		Path:          "/tmp/region.png",
		Message:       "Saved capture to /tmp/region.png",
		Offset:        Point{X: 2860, Y: 1510},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 800, Height: 600},
		DeliveredSize: Size{Width: 400, Height: 300},
		OriginalScale: Scale{X: 2, Y: 2},
	}
	if result.Offset.X != 2860 || result.Offset.Y != 1510 {
		t.Fatalf("unexpected offset: %#v", result.Offset)
	}
	if result.Scale.X != 1 || result.Scale.Y != 1 {
		t.Fatalf("unexpected delivered scale: %#v", result.Scale)
	}
	if result.OriginalSize != (Size{Width: 800, Height: 600}) || result.DeliveredSize != (Size{Width: 400, Height: 300}) {
		t.Fatalf("unexpected capture sizes: %#v", result)
	}
	if result.OriginalScale != (Scale{X: 2, Y: 2}) {
		t.Fatalf("unexpected original scale: %#v", result.OriginalScale)
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
		Path:          path,
		Message:       "Saved capture",
		Offset:        Point{X: 120, Y: 340},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 800, Height: 600},
		DeliveredSize: Size{Width: 400, Height: 300},
		OriginalScale: Scale{X: 2, Y: 2},
	}

	if err := writeCaptureMetadata(want); err != nil {
		t.Fatalf("write capture metadata: %v", err)
	}

	got, err := readCaptureMetadata(path)
	if err != nil {
		t.Fatalf("read capture metadata: %v", err)
	}

	if got.Path != want.Path || got.Offset != want.Offset || got.Scale != want.Scale || got.OriginalSize != want.OriginalSize || got.DeliveredSize != want.DeliveredSize || got.OriginalScale != want.OriginalScale {
		t.Fatalf("unexpected capture metadata: got %#v want %#v", got, want)
	}
}

func TestGetWindowAccessibilitySnapshotBuildsMarkdownInventory(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{
		searchAXElementsMatches: []common.AXElementMatch{
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{}},
				Metadata: common.UIElementMetadata{
					Role:       "AXWindow",
					Title:      "Slack",
					Enabled:    true,
					Frame:      common.Rect{X: 300, Y: 120, Width: 800, Height: 600},
					FrameKnown: true,
				},
				Depth: 0,
			},
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{0}},
				Metadata: common.UIElementMetadata{
					Role:       "AXButton",
					Title:      "Send",
					Enabled:    true,
					Frame:      common.Rect{X: 1000, Y: 700, Width: 32, Height: 24},
					FrameKnown: true,
					Actions:    []string{string(common.AXPress)},
				},
				Depth:            1,
				ActionPoint:      common.Point{X: 1016, Y: 712},
				ActionPointKnown: true,
			},
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{1}},
				Metadata: common.UIElementMetadata{
					Role:       "AXTextField",
					Value:      "Draft message",
					Focused:    true,
					Enabled:    true,
					Frame:      common.Rect{X: 400, Y: 680, Width: 560, Height: 42},
					FrameKnown: true,
				},
				Depth:            1,
				ActionPoint:      common.Point{X: 680, Y: 701},
				ActionPointKnown: true,
			},
		},
	}
	service := newTestServiceWithWindowsAndAccessibility(accessibility, WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})

	result, err := service.GetWindowAccessibilitySnapshot(WindowAccessibilitySnapshotRequest{Handle: 7})
	if err != nil {
		t.Fatalf("GetWindowAccessibilitySnapshot returned error: %v", err)
	}
	if result.SnapshotID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}
	if result.ElementCount != 3 {
		t.Fatalf("expected 3 flattened elements, got %d", result.ElementCount)
	}
	if accessibility.lastSearchAXQuery.Scope != common.AXSearchScopeWindowHandle || accessibility.lastSearchAXQuery.WindowHandle != common.WindowHandle(7) {
		t.Fatalf("expected window-handle scoped AX query, got %+v", accessibility.lastSearchAXQuery)
	}
	if !strings.Contains(result.Markdown, "Window accessibility snapshot") || !strings.Contains(result.Markdown, "`el-002`") || !strings.Contains(result.Markdown, "Send") {
		t.Fatalf("unexpected markdown snapshot:\n%s", result.Markdown)
	}
	if len(result.Elements) < 2 || result.Elements[1].ID == "" || result.Elements[1].ScreenRegion == nil || result.Elements[1].AXRef == nil {
		t.Fatalf("unexpected element list: %#v", result.Elements)
	}
	windowProvider, err := service.nut.Registry.Window()
	if err != nil {
		t.Fatalf("expected window provider: %v", err)
	}
	fakeWindows, ok := windowProvider.(*fakeBackendWindowProvider)
	if !ok {
		t.Fatalf("expected fake backend window provider, got %T", windowProvider)
	}
	if len(fakeWindows.focusCalls) != 0 {
		t.Fatalf("expected snapshot to avoid focusing the window, got %+v", fakeWindows.focusCalls)
	}
}

func TestNewServiceRegistersElementInspectionProvider(t *testing.T) {
	service := NewService()
	if _, err := service.nut.Registry.ElementInspection(); err != nil {
		t.Fatalf("expected default service registry to include element inspection provider, got %v", err)
	}
}

func TestActOnWindowAccessibilityElementFocusUsesCachedScreenRegion(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{
		searchAXElementsMatches: []common.AXElementMatch{
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{}},
				Metadata: common.UIElementMetadata{
					Role:       "AXWindow",
					Title:      "Slack",
					Enabled:    true,
					Frame:      common.Rect{X: 300, Y: 120, Width: 800, Height: 600},
					FrameKnown: true,
				},
			},
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{0}},
				Metadata: common.UIElementMetadata{
					Role:       "AXButton",
					Title:      "Send",
					Enabled:    true,
					Frame:      common.Rect{X: 1000, Y: 700, Width: 32, Height: 24},
					FrameKnown: true,
				},
			},
		},
	}
	service := newTestServiceWithWindowsAndAccessibility(accessibility, WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})

	snapshot, err := service.GetWindowAccessibilitySnapshot(WindowAccessibilitySnapshotRequest{Handle: 7})
	if err != nil {
		t.Fatalf("GetWindowAccessibilitySnapshot returned error: %v", err)
	}

	result, err := service.ActOnWindowAccessibilityElement(WindowAccessibilityElementActionRequest{
		SnapshotID: snapshot.SnapshotID,
		ElementID:  snapshot.Elements[1].ID,
		Action:     "focus",
	})
	if err != nil {
		t.Fatalf("ActOnWindowAccessibilityElement returned error: %v", err)
	}
	if result.ScreenPoint != (Point{X: 1016, Y: 712}) {
		t.Fatalf("unexpected action screen point: %+v", result.ScreenPoint)
	}
	if result.Mode != "background_ax" {
		t.Fatalf("expected background_ax mode, got %+v", result)
	}
	if accessibility.lastFocusAXRef.WindowHandle != 7 || accessibility.lastFocusAXRef.Scope != common.AXSearchScopeWindowHandle {
		t.Fatalf("unexpected AX focus ref: %+v", accessibility.lastFocusAXRef)
	}
}

func TestActOnWindowAccessibilityElementFallsBackToForegroundRawClickWhenAXActionMissing(t *testing.T) {
	accessibility := &fakeBackendAccessibilityProvider{
		searchAXElementsMatches: []common.AXElementMatch{
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{}},
				Metadata: common.UIElementMetadata{
					Role:       "AXWindow",
					Title:      "Slack",
					Enabled:    true,
					Frame:      common.Rect{X: 300, Y: 120, Width: 800, Height: 600},
					FrameKnown: true,
				},
			},
			{
				Ref: common.AXElementRef{Scope: common.AXSearchScopeWindowHandle, OwnerPID: 77, WindowHandle: 7, Path: []int{0}},
				Metadata: common.UIElementMetadata{
					Role:       "AXButton",
					Title:      "Send",
					Enabled:    true,
					Frame:      common.Rect{X: 1000, Y: 700, Width: 32, Height: 24},
					FrameKnown: true,
				},
			},
		},
	}
	service := newTestServiceWithWindowsAndAccessibility(accessibility,
		WindowSummary{Handle: 1, Title: "Terminal", Region: Region{Left: 20, Top: 20, Width: 500, Height: 300}},
		WindowSummary{Handle: 7, Title: "Slack", Region: Region{Left: 300, Top: 120, Width: 800, Height: 600}},
	)

	snapshot, err := service.GetWindowAccessibilitySnapshot(WindowAccessibilitySnapshotRequest{Handle: 7})
	if err != nil {
		t.Fatalf("GetWindowAccessibilitySnapshot returned error: %v", err)
	}

	result, err := service.ActOnWindowAccessibilityElement(WindowAccessibilityElementActionRequest{
		SnapshotID: snapshot.SnapshotID,
		ElementID:  snapshot.Elements[1].ID,
		Action:     "click",
	})
	if err != nil {
		t.Fatalf("ActOnWindowAccessibilityElement returned error: %v", err)
	}
	if result.Mode != "foreground_fallback" {
		t.Fatalf("expected foreground_fallback mode, got %+v", result)
	}

	windowProvider, err := service.nut.Registry.Window()
	if err != nil {
		t.Fatalf("expected window provider: %v", err)
	}
	fakeWindows, ok := windowProvider.(*fakeBackendWindowProvider)
	if !ok {
		t.Fatalf("expected fake backend window provider, got %T", windowProvider)
	}
	if len(fakeWindows.focusCalls) != 1 || fakeWindows.focusCalls[0] != shared.WindowHandle(7) {
		t.Fatalf("expected raw click fallback to focus window 7 first, got %+v", fakeWindows.focusCalls)
	}
}

func TestPlanCaptureDeliveredSizeNoOpWithoutBounds(t *testing.T) {
	size := planCaptureDeliveredSize(Size{Width: 800, Height: 600}, 0, 0)
	if size != (Size{Width: 800, Height: 600}) {
		t.Fatalf("expected no-op delivered size, got %#v", size)
	}
}

func TestPlanCaptureDeliveredSizeDownscalesWithinBounds(t *testing.T) {
	size := planCaptureDeliveredSize(Size{Width: 800, Height: 600}, 400, 250)
	if size != (Size{Width: 333, Height: 250}) {
		t.Fatalf("unexpected downscaled size: %#v", size)
	}
}

func TestTranslateDeliveredImagePointToScreenUsesOriginalAndDeliveredSpaces(t *testing.T) {
	result, err := translateDeliveredImagePointToScreen(CaptureResult{
		Path:          "/tmp/capture.png",
		Message:       "Saved capture",
		Offset:        Point{X: 50, Y: 80},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 800, Height: 600},
		DeliveredSize: Size{Width: 400, Height: 300},
		OriginalScale: Scale{X: 2, Y: 2},
	}, Point{X: 120, Y: 40})
	if err != nil {
		t.Fatalf("translateDeliveredImagePointToScreen returned error: %v", err)
	}
	if result.OriginalImagePoint != (ImagePoint{X: 240, Y: 80}) {
		t.Fatalf("unexpected original image point: %+v", result.OriginalImagePoint)
	}
	if result.ExactScreenPoint != (ImagePoint{X: 170, Y: 120}) {
		t.Fatalf("unexpected exact screen point: %+v", result.ExactScreenPoint)
	}
	if result.AbsoluteScreenPoint != (Point{X: 170, Y: 120}) {
		t.Fatalf("unexpected absolute screen point: %+v", result.AbsoluteScreenPoint)
	}
}

func TestTranslateDeliveredImagePointToScreenSupportsRetinaDownscaledExample(t *testing.T) {
	result, err := translateDeliveredImagePointToScreen(CaptureResult{
		Path:          "/tmp/retina-capture.png",
		Message:       "Saved capture",
		Offset:        Point{X: 2860, Y: 1510},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 1200, Height: 900},
		DeliveredSize: Size{Width: 600, Height: 450},
		OriginalScale: Scale{X: 2, Y: 2},
	}, Point{X: 200, Y: 100})
	if err != nil {
		t.Fatalf("translateDeliveredImagePointToScreen returned error: %v", err)
	}
	if result.OriginalImagePoint != (ImagePoint{X: 400, Y: 200}) {
		t.Fatalf("unexpected original image point: %+v", result.OriginalImagePoint)
	}
	if result.AbsoluteScreenPoint != (Point{X: 3060, Y: 1610}) {
		t.Fatalf("unexpected retina absolute screen point: %+v", result.AbsoluteScreenPoint)
	}
	if result.Capture.Scale != (Scale{X: 1, Y: 1}) || result.Capture.OriginalScale != (Scale{X: 2, Y: 2}) {
		t.Fatalf("unexpected capture scale metadata: %+v", result.Capture)
	}
}

func TestTranslateDeliveredImagePointToScreenRejectsOutOfBoundsPoint(t *testing.T) {
	_, err := translateDeliveredImagePointToScreen(CaptureResult{
		Path:          "/tmp/capture.png",
		Message:       "Saved capture",
		Offset:        Point{X: 0, Y: 0},
		Scale:         Scale{X: 1, Y: 1},
		OriginalSize:  Size{Width: 800, Height: 600},
		DeliveredSize: Size{Width: 400, Height: 300},
		OriginalScale: Scale{X: 2, Y: 2},
	}, Point{X: 400, Y: 10})
	if err == nil {
		t.Fatal("expected out-of-bounds delivered point to fail")
	}
	if !strings.Contains(err.Error(), "outside delivered image bounds") {
		t.Fatalf("expected bounds error, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
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

func TestFindWindowByHandleReturnsSummaryForKnownHandle(t *testing.T) {
	service := newTestServiceWithWindows(WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})

	window, err := service.FindWindowByHandle(WindowHandleRequest{Handle: 7})
	if err != nil {
		t.Fatalf("expected known handle lookup to succeed, got %v", err)
	}
	if window.Handle != 7 || window.Title != "Slack" || window.Region.Left != 300 || window.Region.Top != 120 {
		t.Fatalf("unexpected window summary: %#v", window)
	}
}

func TestFindWindowByHandleRejectsUnknownHandle(t *testing.T) {
	service := newTestServiceWithWindows(WindowSummary{
		Handle: 7,
		Title:  "Slack",
		Region: Region{Left: 300, Top: 120, Width: 800, Height: 600},
	})

	_, err := service.FindWindowByHandle(WindowHandleRequest{Handle: 999})
	if !errors.Is(err, errWindowHandleNotFound) {
		t.Fatalf("expected errWindowHandleNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "999") {
		t.Fatalf("expected invalid handle error to include the missing handle, got %v", err)
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

func TestCaptureCropBoundsAppliesScale(t *testing.T) {
	bounds, err := captureCropBounds(
		Region{Left: 10, Top: 20, Width: 30, Height: 40},
		Scale{X: 2, Y: 1.5},
		image.Rect(0, 0, 200, 200),
	)
	if err != nil {
		t.Fatalf("captureCropBounds returned error: %v", err)
	}

	want := image.Rect(20, 30, 80, 90)
	if bounds != want {
		t.Fatalf("unexpected crop bounds: got %v want %v", bounds, want)
	}
}

func TestCaptureCropBoundsRoundsOutwardForFractionalScale(t *testing.T) {
	bounds, err := captureCropBounds(
		Region{Left: 1, Top: 2, Width: 3, Height: 4},
		Scale{X: 1.5, Y: 1.5},
		image.Rect(0, 0, 20, 20),
	)
	if err != nil {
		t.Fatalf("captureCropBounds returned error: %v", err)
	}

	want := image.Rect(1, 3, 6, 9)
	if bounds != want {
		t.Fatalf("unexpected fractional crop bounds: got %v want %v", bounds, want)
	}
}

func TestCaptureCropBoundsRejectsOutOfBoundsRegion(t *testing.T) {
	_, err := captureCropBounds(
		Region{Left: 80, Top: 10, Width: 30, Height: 20},
		Scale{X: 2, Y: 2},
		image.Rect(0, 0, 200, 200),
	)
	if err == nil {
		t.Fatal("expected out-of-bounds crop to fail")
	}
	if !strings.Contains(err.Error(), "outside captured screen bounds") {
		t.Fatalf("expected out-of-bounds error, got %v", err)
	}
}

func TestCropCapturedRegionWritesCroppedImage(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.png")
	destPath := filepath.Join(dir, "cropped.png")

	source := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 20), B: uint8(x + y), A: 255})
		}
	}
	writePNG(t, sourcePath, source)

	region := Region{Left: 1, Top: 1, Width: 2, Height: 2}
	if err := cropCapturedRegion(sourcePath, destPath, region, Scale{X: 2, Y: 2}); err != nil {
		t.Fatalf("cropCapturedRegion returned error: %v", err)
	}

	cropped := readPNG(t, destPath)
	if cropped.Bounds().Dx() != 4 || cropped.Bounds().Dy() != 4 {
		t.Fatalf("unexpected cropped size: %v", cropped.Bounds())
	}
	if got := color.RGBAModel.Convert(cropped.At(0, 0)).(color.RGBA); got != (color.RGBA{R: 20, G: 40, B: 4, A: 255}) {
		t.Fatalf("unexpected cropped top-left pixel: %#v", got)
	}
	if got := color.RGBAModel.Convert(cropped.At(3, 3)).(color.RGBA); got != (color.RGBA{R: 50, G: 100, B: 10, A: 255}) {
		t.Fatalf("unexpected cropped bottom-right pixel: %#v", got)
	}

	scale := captureScale(destPath, Size{Width: region.Width, Height: region.Height})
	if scale.X != 2 || scale.Y != 2 {
		t.Fatalf("expected cropped capture scale to remain 2x, got %#v", scale)
	}
}

func TestCaptureRegionResultPreservesRequestedOffsetAndDeliveredScale(t *testing.T) {
	result := captureRegionResult(
		"/tmp/region.png",
		Region{Left: 2860, Top: 1510, Width: 400, Height: 300},
		Scale{X: 2, Y: 2},
		Size{Width: 800, Height: 600},
		Size{Width: 400, Height: 300},
	)

	if result.Offset != (Point{X: 2860, Y: 1510}) {
		t.Fatalf("unexpected offset: %#v", result.Offset)
	}
	if result.Scale != (Scale{X: 1, Y: 1}) {
		t.Fatalf("unexpected delivered scale: %#v", result.Scale)
	}
	if result.OriginalScale != (Scale{X: 2, Y: 2}) {
		t.Fatalf("unexpected original scale: %#v", result.OriginalScale)
	}
	if result.OriginalSize != (Size{Width: 800, Height: 600}) || result.DeliveredSize != (Size{Width: 400, Height: 300}) {
		t.Fatalf("unexpected capture sizes: %#v", result)
	}
}

func TestCaptureRegionResultNormalizesInvalidScale(t *testing.T) {
	result := captureRegionResult(
		"/tmp/region.png",
		Region{Left: 10, Top: 20, Width: 30, Height: 40},
		Scale{},
		Size{Width: 30, Height: 40},
		Size{Width: 30, Height: 40},
	)

	if result.Scale != (Scale{X: 1, Y: 1}) {
		t.Fatalf("unexpected normalized delivered scale: %#v", result.Scale)
	}
	if result.OriginalScale != (Scale{X: 1, Y: 1}) {
		t.Fatalf("unexpected normalized original scale: %#v", result.OriginalScale)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png %s: %v", path, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png %s: %v", path, err)
	}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open png %s: %v", path, err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode png %s: %v", path, err)
	}
	return img
}
