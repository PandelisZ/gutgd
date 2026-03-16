package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gut"
	"gut/clipboard"
	"gut/native/common"
	"gut/shared"
	guttesting "gut/testing"
	gutwindow "gut/window"
)

func requiredCapabilitiesForGOOS(goos string) []common.Capability {
	capabilities := []common.Capability{
		common.CapabilityKeyboardType,
		common.CapabilityKeyboardDelay,
		common.CapabilityMouseMove,
		common.CapabilityMouseDrag,
		common.CapabilityMousePosition,
		common.CapabilityMouseClick,
		common.CapabilityMouseToggle,
		common.CapabilityMouseScroll,
		common.CapabilityMouseDelay,
		common.CapabilityScreenSize,
		common.CapabilityWindowList,
		common.CapabilityWindowActive,
		common.CapabilityWindowRect,
		common.CapabilityWindowTitle,
		common.CapabilityWindowFocus,
		common.CapabilityWindowMove,
		common.CapabilityWindowResize,
	}
	if goos != "darwin" {
		capabilities = append(capabilities,
			common.CapabilityKeyboardTap,
			common.CapabilityKeyboardToggle,
			common.CapabilityScreenHighlight,
			common.CapabilityScreenCapture,
		)
	}
	return capabilities
}

type Service struct {
	mu                    sync.Mutex
	nut                   *gut.Nut
	clipboard             *clipboard.SystemProvider
	artifactDir           string
	emitEvent             func(string, any)
	agentCoordinateStates map[string]agentCoordinateState
}

type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func (s *Service) SetEventEmitter(emit func(string, any)) {
	s.emitEvent = emit
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Region struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Color struct {
	R   uint8  `json:"r"`
	G   uint8  `json:"g"`
	B   uint8  `json:"b"`
	A   uint8  `json:"a"`
	Hex string `json:"hex"`
}

type WindowSummary struct {
	Handle uint64 `json:"handle"`
	Title  string `json:"title"`
	Region Region `json:"region"`
}

type FeatureStatus struct {
	ID           string `json:"id"`
	Availability string `json:"availability"`
	Reason       string `json:"reason"`
}

type DiagnosticsResponse struct {
	Report        guttesting.EnvironmentReport `json:"report"`
	FeatureStatus []FeatureStatus              `json:"feature_status"`
	ArtifactsPath string                       `json:"artifacts_path"`
	WorkingDir    string                       `json:"working_dir"`
	Runtime       string                       `json:"runtime"`
}

type KeyboardTextRequest struct {
	Text        string `json:"text"`
	AutoDelayMS int    `json:"auto_delay_ms"`
}

type KeyboardKeysRequest struct {
	Keys        []string `json:"keys"`
	AutoDelayMS int      `json:"auto_delay_ms"`
}

type KeyboardSpecialKeyRequest struct {
	Key         string `json:"key"`
	RepeatCount int    `json:"repeat_count"`
}

type MouseMoveRequest struct {
	X           int `json:"x"`
	Y           int `json:"y"`
	AutoDelayMS int `json:"auto_delay_ms"`
}

type MouseLineRequest struct {
	X           int `json:"x"`
	Y           int `json:"y"`
	Speed       int `json:"speed"`
	AutoDelayMS int `json:"auto_delay_ms"`
}

type MouseButtonRequest struct {
	Button      string `json:"button"`
	AutoDelayMS int    `json:"auto_delay_ms"`
}

type MouseScrollRequest struct {
	Direction   string `json:"direction"`
	Amount      int    `json:"amount"`
	AutoDelayMS int    `json:"auto_delay_ms"`
}

type MouseDragRequest struct {
	FromX       int `json:"from_x"`
	FromY       int `json:"from_y"`
	ToX         int `json:"to_x"`
	ToY         int `json:"to_y"`
	Speed       int `json:"speed"`
	AutoDelayMS int `json:"auto_delay_ms"`
}

type CaptureRequest struct {
	FileName string `json:"file_name"`
}

type CaptureRegionRequest struct {
	FileName string `json:"file_name"`
	Region   Region `json:"region"`
}

type CaptureResult struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Offset  Point  `json:"offset"`
	Scale   Scale  `json:"scale"`
}

type PointRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ColorPointResult struct {
	Point   Point  `json:"point"`
	Color   Color  `json:"color"`
	Message string `json:"message"`
}

type ScreenSizeResult struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Scale struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type ClipboardCopyRequest struct {
	Text string `json:"text"`
}

type ClipboardPasteResult struct {
	Text string `json:"text"`
}

type ClipboardState struct {
	HasText bool `json:"has_text"`
}

type WindowHandleRequest struct {
	Handle uint64 `json:"handle"`
}

type WindowMoveRequest struct {
	Handle uint64 `json:"handle"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

type WindowResizeRequest struct {
	Handle uint64 `json:"handle"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type WindowQueryRequest struct {
	Title      string `json:"title"`
	UseRegex   bool   `json:"use_regex"`
	TimeoutMS  int    `json:"timeout_ms"`
	IntervalMS int    `json:"interval_ms"`
}

type ColorQueryRequest struct {
	R          uint8   `json:"r"`
	G          uint8   `json:"g"`
	B          uint8   `json:"b"`
	A          uint8   `json:"a"`
	Region     *Region `json:"region"`
	TimeoutMS  int     `json:"timeout_ms"`
	IntervalMS int     `json:"interval_ms"`
}

func NewService() *Service {
	return &Service{
		nut:                   gut.NewDefault(),
		clipboard:             clipboard.NewSystemProvider(),
		artifactDir:           filepath.Join(".", ".artifacts"),
		agentCoordinateStates: make(map[string]agentCoordinateState),
	}
}

func (s *Service) GetDiagnostics(mutable bool) (DiagnosticsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	report := guttesting.Evaluate(guttesting.Options{
		Mutable:              mutable,
		RequiredCapabilities: requiredCapabilitiesForGOOS(runtime.GOOS),
	})
	workingDir, err := os.Getwd()
	if err != nil {
		return DiagnosticsResponse{}, err
	}

	return DiagnosticsResponse{
		Report:        report.CapabilityReport(),
		FeatureStatus: featureStatusesForGOOS(runtime.GOOS, report),
		ArtifactsPath: s.artifactDir,
		WorkingDir:    workingDir,
		Runtime:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}, nil
}

func (s *Service) TypeText(req KeyboardTextRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Keyboard.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	if err := s.nut.Keyboard.TypeText(ctx, req.Text); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Typed %d characters", len(req.Text))}, nil
}

func (s *Service) TapKeys(req KeyboardKeysRequest) (ActionResult, error) {
	return s.runKeyboardKeys(req, func(ctx context.Context, keys []shared.Key) error {
		return s.nut.Keyboard.Tap(ctx, keys...)
	}, "Tapped")
}

func (s *Service) PressKeys(req KeyboardKeysRequest) (ActionResult, error) {
	return s.runKeyboardKeys(req, func(ctx context.Context, keys []shared.Key) error {
		return s.nut.Keyboard.Press(ctx, keys...)
	}, "Pressed")
}

func (s *Service) ReleaseKeys(req KeyboardKeysRequest) (ActionResult, error) {
	return s.runKeyboardKeys(req, func(ctx context.Context, keys []shared.Key) error {
		return s.nut.Keyboard.Release(ctx, keys...)
	}, "Released")
}

func (s *Service) PressSpecialKey(req KeyboardSpecialKeyRequest) (ActionResult, error) {
	if runtime.GOOS != "darwin" {
		return ActionResult{}, fmt.Errorf("press_special_key is currently only exposed as a safe fallback on macOS")
	}
	keyCode, ok := darwinSpecialKeyCode(strings.TrimSpace(req.Key))
	if !ok {
		return ActionResult{}, fmt.Errorf("unsupported special key %q", req.Key)
	}
	repeatCount := req.RepeatCount
	if repeatCount <= 0 {
		repeatCount = 1
	}
	script := darwinSpecialKeyScript(keyCode, repeatCount)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return ActionResult{}, err
		}
		return ActionResult{}, fmt.Errorf("%v: %s", err, message)
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Pressed %s %d time(s)", req.Key, repeatCount)}, nil
}

func (s *Service) GetMousePosition() (Point, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	position, err := s.nut.Mouse.Position(ctx)
	if err != nil {
		return Point{}, err
	}
	return pointFromShared(position), nil
}

func (s *Service) SetMousePosition(req MouseMoveRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Mouse.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	if err := s.nut.Mouse.SetPosition(ctx, shared.Point{X: req.X, Y: req.Y}); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Moved pointer to (%d, %d)", req.X, req.Y)}, nil
}

func (s *Service) MoveMouseLine(req MouseLineRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Mouse.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	if req.Speed > 0 {
		s.nut.Mouse.SetSpeed(req.Speed)
	}
	path, err := s.nut.Mouse.StraightTo(ctx, shared.Point{X: req.X, Y: req.Y})
	if err != nil {
		return ActionResult{}, err
	}
	if err := s.nut.Mouse.Move(ctx, path); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Moved along %d points to (%d, %d)", len(path), req.X, req.Y)}, nil
}

func (s *Service) ClickMouse(req MouseButtonRequest) (ActionResult, error) {
	return s.runMouseButton(req, func(ctx context.Context, button shared.Button) error {
		return s.nut.Mouse.Click(ctx, button)
	}, "Clicked")
}

func (s *Service) DoubleClickMouse(req MouseButtonRequest) (ActionResult, error) {
	return s.runMouseButton(req, func(ctx context.Context, button shared.Button) error {
		return s.nut.Mouse.DoubleClick(ctx, button)
	}, "Double-clicked")
}

func (s *Service) ScrollMouse(req MouseScrollRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Mouse.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	switch strings.ToLower(strings.TrimSpace(req.Direction)) {
	case "up":
		if err := s.nut.Mouse.ScrollUp(ctx, req.Amount); err != nil {
			return ActionResult{}, err
		}
	case "down":
		if err := s.nut.Mouse.ScrollDown(ctx, req.Amount); err != nil {
			return ActionResult{}, err
		}
	case "left":
		if err := s.nut.Mouse.ScrollLeft(ctx, req.Amount); err != nil {
			return ActionResult{}, err
		}
	case "right":
		if err := s.nut.Mouse.ScrollRight(ctx, req.Amount); err != nil {
			return ActionResult{}, err
		}
	default:
		return ActionResult{}, fmt.Errorf("unsupported scroll direction %q", req.Direction)
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Scrolled %s by %d", req.Direction, req.Amount)}, nil
}

func (s *Service) DragMouse(req MouseDragRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Mouse.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	if req.Speed > 0 {
		s.nut.Mouse.SetSpeed(req.Speed)
	}
	path := []shared.Point{
		{X: req.FromX, Y: req.FromY},
		{X: req.ToX, Y: req.ToY},
	}
	if err := s.nut.Mouse.Drag(ctx, path); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Dragged from (%d, %d) to (%d, %d)", req.FromX, req.FromY, req.ToX, req.ToY)}, nil
}

func (s *Service) GetScreenSize() (ScreenSizeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	width, err := s.nut.Screen.Width(ctx)
	if err != nil {
		return ScreenSizeResult{}, err
	}
	height, err := s.nut.Screen.Height(ctx)
	if err != nil {
		return ScreenSizeResult{}, err
	}
	return ScreenSizeResult{Width: width, Height: height}, nil
}

func (s *Service) CaptureScreen(req CaptureRequest) (CaptureResult, error) {
	if runtime.GOOS == "darwin" {
		return s.captureWithCommand("", req.FileName, Point{}, Size{})
	}
	return s.capture(func(ctx context.Context, path string) (string, error) {
		return s.nut.Screen.Capture(ctx, path)
	}, req.FileName, Point{})
}

func (s *Service) CaptureRegion(req CaptureRegionRequest) (CaptureResult, error) {
	if runtime.GOOS == "darwin" {
		return s.captureWithCommand(
			fmt.Sprintf("%d,%d,%d,%d", req.Region.Left, req.Region.Top, req.Region.Width, req.Region.Height),
			req.FileName,
			Point{X: req.Region.Left, Y: req.Region.Top},
			Size{Width: req.Region.Width, Height: req.Region.Height},
		)
	}
	region := shared.Region{
		Left:   req.Region.Left,
		Top:    req.Region.Top,
		Width:  req.Region.Width,
		Height: req.Region.Height,
	}
	return s.capture(func(ctx context.Context, path string) (string, error) {
		return s.nut.Screen.CaptureRegion(ctx, path, region)
	}, req.FileName, Point{X: req.Region.Left, Y: req.Region.Top})
}

func (s *Service) ColorAt(req PointRequest) (ColorPointResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	color, err := s.nut.Screen.ColorAt(ctx, shared.Point{X: req.X, Y: req.Y})
	if err != nil {
		return ColorPointResult{}, err
	}
	return ColorPointResult{
		Point:   Point{X: req.X, Y: req.Y},
		Color:   colorFromShared(color),
		Message: fmt.Sprintf("Read %s at (%d, %d)", color.Hex(), req.X, req.Y),
	}, nil
}

func (s *Service) HighlightRegion(req Region) (ActionResult, error) {
	if runtime.GOOS == "darwin" {
		return ActionResult{}, common.CapabilityUnavailable("highlightRegion", runtime.GOOS, common.CapabilityScreenHighlight, "highlight_region is intentionally hidden on macOS 26+; use capture_region plus analyze_screenshot instead")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.nut.Screen.Highlight(ctx, shared.Region{
		Left:   req.Left,
		Top:    req.Top,
		Width:  req.Width,
		Height: req.Height,
	})
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: "Highlighted region"}, nil
}

func (s *Service) ListWindows() ([]WindowSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	windows, err := gut.GetWindows(ctx, s.nut.Registry)
	if err != nil {
		return nil, err
	}
	return s.windowSummaries(ctx, windows)
}

func (s *Service) GetActiveWindow() (WindowSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	active, err := gut.GetActiveWindow(ctx, s.nut.Registry)
	if err != nil {
		return WindowSummary{}, err
	}
	return s.windowSummary(ctx, active)
}

func (s *Service) FocusWindow(req WindowHandleRequest) (ActionResult, error) {
	return s.runWindowMutation(req.Handle, func(ctx context.Context, window *gut.Window) (bool, error) {
		return window.Focus(ctx)
	}, "Focused")
}

func (s *Service) MoveWindow(req WindowMoveRequest) (ActionResult, error) {
	return s.runWindowMutation(req.Handle, func(ctx context.Context, window *gut.Window) (bool, error) {
		return window.Move(ctx, shared.Point{X: req.X, Y: req.Y})
	}, "Moved")
}

func (s *Service) ResizeWindow(req WindowResizeRequest) (ActionResult, error) {
	return s.runWindowMutation(req.Handle, func(ctx context.Context, window *gut.Window) (bool, error) {
		return window.Resize(ctx, shared.Size{Width: req.Width, Height: req.Height})
	}, "Resized")
}

func (s *Service) MinimizeWindow(req WindowHandleRequest) (ActionResult, error) {
	return s.runWindowMutation(req.Handle, func(ctx context.Context, window *gut.Window) (bool, error) {
		return window.Minimize(ctx)
	}, "Minimized")
}

func (s *Service) RestoreWindow(req WindowHandleRequest) (ActionResult, error) {
	return s.runWindowMutation(req.Handle, func(ctx context.Context, window *gut.Window) (bool, error) {
		return window.Restore(ctx)
	}, "Restored")
}

func (s *Service) FindColor(req ColorQueryRequest) (Point, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	var opts *gut.ScreenFindOptions
	if req.Region != nil {
		opts = &gut.ScreenFindOptions{
			SearchRegion: &shared.Region{
				Left:   req.Region.Left,
				Top:    req.Region.Top,
				Width:  req.Region.Width,
				Height: req.Region.Height,
			},
		}
	}
	match, err := s.nut.Screen.Find(ctx, gut.PixelWithColor(shared.RGBA{
		R: req.R,
		G: req.G,
		B: req.B,
		A: req.A,
	}), opts)
	if err != nil {
		return Point{}, err
	}
	point, ok := match.(shared.Point)
	if !ok {
		return Point{}, fmt.Errorf("unexpected color match type %T", match)
	}
	return pointFromShared(point), nil
}

func (s *Service) WaitForColor(req ColorQueryRequest) (Point, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout(req.TimeoutMS, 30*time.Second))
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	var opts *gut.ScreenFindOptions
	if req.Region != nil {
		opts = &gut.ScreenFindOptions{
			SearchRegion: &shared.Region{
				Left:   req.Region.Left,
				Top:    req.Region.Top,
				Width:  req.Region.Width,
				Height: req.Region.Height,
			},
		}
	}
	match, err := s.nut.Screen.WaitFor(ctx, gut.PixelWithColor(shared.RGBA{
		R: req.R,
		G: req.G,
		B: req.B,
		A: req.A,
	}), s.timeout(req.TimeoutMS, 5*time.Second), s.timeout(req.IntervalMS, 250*time.Millisecond), opts)
	if err != nil {
		return Point{}, err
	}
	point, ok := match.(shared.Point)
	if !ok {
		return Point{}, fmt.Errorf("unexpected color match type %T", match)
	}
	return pointFromShared(point), nil
}

func (s *Service) AssertColorVisible(req ColorQueryRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	var searchRegion *shared.Region
	if req.Region != nil {
		searchRegion = &shared.Region{
			Left:   req.Region.Left,
			Top:    req.Region.Top,
			Width:  req.Region.Width,
			Height: req.Region.Height,
		}
	}
	err := s.nut.Assert.Visible(ctx, gut.PixelWithColor(shared.RGBA{
		R: req.R,
		G: req.G,
		B: req.B,
		A: req.A,
	}), searchRegion, nil)
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: "Color is visible"}, nil
}

func (s *Service) FindWindowByTitle(req WindowQueryRequest) (WindowSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	window, err := s.findWindow(ctx, req)
	if err != nil {
		return WindowSummary{}, err
	}
	return s.windowSummary(ctx, window)
}

func (s *Service) WaitForWindowByTitle(req WindowQueryRequest) (WindowSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout(req.TimeoutMS, 30*time.Second))
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	query, err := buildWindowQuery(req)
	if err != nil {
		return WindowSummary{}, err
	}
	match, err := s.nut.Screen.WaitFor(ctx, query, s.timeout(req.TimeoutMS, 5*time.Second), s.timeout(req.IntervalMS, 250*time.Millisecond), nil)
	if err != nil {
		return WindowSummary{}, err
	}
	window, ok := match.(*gut.Window)
	if !ok {
		return WindowSummary{}, fmt.Errorf("unexpected window match type %T", match)
	}
	return s.windowSummary(ctx, window)
}

func (s *Service) AssertWindowVisible(req WindowQueryRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	query, err := buildWindowQuery(req)
	if err != nil {
		return ActionResult{}, err
	}
	if err := s.nut.Assert.Visible(ctx, query, nil, nil); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: "Window is visible"}, nil
}

func (s *Service) ClipboardHasText() (ClipboardState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	hasText, err := s.clipboard.HasText(ctx)
	if err != nil {
		return ClipboardState{}, err
	}
	return ClipboardState{HasText: hasText}, nil
}

func (s *Service) ClipboardCopy(req ClipboardCopyRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.clipboard.Copy(ctx, req.Text); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Copied %d characters to the clipboard", len(req.Text))}, nil
}

func (s *Service) ClipboardPaste() (ClipboardPasteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	text, err := s.clipboard.Paste(ctx)
	if err != nil {
		return ClipboardPasteResult{}, err
	}
	return ClipboardPasteResult{Text: text}, nil
}

func (s *Service) ClipboardClear() (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.clipboard.Clear(ctx)
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: "Cleared clipboard text"}, nil
}

func (s *Service) capture(run func(context.Context, string) (string, error), fileName string, offset Point) (CaptureResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.capturePath(fileName)
	if err != nil {
		return CaptureResult{}, err
	}
	savedPath, err := run(ctx, path)
	if err != nil {
		return CaptureResult{}, err
	}
	result := CaptureResult{
		Path:    savedPath,
		Message: fmt.Sprintf("Saved capture to %s", savedPath),
		Offset:  offset,
		Scale:   Scale{X: 1, Y: 1},
	}
	if err := writeCaptureMetadata(result); err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

func (s *Service) captureWithCommand(region string, fileName string, offset Point, logicalSize Size) (CaptureResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.capturePath(fileName)
	if err != nil {
		return CaptureResult{}, err
	}

	cmd := exec.CommandContext(ctx, "screencapture", screencaptureArgs(region, path)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return CaptureResult{}, err
		}
		return CaptureResult{}, fmt.Errorf("%v: %s", err, message)
	}

	if logicalSize.Width == 0 || logicalSize.Height == 0 {
		width, err := s.nut.Screen.Width(ctx)
		if err != nil {
			return CaptureResult{}, err
		}
		height, err := s.nut.Screen.Height(ctx)
		if err != nil {
			return CaptureResult{}, err
		}
		logicalSize = Size{Width: width, Height: height}
	}

	scale := captureScale(path, logicalSize)

	result := CaptureResult{
		Path:    path,
		Message: fmt.Sprintf("Saved capture to %s", path),
		Offset:  offset,
		Scale:   scale,
	}
	if err := writeCaptureMetadata(result); err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

func captureScale(path string, logicalSize Size) Scale {
	if logicalSize.Width <= 0 || logicalSize.Height <= 0 {
		return Scale{X: 1, Y: 1}
	}

	file, err := os.Open(path)
	if err != nil {
		return Scale{X: 1, Y: 1}
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return Scale{X: 1, Y: 1}
	}

	scaleX := float64(config.Width) / float64(logicalSize.Width)
	scaleY := float64(config.Height) / float64(logicalSize.Height)
	if scaleX <= 0 {
		scaleX = 1
	}
	if scaleY <= 0 {
		scaleY = 1
	}

	return Scale{X: scaleX, Y: scaleY}
}

func writeCaptureMetadata(result CaptureResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(captureMetadataPath(result.Path), data, 0o644)
}

func readCaptureMetadata(path string) (CaptureResult, error) {
	data, err := os.ReadFile(captureMetadataPath(path))
	if err != nil {
		return CaptureResult{}, err
	}
	var result CaptureResult
	if err := json.Unmarshal(data, &result); err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

func captureMetadataPath(path string) string {
	return path + ".json"
}

func screencaptureArgs(region string, path string) []string {
	args := []string{"-x"}
	if strings.TrimSpace(region) != "" {
		args = append(args, "-R", region)
	}
	return append(args, path)
}

func darwinSpecialKeyCode(value string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enter", "return":
		return 36, true
	case "tab":
		return 48, true
	case "escape", "esc":
		return 53, true
	case "space":
		return 49, true
	case "backspace", "delete":
		return 51, true
	case "forward_delete":
		return 117, true
	case "up":
		return 126, true
	case "down":
		return 125, true
	case "left":
		return 123, true
	case "right":
		return 124, true
	default:
		return 0, false
	}
}

func darwinSpecialKeyScript(keyCode int, repeatCount int) string {
	return fmt.Sprintf(`tell application "System Events"
repeat %d times
	key code %d
end repeat
end tell`, repeatCount, keyCode)
}

func (s *Service) capturePath(fileName string) (string, error) {
	if err := os.MkdirAll(s.artifactDir, 0o755); err != nil {
		return "", err
	}
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = fmt.Sprintf("capture-%s.png", time.Now().Format("20060102-150405"))
	}
	if filepath.Ext(name) == "" {
		name += ".png"
	}
	return filepath.Join(s.artifactDir, filepath.Base(name)), nil
}

func (s *Service) runKeyboardKeys(req KeyboardKeysRequest, run func(context.Context, []shared.Key) error, verb string) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]shared.Key, 0, len(req.Keys))
	for _, item := range req.Keys {
		key, err := parseKey(item)
		if err != nil {
			return ActionResult{}, err
		}
		keys = append(keys, key)
	}
	s.nut.Keyboard.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	if err := run(ctx, keys); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("%s %s", verb, strings.Join(req.Keys, ", "))}, nil
}

func (s *Service) runMouseButton(req MouseButtonRequest, run func(context.Context, shared.Button) error, verb string) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	button, err := parseButton(req.Button)
	if err != nil {
		return ActionResult{}, err
	}
	s.nut.Mouse.SetAutoDelay(time.Duration(req.AutoDelayMS) * time.Millisecond)
	if err := run(ctx, button); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("%s %s button", verb, req.Button)}, nil
}

func (s *Service) runWindowMutation(handle uint64, run func(context.Context, *gut.Window) (bool, error), verb string) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	windowPtr := gutwindow.New(s.nut.Registry, shared.WindowHandle(handle))
	ok, err := run(ctx, windowPtr)
	if err != nil {
		return ActionResult{}, err
	}
	if !ok {
		return ActionResult{OK: false, Message: fmt.Sprintf("%s window request was not acknowledged", verb)}, nil
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("%s window %d", verb, handle)}, nil
}

func (s *Service) findWindow(ctx context.Context, req WindowQueryRequest) (*gut.Window, error) {
	query, err := buildWindowQuery(req)
	if err != nil {
		return nil, err
	}
	match, err := s.nut.Screen.Find(ctx, query, nil)
	if err != nil {
		return nil, err
	}
	window, ok := match.(*gut.Window)
	if !ok {
		return nil, fmt.Errorf("unexpected window match type %T", match)
	}
	return window, nil
}

func (s *Service) windowSummaries(ctx context.Context, windows []*gut.Window) ([]WindowSummary, error) {
	result := make([]WindowSummary, 0, len(windows))
	for _, item := range windows {
		summary, err := s.windowSummary(ctx, item)
		if err != nil {
			return nil, err
		}
		result = append(result, summary)
	}
	return result, nil
}

func (s *Service) windowSummary(ctx context.Context, window *gut.Window) (WindowSummary, error) {
	title, err := window.Title(ctx)
	if err != nil {
		return WindowSummary{}, err
	}
	region, err := window.Region(ctx)
	if err != nil {
		return WindowSummary{}, err
	}
	return WindowSummary{
		Handle: uint64(window.Handle),
		Title:  title,
		Region: regionFromShared(region),
	}, nil
}

func featureStatusesForGOOS(goos string, report guttesting.Report) []FeatureStatus {
	statuses := make([]FeatureStatus, 0, len(report.CapabilityStatuses())+11)
	for _, status := range report.CapabilityStatuses() {
		statuses = append(statuses, FeatureStatus{
			ID:           string(status.Capability),
			Availability: string(status.Availability),
			Reason:       status.Reason,
		})
	}

	if goos == "darwin" {
		statuses = append(statuses,
			availableFeatureStatus("capture_screen", "available via the macOS safe screenshot fallback; native screen.capture is intentionally not required"),
			availableFeatureStatus("capture_region", "available via the macOS safe screenshot fallback; native screen.capture is intentionally not required"),
			availableFeatureStatus("press_special_key", "available via the macOS System Events special-key path"),
			unavailableFeatureStatus("tap_keys", "intentionally hidden on macOS 26+; use type_text or press_special_key instead"),
			unavailableFeatureStatus("press_keys", "intentionally hidden on macOS 26+ because low-level key toggles are not exposed"),
			unavailableFeatureStatus("release_keys", "intentionally hidden on macOS 26+ because low-level key toggles are not exposed"),
			unavailableFeatureStatus("highlight_region", "intentionally hidden on macOS 26+; use capture_region plus analyze_screenshot instead"),
		)
	} else {
		statuses = append(statuses,
			toolStatusFromCapability("capture_screen", report.Capabilities.Status(common.CapabilityScreenCapture), "available through the native screen.capture backend"),
			toolStatusFromCapability("capture_region", report.Capabilities.Status(common.CapabilityScreenCapture), "available through the native screen.capture backend"),
			unavailableFeatureStatus("press_special_key", "only exposed on darwin as the macOS safe special-key path"),
			toolStatusFromCapability("tap_keys", report.Capabilities.Status(common.CapabilityKeyboardTap), "available through the native keyboard.tap backend"),
			toolStatusFromCapability("press_keys", report.Capabilities.Status(common.CapabilityKeyboardToggle), "available through the native keyboard.toggle backend"),
			toolStatusFromCapability("release_keys", report.Capabilities.Status(common.CapabilityKeyboardToggle), "available through the native keyboard.toggle backend"),
			toolStatusFromCapability("highlight_region", report.Capabilities.Status(common.CapabilityScreenHighlight), "available through the native screen.highlight backend"),
		)
	}

	statuses = append(statuses,
		toolStatusFromCapability("minimize_window", report.Capabilities.Status(common.CapabilityWindowMinimize), "available through the native window.minimize backend"),
		toolStatusFromCapability("restore_window", report.Capabilities.Status(common.CapabilityWindowRestore), "available through the native window.restore backend"),
		FeatureStatus{
			ID:           "screen.find.image",
			Availability: "unavailable",
			Reason:       "image finder is not registered in the default gut registry",
		},
		FeatureStatus{
			ID:           "screen.find.text",
			Availability: "unavailable",
			Reason:       "text finder is not registered in the default gut registry",
		},
		FeatureStatus{
			ID:           "window.elements",
			Availability: "unavailable",
			Reason:       "window element inspection is defined in the API but not wired into the default gut registry",
		},
	)
	return statuses
}

func availableFeatureStatus(id string, reason string) FeatureStatus {
	return FeatureStatus{
		ID:           id,
		Availability: string(common.AvailabilityAvailable),
		Reason:       reason,
	}
}

func unavailableFeatureStatus(id string, reason string) FeatureStatus {
	return FeatureStatus{
		ID:           id,
		Availability: string(common.AvailabilityUnavailable),
		Reason:       reason,
	}
}

func toolStatusFromCapability(id string, status common.CapabilityStatus, availableReason string) FeatureStatus {
	if status.Availability == common.AvailabilityAvailable {
		return availableFeatureStatus(id, availableReason)
	}

	reason := fmt.Sprintf("native %s is %s", status.Capability, status.Availability)
	if status.Reason != "" {
		reason += ": " + status.Reason
	}
	return unavailableFeatureStatus(id, reason)
}

func (s *Service) timeout(valueMS int, fallback time.Duration) time.Duration {
	if valueMS <= 0 {
		return fallback
	}
	return time.Duration(valueMS) * time.Millisecond
}

func pointFromShared(point shared.Point) Point {
	return Point{X: point.X, Y: point.Y}
}

func regionFromShared(region shared.Region) Region {
	return Region{
		Left:   region.Left,
		Top:    region.Top,
		Width:  region.Width,
		Height: region.Height,
	}
}

func colorFromShared(color shared.RGBA) Color {
	return Color{
		R:   color.R,
		G:   color.G,
		B:   color.B,
		A:   color.A,
		Hex: color.Hex(),
	}
}
