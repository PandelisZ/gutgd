package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PandelisZ/gut"
	"github.com/PandelisZ/gut/backgroundmouse"
	"github.com/PandelisZ/gut/clipboard"
	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/shared"
	guttesting "github.com/PandelisZ/gut/testing"
	gutwindow "github.com/PandelisZ/gut/window"
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
	mu                     sync.Mutex
	nut                    *gut.Nut
	clipboard              *clipboard.SystemProvider
	artifactDir            string
	emitEvent              func(string, any)
	agentCoordinateStates  map[string]agentCoordinateState
	agentPointerStates     map[string]agentPointerState
	accessibilitySnapshots map[string]windowAccessibilitySnapshotCache
	agentLuaSessions       map[string]*agentLuaSession
	agentTranscriptStates  map[string][]AgentTranscriptItem
}

type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

const (
	agentKeyboardAutoDelay         = 0 * time.Millisecond
	agentMouseAutoDelay            = 0 * time.Millisecond
	agentMouseSpeed                = 5000
	agentMouseInstantPathThreshold = 24
	agentMouseMaxPathPoints        = 48
)

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

var errWindowHandleNotFound = errors.New("window handle not found")

type FeatureStatus struct {
	ID           string `json:"id"`
	Availability string `json:"availability"`
	Reason       string `json:"reason"`
}

type PermissionStatusResult struct {
	Granted   bool   `json:"granted"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

type PermissionSnapshotResult struct {
	Accessibility   PermissionStatusResult `json:"accessibility"`
	ScreenRecording PermissionStatusResult `json:"screen_recording"`
}

type PermissionReadinessResult struct {
	Permissions  PermissionSnapshotResult `json:"permissions"`
	Capabilities []FeatureStatus          `json:"capabilities"`
	Message      string                   `json:"message"`
}

type FocusedWindowMetadataResult struct {
	Handle      uint64 `json:"handle"`
	Title       string `json:"title"`
	Role        string `json:"role"`
	Subrole     string `json:"subrole"`
	Region      Region `json:"region"`
	RegionKnown bool   `json:"region_known"`
	Focused     bool   `json:"focused"`
	Main        bool   `json:"main"`
	Minimized   bool   `json:"minimized"`
	OwnerPID    int    `json:"owner_pid"`
	OwnerName   string `json:"owner_name"`
	BundleID    string `json:"bundle_id"`
}

type UIElementMetadataResult struct {
	Role        string   `json:"role"`
	Subrole     string   `json:"subrole"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Value       string   `json:"value"`
	Enabled     bool     `json:"enabled"`
	Focused     bool     `json:"focused"`
	Frame       Region   `json:"frame"`
	FrameKnown  bool     `json:"frame_known"`
	Actions     []string `json:"actions"`
}

type WindowAccessibilitySnapshotRequest struct {
	Handle uint64 `json:"handle"`
}

type WindowAccessibilityElement struct {
	ID                    string              `json:"id"`
	Path                  string              `json:"path"`
	Depth                 int                 `json:"depth"`
	Type                  string              `json:"type"`
	Role                  string              `json:"role"`
	Subrole               string              `json:"subrole"`
	Title                 string              `json:"title"`
	Value                 string              `json:"value"`
	SelectedText          string              `json:"selected_text"`
	EnabledKnown          bool                `json:"enabled_known"`
	Enabled               bool                `json:"enabled"`
	FocusedKnown          bool                `json:"focused_known"`
	Focused               bool                `json:"focused"`
	ScreenRegion          *Region             `json:"screen_region,omitempty"`
	ActionPoint           Point               `json:"action_point"`
	ActionPointKnown      bool                `json:"action_point_known"`
	AXRef                 *AXElementRefResult `json:"ax_ref,omitempty"`
	AXActions             []string            `json:"ax_actions"`
	BackgroundSafeActions []string            `json:"background_safe_actions"`
	AvailableActions      []string            `json:"available_actions"`
}

type WindowAccessibilitySnapshotResult struct {
	SnapshotID   string                       `json:"snapshot_id"`
	Window       WindowSummary                `json:"window"`
	ElementCount int                          `json:"element_count"`
	Elements     []WindowAccessibilityElement `json:"elements"`
	Markdown     string                       `json:"markdown"`
	Message      string                       `json:"message"`
}

type WindowAccessibilityElementActionRequest struct {
	SnapshotID string `json:"snapshot_id"`
	ElementID  string `json:"element_id"`
	Action     string `json:"action"`
}

type WindowAccessibilityElementActionResult struct {
	SnapshotID  string       `json:"snapshot_id"`
	ElementID   string       `json:"element_id"`
	Action      string       `json:"action"`
	ScreenPoint Point        `json:"screen_point"`
	Mode        string       `json:"mode"`
	Result      ActionResult `json:"result"`
	Message     string       `json:"message"`
}

type BackgroundMouseResolveRequest struct {
	SnapshotID string `json:"snapshot_id"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
}

type BackgroundMouseResolveResult struct {
	SnapshotID     string                     `json:"snapshot_id"`
	RequestedPoint Point                      `json:"requested_point"`
	ScreenPoint    Point                      `json:"screen_point"`
	Snapped        bool                       `json:"snapped"`
	ElementID      string                     `json:"element_id"`
	Element        WindowAccessibilityElement `json:"element"`
	Mode           string                     `json:"mode"`
	Message        string                     `json:"message"`
}

type BackgroundMouseActionRequest struct {
	SnapshotID string        `json:"snapshot_id"`
	Action     string        `json:"action"`
	Point      *PointRequest `json:"point,omitempty"`
	ElementID  string        `json:"element_id,omitempty"`
}

type BackgroundMouseActionResult struct {
	SnapshotID     string                     `json:"snapshot_id"`
	Action         string                     `json:"action"`
	RequestedPoint *Point                     `json:"requested_point,omitempty"`
	ScreenPoint    Point                      `json:"screen_point"`
	Snapped        bool                       `json:"snapped"`
	ElementID      string                     `json:"element_id"`
	Element        WindowAccessibilityElement `json:"element"`
	Mode           string                     `json:"mode"`
	Result         ActionResult               `json:"result"`
	Message        string                     `json:"message"`
}

type windowAccessibilitySnapshotCache struct {
	Window             WindowSummary
	Elements           map[string]WindowAccessibilityElement
	ElementIDsByRef    map[string]string
	BackgroundSnapshot backgroundmouse.WindowSnapshot
	LastVirtualPoint   *Point
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

type KeyboardShortcutRequest struct {
	Keys []string `json:"keys"`
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
	FileName       string `json:"file_name"`
	MaxImageWidth  int    `json:"max_image_width"`
	MaxImageHeight int    `json:"max_image_height"`
}

type CaptureRegionRequest struct {
	FileName       string `json:"file_name"`
	Region         Region `json:"region"`
	MaxImageWidth  int    `json:"max_image_width"`
	MaxImageHeight int    `json:"max_image_height"`
}

type CaptureWindowRequest struct {
	Handle         uint64 `json:"handle"`
	FileName       string `json:"file_name"`
	MaxImageWidth  int    `json:"max_image_width"`
	MaxImageHeight int    `json:"max_image_height"`
}

type CaptureResult struct {
	Path          string `json:"path"`
	Message       string `json:"message"`
	Offset        Point  `json:"offset"`
	Scale         Scale  `json:"scale"`
	OriginalSize  Size   `json:"original_size"`
	DeliveredSize Size   `json:"delivered_size"`
	OriginalScale Scale  `json:"original_scale"`
}

type ImagePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type ImagePointTranslationResult struct {
	Path                    string        `json:"path"`
	RequestedDeliveredPoint Point         `json:"requested_delivered_point"`
	OriginalImagePoint      ImagePoint    `json:"original_image_point"`
	ExactScreenPoint        ImagePoint    `json:"exact_screen_point"`
	AbsoluteScreenPoint     Point         `json:"absolute_screen_point"`
	Capture                 CaptureResult `json:"capture"`
	Message                 string        `json:"message"`
}

type PointRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type AXActionRequest struct {
	Action string `json:"action"`
}

type SearchAXElementsRequest struct {
	Scope               string `json:"scope"`
	WindowHandle        uint64 `json:"window_handle"`
	Role                string `json:"role"`
	Subrole             string `json:"subrole"`
	TitleContains       string `json:"title_contains"`
	ValueContains       string `json:"value_contains"`
	DescriptionContains string `json:"description_contains"`
	Action              string `json:"action"`
	Enabled             *bool  `json:"enabled"`
	Focused             *bool  `json:"focused"`
	Limit               int    `json:"limit"`
	MaxDepth            int    `json:"max_depth"`
}

type AXElementRefResult struct {
	Scope        string `json:"scope"`
	OwnerPID     int    `json:"owner_pid"`
	WindowHandle uint64 `json:"window_handle"`
	Path         []int  `json:"path"`
}

type AXElementMatchResult struct {
	Ref              AXElementRefResult      `json:"ref"`
	Metadata         UIElementMetadataResult `json:"metadata"`
	Depth            int                     `json:"depth"`
	ActionPoint      Point                   `json:"action_point"`
	ActionPointKnown bool                    `json:"action_point_known"`
}

type SearchAXElementsResult struct {
	Query   SearchAXElementsRequest `json:"query"`
	Matches []AXElementMatchResult  `json:"matches"`
	Message string                  `json:"message"`
}

type FocusAXElementRequest struct {
	Ref AXElementRefResult `json:"ref"`
}

type FocusAXElementResult struct {
	OK      bool               `json:"ok"`
	Ref     AXElementRefResult `json:"ref"`
	Message string             `json:"message"`
}

type AXActionAtPointRequest struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Action string `json:"action"`
}

type PerformAXElementActionOnRefRequest struct {
	Ref    AXElementRefResult `json:"ref"`
	Action string             `json:"action"`
}

type PerformAXElementActionResult struct {
	OK      bool               `json:"ok"`
	Ref     AXElementRefResult `json:"ref"`
	Action  string             `json:"action"`
	Message string             `json:"message"`
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
		nut:                    gut.NewDefault(),
		clipboard:              clipboard.NewSystemProvider(),
		artifactDir:            filepath.Join(".", ".artifacts"),
		agentCoordinateStates:  make(map[string]agentCoordinateState),
		agentPointerStates:     make(map[string]agentPointerState),
		accessibilitySnapshots: make(map[string]windowAccessibilitySnapshotCache),
		agentLuaSessions:       make(map[string]*agentLuaSession),
		agentTranscriptStates:  make(map[string][]AgentTranscriptItem),
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

func (s *Service) GetPermissionReadiness() (PermissionReadinessResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return PermissionReadinessResult{}, err
	}

	snapshot, err := accessibility.GetPermissionSnapshot(ctx)
	if err != nil {
		return PermissionReadinessResult{}, err
	}

	capabilities := accessibility.Capabilities()
	return PermissionReadinessResult{
		Permissions: permissionSnapshotResultFromCommon(snapshot),
		Capabilities: []FeatureStatus{
			capabilityFeatureStatus(capabilities.Status(common.CapabilityPermissionReadiness), "available through the native accessibility permission readiness backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXFocusedWindowMetadata), "available through the native focused-window accessibility metadata backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXFocusedElementMetadata), "available through the native focused-element accessibility metadata backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXElementAtPointMetadata), "available through the native element-at-point accessibility metadata backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXFocusedWindowRaise), "available through the native focused-window accessibility action backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXFocusedElementAction), "available through the native focused-element accessibility action backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXElementActionAtPoint), "available through the native element-at-point accessibility action backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXElementFocusAtPoint), "available through the native element-at-point accessibility focus backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXElementSearch), "available through the native AX element search backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXElementFocusMatch), "available through the native AX element ref focus backend"),
			capabilityFeatureStatus(capabilities.Status(common.CapabilityAXElementActionMatch), "available through the native AX element ref action backend"),
		},
		Message: "These accessibility capability entries back the AX metadata and action tools.",
	}, nil
}

func (s *Service) GetFocusedWindowMetadata() (FocusedWindowMetadataResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return FocusedWindowMetadataResult{}, err
	}

	metadata, err := accessibility.GetFocusedWindow(ctx)
	if err != nil {
		return FocusedWindowMetadataResult{}, err
	}
	return focusedWindowMetadataResultFromCommon(metadata), nil
}

func (s *Service) GetFocusedElementMetadata() (UIElementMetadataResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return UIElementMetadataResult{}, err
	}

	metadata, err := accessibility.GetFocusedElement(ctx)
	if err != nil {
		return UIElementMetadataResult{}, err
	}
	return uiElementMetadataResultFromCommon(metadata), nil
}

func (s *Service) GetElementAtPointMetadata(req PointRequest) (UIElementMetadataResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return UIElementMetadataResult{}, err
	}

	metadata, err := accessibility.GetElementAtPoint(ctx, shared.Point{X: req.X, Y: req.Y})
	if err != nil {
		return UIElementMetadataResult{}, err
	}
	return uiElementMetadataResultFromCommon(metadata), nil
}

func (s *Service) RaiseFocusedWindow() (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return ActionResult{}, err
	}
	if err := accessibility.RaiseFocusedWindow(ctx); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: "Raised focused window"}, nil
}

func (s *Service) PerformFocusedElementAction(req AXActionRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	action, err := parseAXAction(req.Action)
	if err != nil {
		return ActionResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return ActionResult{}, err
	}
	if err := accessibility.PerformFocusedElementAction(ctx, action); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Performed %s on focused element", action)}, nil
}

func (s *Service) PerformElementActionAtPoint(req AXActionAtPointRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	action, err := parseAXAction(req.Action)
	if err != nil {
		return ActionResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return ActionResult{}, err
	}
	if err := accessibility.PerformElementActionAtPoint(ctx, shared.Point{X: req.X, Y: req.Y}, action); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Performed %s at (%d, %d)", action, req.X, req.Y)}, nil
}

func (s *Service) FocusElementAtPoint(req PointRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return ActionResult{}, err
	}
	if err := accessibility.FocusElementAtPoint(ctx, shared.Point{X: req.X, Y: req.Y}); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Focused element at (%d, %d)", req.X, req.Y)}, nil
}

func (s *Service) SearchAXElements(req SearchAXElementsRequest) (SearchAXElementsResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query, err := parseAXElementSearchQuery(req)
	if err != nil {
		return SearchAXElementsResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return SearchAXElementsResult{}, err
	}
	matches, err := accessibility.SearchAXElements(ctx, query)
	if err != nil {
		return SearchAXElementsResult{}, err
	}
	resultMatches := make([]AXElementMatchResult, 0, len(matches))
	for _, match := range matches {
		resultMatches = append(resultMatches, axElementMatchResultFromCommon(match))
	}
	return SearchAXElementsResult{
		Query:   searchAXElementsRequestFromCommon(query),
		Matches: resultMatches,
		Message: fmt.Sprintf("Found %d AX element matches.", len(resultMatches)),
	}, nil
}

func (s *Service) FocusAXElement(req FocusAXElementRequest) (FocusAXElementResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ref, err := parseAXElementRef(req.Ref)
	if err != nil {
		return FocusAXElementResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return FocusAXElementResult{}, err
	}
	if err := accessibility.FocusAXElement(ctx, ref); err != nil {
		return FocusAXElementResult{}, err
	}
	return FocusAXElementResult{OK: true, Ref: axElementRefResultFromCommon(ref), Message: "Focused AX element by ref"}, nil
}

func (s *Service) PerformAXElementAction(req PerformAXElementActionOnRefRequest) (PerformAXElementActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ref, err := parseAXElementRef(req.Ref)
	if err != nil {
		return PerformAXElementActionResult{}, err
	}
	action, err := parseAXAction(req.Action)
	if err != nil {
		return PerformAXElementActionResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	accessibility, err := s.nut.Registry.Accessibility()
	if err != nil {
		return PerformAXElementActionResult{}, err
	}
	if err := accessibility.PerformAXElementAction(ctx, ref, action); err != nil {
		return PerformAXElementActionResult{}, err
	}
	return PerformAXElementActionResult{
		OK:      true,
		Ref:     axElementRefResultFromCommon(ref),
		Action:  string(action),
		Message: fmt.Sprintf("Performed %s on AX element ref", action),
	}, nil
}

func (s *Service) TypeText(req KeyboardTextRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Keyboard.SetAutoDelay(agentKeyboardAutoDelay)
	if err := s.nut.Keyboard.TypeText(ctx, req.Text); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Typed %d characters", len(req.Text))}, nil
}

func (s *Service) TapKeys(req KeyboardKeysRequest) (ActionResult, error) {
	if shouldUseDarwinShortcutPath(req.Keys) {
		return s.PressShortcut(KeyboardShortcutRequest{Keys: append([]string(nil), req.Keys...)})
	}
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

func (s *Service) PressShortcut(req KeyboardShortcutRequest) (ActionResult, error) {
	keys := make([]string, 0, len(req.Keys))
	for _, item := range req.Keys {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	if len(keys) == 0 {
		return ActionResult{}, fmt.Errorf("at least one key is required")
	}

	if currentRuntimeGOOS != "darwin" {
		return s.TapKeys(KeyboardKeysRequest{Keys: keys})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	script, err := darwinShortcutScript(keys)
	if err != nil {
		return ActionResult{}, err
	}
	if err := executeDarwinAppleScript(ctx, script); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("Pressed shortcut %s", strings.Join(keys, " + "))}, nil
}

func (s *Service) PressSpecialKey(req KeyboardSpecialKeyRequest) (ActionResult, error) {
	if currentRuntimeGOOS != "darwin" {
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

	if err := executeDarwinAppleScript(ctx, script); err != nil {
		return ActionResult{}, err
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

	s.nut.Mouse.SetAutoDelay(agentMouseAutoDelay)
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

	s.nut.Mouse.SetAutoDelay(agentMouseAutoDelay)
	s.nut.Mouse.SetSpeed(agentMouseSpeed)
	path, err := s.nut.Mouse.StraightTo(ctx, shared.Point{X: req.X, Y: req.Y})
	if err != nil {
		return ActionResult{}, err
	}
	path = optimizeMousePath(path)
	if len(path) <= 1 {
		if err := s.nut.Mouse.SetPosition(ctx, shared.Point{X: req.X, Y: req.Y}); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{OK: true, Message: fmt.Sprintf("Moved pointer to (%d, %d)", req.X, req.Y)}, nil
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

func (s *Service) MouseDown(req MouseButtonRequest) (ActionResult, error) {
	return s.runMouseButton(req, func(ctx context.Context, button shared.Button) error {
		return s.nut.Mouse.PressButton(ctx, button)
	}, "Pressed")
}

func (s *Service) MouseUp(req MouseButtonRequest) (ActionResult, error) {
	return s.runMouseButton(req, func(ctx context.Context, button shared.Button) error {
		return s.nut.Mouse.ReleaseButton(ctx, button)
	}, "Released")
}

func (s *Service) ScrollMouse(req MouseScrollRequest) (ActionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nut.Mouse.SetAutoDelay(agentMouseAutoDelay)
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

	s.nut.Mouse.SetAutoDelay(agentMouseAutoDelay)
	s.nut.Mouse.SetSpeed(agentMouseSpeed)
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
		return s.captureWithCommand(req.FileName, Point{}, Size{}, req.MaxImageWidth, req.MaxImageHeight)
	}
	screenSize, err := s.GetScreenSize()
	if err != nil {
		return CaptureResult{}, err
	}
	return s.capture(func(ctx context.Context, path string) (string, error) {
		return s.nut.Screen.Capture(ctx, path)
	}, req.FileName, Point{}, Size{Width: screenSize.Width, Height: screenSize.Height}, req.MaxImageWidth, req.MaxImageHeight)
}

func (s *Service) CaptureRegion(req CaptureRegionRequest) (CaptureResult, error) {
	if runtime.GOOS == "darwin" {
		return s.captureRegionWithCommand(req.FileName, req.Region, req.MaxImageWidth, req.MaxImageHeight)
	}
	region := shared.Region{
		Left:   req.Region.Left,
		Top:    req.Region.Top,
		Width:  req.Region.Width,
		Height: req.Region.Height,
	}
	return s.capture(func(ctx context.Context, path string) (string, error) {
		return s.nut.Screen.CaptureRegion(ctx, path, region)
	}, req.FileName, Point{X: req.Region.Left, Y: req.Region.Top}, Size{Width: req.Region.Width, Height: req.Region.Height}, req.MaxImageWidth, req.MaxImageHeight)
}

func (s *Service) CaptureActiveWindow(req CaptureRequest) (CaptureResult, error) {
	window, err := s.GetActiveWindow()
	if err != nil {
		return CaptureResult{}, err
	}
	return s.CaptureRegion(CaptureRegionRequest{
		FileName:       req.FileName,
		Region:         window.Region,
		MaxImageWidth:  req.MaxImageWidth,
		MaxImageHeight: req.MaxImageHeight,
	})
}

func (s *Service) CaptureWindow(req CaptureWindowRequest) (CaptureResult, error) {
	window, err := s.FindWindowByHandle(WindowHandleRequest{Handle: req.Handle})
	if err != nil {
		return CaptureResult{}, err
	}
	return s.CaptureRegion(CaptureRegionRequest{
		FileName:       req.FileName,
		Region:         window.Region,
		MaxImageWidth:  req.MaxImageWidth,
		MaxImageHeight: req.MaxImageHeight,
	})
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

func (s *Service) FindWindowByHandle(req WindowHandleRequest) (WindowSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	window, err := s.windowByHandle(ctx, req.Handle)
	if err != nil {
		return WindowSummary{}, err
	}
	return s.windowSummary(ctx, window)
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

func (s *Service) capture(run func(context.Context, string) (string, error), fileName string, offset Point, logicalSize Size, maxImageWidth int, maxImageHeight int) (CaptureResult, error) {
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
	originalSize, err := captureImageSize(savedPath)
	if err != nil {
		return CaptureResult{}, err
	}
	deliveredSize, err := maybeDownscaleCapture(savedPath, originalSize, maxImageWidth, maxImageHeight)
	if err != nil {
		return CaptureResult{}, err
	}
	result := captureResultForImage(savedPath, fmt.Sprintf("Saved capture to %s", savedPath), offset, logicalSize, originalSize, deliveredSize)
	if err := writeCaptureMetadata(result); err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

func (s *Service) captureWithCommand(fileName string, offset Point, logicalSize Size, maxImageWidth int, maxImageHeight int) (CaptureResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.capturePath(fileName)
	if err != nil {
		return CaptureResult{}, err
	}

	if err := runScreencapture(ctx, path); err != nil {
		return CaptureResult{}, err
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

	originalSize, err := captureImageSize(path)
	if err != nil {
		return CaptureResult{}, err
	}
	deliveredSize, err := maybeDownscaleCapture(path, originalSize, maxImageWidth, maxImageHeight)
	if err != nil {
		return CaptureResult{}, err
	}
	result := captureResultForImage(path, fmt.Sprintf("Saved capture to %s", path), offset, logicalSize, originalSize, deliveredSize)
	if err := writeCaptureMetadata(result); err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

func (s *Service) captureRegionWithCommand(fileName string, region Region, maxImageWidth int, maxImageHeight int) (CaptureResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.capturePath(fileName)
	if err != nil {
		return CaptureResult{}, err
	}

	width, err := s.nut.Screen.Width(ctx)
	if err != nil {
		return CaptureResult{}, err
	}
	height, err := s.nut.Screen.Height(ctx)
	if err != nil {
		return CaptureResult{}, err
	}
	screenSize := Size{Width: width, Height: height}

	tempFile, err := os.CreateTemp(filepath.Dir(path), "capture-full-*.png")
	if err != nil {
		return CaptureResult{}, err
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return CaptureResult{}, err
	}
	defer os.Remove(tempPath)

	if err := runScreencapture(ctx, tempPath); err != nil {
		return CaptureResult{}, err
	}

	fullScale := captureScale(tempPath, screenSize)
	if err := cropCapturedRegion(tempPath, path, region, fullScale); err != nil {
		return CaptureResult{}, err
	}

	originalSize, err := captureImageSize(path)
	if err != nil {
		return CaptureResult{}, err
	}
	deliveredSize, err := maybeDownscaleCapture(path, originalSize, maxImageWidth, maxImageHeight)
	if err != nil {
		return CaptureResult{}, err
	}
	result := captureRegionResult(path, region, fullScale, originalSize, deliveredSize)
	if err := writeCaptureMetadata(result); err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

func captureScale(path string, logicalSize Size) Scale {
	imageSize, err := captureImageSize(path)
	if err != nil {
		return Scale{X: 1, Y: 1}
	}
	return captureScaleForSize(imageSize, logicalSize)
}

func captureScaleForSize(imageSize Size, logicalSize Size) Scale {
	if logicalSize.Width <= 0 || logicalSize.Height <= 0 {
		return Scale{X: 1, Y: 1}
	}

	scaleX := float64(imageSize.Width) / float64(logicalSize.Width)
	scaleY := float64(imageSize.Height) / float64(logicalSize.Height)
	return normalizedCaptureScale(Scale{X: scaleX, Y: scaleY})
}

func captureImageSize(path string) (Size, error) {
	file, err := os.Open(path)
	if err != nil {
		return Size{}, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return Size{}, err
	}
	return Size{Width: config.Width, Height: config.Height}, nil
}

func writeCaptureMetadata(result CaptureResult) error {
	result = normalizeCaptureResultMetadata(result)
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
	if strings.TrimSpace(result.Path) == "" {
		result.Path = path
	}
	return normalizeCaptureResultMetadata(result), nil
}

func captureMetadataPath(path string) string {
	return path + ".json"
}

func runScreencapture(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "screencapture", screencaptureArgs("", path)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, message)
	}
	return nil
}

func cropCapturedRegion(sourcePath string, destPath string, region Region, scale Scale) error {
	if region.Width <= 0 || region.Height <= 0 {
		return fmt.Errorf("region width and height must be positive")
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceImage, _, err := image.Decode(sourceFile)
	if err != nil {
		return err
	}

	cropBounds, err := captureCropBounds(region, scale, sourceImage.Bounds())
	if err != nil {
		return err
	}

	cropped := image.NewRGBA(image.Rect(0, 0, cropBounds.Dx(), cropBounds.Dy()))
	draw.Draw(cropped, cropped.Bounds(), sourceImage, cropBounds.Min, draw.Src)
	return encodeCaptureImage(destPath, cropped)
}

func captureRegionResult(path string, region Region, originalScale Scale, originalSize Size, deliveredSize Size) CaptureResult {
	return captureResultForImage(
		path,
		fmt.Sprintf("Saved capture to %s", path),
		Point{X: region.Left, Y: region.Top},
		Size{Width: region.Width, Height: region.Height},
		originalSize,
		deliveredSize,
	).withOriginalScale(originalScale)
}

func normalizedCaptureScale(scale Scale) Scale {
	if scale.X <= 0 {
		scale.X = 1
	}
	if scale.Y <= 0 {
		scale.Y = 1
	}
	return scale
}

func normalizedCaptureSize(size Size) Size {
	if size.Width < 0 {
		size.Width = 0
	}
	if size.Height < 0 {
		size.Height = 0
	}
	return size
}

func captureResultForImage(path string, message string, offset Point, logicalSize Size, originalSize Size, deliveredSize Size) CaptureResult {
	originalSize = normalizedCaptureSize(originalSize)
	deliveredSize = normalizedCaptureSize(deliveredSize)
	originalScale := captureScaleForSize(originalSize, logicalSize)
	result := CaptureResult{
		Path:          path,
		Message:       message,
		Offset:        offset,
		OriginalSize:  originalSize,
		DeliveredSize: deliveredSize,
		OriginalScale: originalScale,
		Scale:         deliveredCaptureScale(originalScale, originalSize, deliveredSize),
	}
	return normalizeCaptureResultMetadata(result)
}

func (result CaptureResult) withOriginalScale(originalScale Scale) CaptureResult {
	result.OriginalScale = normalizedCaptureScale(originalScale)
	result.Scale = deliveredCaptureScale(result.OriginalScale, result.OriginalSize, result.DeliveredSize)
	return normalizeCaptureResultMetadata(result)
}

func normalizeCaptureResultMetadata(result CaptureResult) CaptureResult {
	result.Path = strings.TrimSpace(result.Path)
	result.OriginalSize = normalizedCaptureSize(result.OriginalSize)
	result.DeliveredSize = normalizedCaptureSize(result.DeliveredSize)

	if result.OriginalSize.Width == 0 || result.OriginalSize.Height == 0 || result.DeliveredSize.Width == 0 || result.DeliveredSize.Height == 0 {
		if size, err := captureImageSize(result.Path); err == nil {
			if result.OriginalSize.Width == 0 || result.OriginalSize.Height == 0 {
				result.OriginalSize = size
			}
			if result.DeliveredSize.Width == 0 || result.DeliveredSize.Height == 0 {
				result.DeliveredSize = size
			}
		}
	}
	if result.OriginalSize.Width == 0 || result.OriginalSize.Height == 0 {
		result.OriginalSize = result.DeliveredSize
	}
	if result.DeliveredSize.Width == 0 || result.DeliveredSize.Height == 0 {
		result.DeliveredSize = result.OriginalSize
	}

	deliveredRatioX, deliveredRatioY := deliveredSizeRatio(result.OriginalSize, result.DeliveredSize)
	result.Scale = normalizedCaptureScale(result.Scale)
	if result.OriginalScale.X <= 0 {
		result.OriginalScale.X = result.Scale.X
		if deliveredRatioX > 0 {
			result.OriginalScale.X = result.Scale.X / deliveredRatioX
		}
	}
	if result.OriginalScale.Y <= 0 {
		result.OriginalScale.Y = result.Scale.Y
		if deliveredRatioY > 0 {
			result.OriginalScale.Y = result.Scale.Y / deliveredRatioY
		}
	}
	result.OriginalScale = normalizedCaptureScale(result.OriginalScale)
	result.Scale = deliveredCaptureScale(result.OriginalScale, result.OriginalSize, result.DeliveredSize)
	return result
}

func deliveredCaptureScale(originalScale Scale, originalSize Size, deliveredSize Size) Scale {
	originalScale = normalizedCaptureScale(originalScale)
	deliveredRatioX, deliveredRatioY := deliveredSizeRatio(originalSize, deliveredSize)
	return normalizedCaptureScale(Scale{
		X: originalScale.X * deliveredRatioX,
		Y: originalScale.Y * deliveredRatioY,
	})
}

func deliveredSizeRatio(originalSize Size, deliveredSize Size) (float64, float64) {
	if originalSize.Width <= 0 || originalSize.Height <= 0 || deliveredSize.Width <= 0 || deliveredSize.Height <= 0 {
		return 1, 1
	}
	return float64(deliveredSize.Width) / float64(originalSize.Width), float64(deliveredSize.Height) / float64(originalSize.Height)
}

func planCaptureDeliveredSize(originalSize Size, maxImageWidth int, maxImageHeight int) Size {
	originalSize = normalizedCaptureSize(originalSize)
	if originalSize.Width <= 0 || originalSize.Height <= 0 {
		return Size{}
	}
	if maxImageWidth <= 0 && maxImageHeight <= 0 {
		return originalSize
	}

	scaleFactor := 1.0
	if maxImageWidth > 0 && originalSize.Width > maxImageWidth {
		scaleFactor = math.Min(scaleFactor, float64(maxImageWidth)/float64(originalSize.Width))
	}
	if maxImageHeight > 0 && originalSize.Height > maxImageHeight {
		scaleFactor = math.Min(scaleFactor, float64(maxImageHeight)/float64(originalSize.Height))
	}
	if scaleFactor >= 1 {
		return originalSize
	}

	return Size{
		Width:  max(1, int(math.Floor(float64(originalSize.Width)*scaleFactor))),
		Height: max(1, int(math.Floor(float64(originalSize.Height)*scaleFactor))),
	}
}

func maybeDownscaleCapture(path string, originalSize Size, maxImageWidth int, maxImageHeight int) (Size, error) {
	targetSize := planCaptureDeliveredSize(originalSize, maxImageWidth, maxImageHeight)
	if targetSize == originalSize {
		return originalSize, nil
	}
	if err := resizeCaptureImage(path, targetSize); err != nil {
		return Size{}, err
	}
	return targetSize, nil
}

func resizeCaptureImage(path string, targetSize Size) error {
	targetSize = normalizedCaptureSize(targetSize)
	if targetSize.Width <= 0 || targetSize.Height <= 0 {
		return fmt.Errorf("target size must be positive")
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(file)
	file.Close()
	if err != nil {
		return err
	}

	return encodeCaptureImage(path, resizeImageNearestNeighbor(img, targetSize))
}

func resizeImageNearestNeighbor(source image.Image, targetSize Size) image.Image {
	target := image.NewRGBA(image.Rect(0, 0, targetSize.Width, targetSize.Height))
	sourceBounds := source.Bounds()
	sourceWidth := sourceBounds.Dx()
	sourceHeight := sourceBounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return target
	}

	for y := 0; y < targetSize.Height; y++ {
		sourceY := sourceBounds.Min.Y + min(sourceHeight-1, int(float64(y)*float64(sourceHeight)/float64(targetSize.Height)))
		for x := 0; x < targetSize.Width; x++ {
			sourceX := sourceBounds.Min.X + min(sourceWidth-1, int(float64(x)*float64(sourceWidth)/float64(targetSize.Width)))
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return target
}

func translateDeliveredImagePointToScreen(capture CaptureResult, deliveredPoint Point) (ImagePointTranslationResult, error) {
	capture = normalizeCaptureResultMetadata(capture)
	if strings.TrimSpace(capture.Path) == "" {
		return ImagePointTranslationResult{}, fmt.Errorf("capture path is required")
	}
	if deliveredPoint.X < 0 || deliveredPoint.Y < 0 {
		return ImagePointTranslationResult{}, fmt.Errorf("image coordinates must be non-negative")
	}
	if capture.DeliveredSize.Width <= 0 || capture.DeliveredSize.Height <= 0 {
		return ImagePointTranslationResult{}, fmt.Errorf("capture metadata is missing delivered image size")
	}
	if capture.OriginalSize.Width <= 0 || capture.OriginalSize.Height <= 0 {
		return ImagePointTranslationResult{}, fmt.Errorf("capture metadata is missing original image size")
	}
	if deliveredPoint.X >= capture.DeliveredSize.Width || deliveredPoint.Y >= capture.DeliveredSize.Height {
		return ImagePointTranslationResult{}, fmt.Errorf("image point (%d, %d) is outside delivered image bounds %dx%d", deliveredPoint.X, deliveredPoint.Y, capture.DeliveredSize.Width, capture.DeliveredSize.Height)
	}
	if capture.OriginalScale.X <= 0 || capture.OriginalScale.Y <= 0 {
		return ImagePointTranslationResult{}, fmt.Errorf("capture metadata is missing original image scale")
	}

	originalPoint := ImagePoint{
		X: float64(deliveredPoint.X) * float64(capture.OriginalSize.Width) / float64(capture.DeliveredSize.Width),
		Y: float64(deliveredPoint.Y) * float64(capture.OriginalSize.Height) / float64(capture.DeliveredSize.Height),
	}
	exactScreenPoint := ImagePoint{
		X: float64(capture.Offset.X) + originalPoint.X/capture.OriginalScale.X,
		Y: float64(capture.Offset.Y) + originalPoint.Y/capture.OriginalScale.Y,
	}
	absoluteScreenPoint := Point{
		X: int(math.Round(exactScreenPoint.X)),
		Y: int(math.Round(exactScreenPoint.Y)),
	}
	return ImagePointTranslationResult{
		Path:                    capture.Path,
		RequestedDeliveredPoint: deliveredPoint,
		OriginalImagePoint:      originalPoint,
		ExactScreenPoint:        exactScreenPoint,
		AbsoluteScreenPoint:     absoluteScreenPoint,
		Capture:                 capture,
		Message:                 fmt.Sprintf("Translated delivered image point (%d, %d) to absolute screen point (%d, %d).", deliveredPoint.X, deliveredPoint.Y, absoluteScreenPoint.X, absoluteScreenPoint.Y),
	}, nil
}

func (s *Service) TranslateImagePointToScreen(path string, deliveredPoint Point) (ImagePointTranslationResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ImagePointTranslationResult{}, fmt.Errorf("path is required")
	}
	capture, err := readCaptureMetadata(path)
	if err != nil {
		return ImagePointTranslationResult{}, err
	}
	return translateDeliveredImagePointToScreen(capture, deliveredPoint)
}

func captureCropBounds(region Region, scale Scale, sourceBounds image.Rectangle) (image.Rectangle, error) {
	if region.Width <= 0 || region.Height <= 0 {
		return image.Rectangle{}, fmt.Errorf("region width and height must be positive")
	}
	scale = normalizedCaptureScale(scale)

	left := scaledPixelStart(region.Left, scale.X)
	top := scaledPixelStart(region.Top, scale.Y)
	right := scaledPixelEnd(region.Left+region.Width, scale.X)
	bottom := scaledPixelEnd(region.Top+region.Height, scale.Y)
	cropBounds := image.Rect(left, top, right, bottom)
	if cropBounds.Empty() {
		return image.Rectangle{}, fmt.Errorf("region %v produced an empty crop", region)
	}
	if !cropBounds.In(sourceBounds) {
		return image.Rectangle{}, fmt.Errorf("region %v is outside captured screen bounds %v", region, sourceBounds)
	}
	return cropBounds, nil
}

func scaledPixelStart(value int, scale float64) int {
	return int(math.Floor(float64(value) * scale))
}

func scaledPixelEnd(value int, scale float64) int {
	return int(math.Ceil(float64(value) * scale))
}

func encodeCaptureImage(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
	case ".gif":
		return gif.Encode(file, img, nil)
	default:
		return png.Encode(file, img)
	}
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

func darwinShortcutScript(keys []string) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("at least one key is required")
	}
	modifiers := make([]string, 0, len(keys)-1)
	nonModifiers := make([]string, 0, len(keys))
	for _, item := range keys {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "cmd", "command", "meta", "super":
			modifiers = append(modifiers, "command down")
		case "ctrl", "control":
			modifiers = append(modifiers, "control down")
		case "alt", "option":
			modifiers = append(modifiers, "option down")
		case "shift":
			modifiers = append(modifiers, "shift down")
		default:
			nonModifiers = append(nonModifiers, strings.ToLower(strings.TrimSpace(item)))
		}
	}
	if len(nonModifiers) != 1 {
		return "", fmt.Errorf("shortcut requires exactly one non-modifier key")
	}

	trigger := nonModifiers[0]
	modifierClause := ""
	if len(modifiers) > 0 {
		modifierClause = " using {" + strings.Join(modifiers, ", ") + "}"
	}
	if len(trigger) == 1 {
		return fmt.Sprintf(`tell application "System Events"
	keystroke %q%s
end tell`, trigger, modifierClause), nil
	}
	if keyCode, ok := darwinSpecialKeyCode(trigger); ok {
		return fmt.Sprintf(`tell application "System Events"
	key code %d%s
end tell`, keyCode, modifierClause), nil
	}
	return "", fmt.Errorf("unsupported shortcut key %q", trigger)
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
	s.nut.Keyboard.SetAutoDelay(agentKeyboardAutoDelay)
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
	s.nut.Mouse.SetAutoDelay(agentMouseAutoDelay)
	if err := run(ctx, button); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{OK: true, Message: fmt.Sprintf("%s %s button", verb, req.Button)}, nil
}

func (s *Service) windowByHandle(ctx context.Context, handle uint64) (*gut.Window, error) {
	windows, err := gut.GetWindows(ctx, s.nut.Registry)
	if err != nil {
		return nil, err
	}
	for _, window := range windows {
		if uint64(window.Handle) == handle {
			return window, nil
		}
	}
	return nil, fmt.Errorf("%w: %d", errWindowHandleNotFound, handle)
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

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func markdownInline(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "unnamed"
	}
	text = strings.ReplaceAll(text, "\n", " ")
	return strings.Join(strings.Fields(text), " ")
}

func backtickJoin(values []string) string {
	formatted := make([]string, 0, len(values))
	for _, value := range values {
		formatted = append(formatted, fmt.Sprintf("`%s`", value))
	}
	return strings.Join(formatted, ", ")
}

func featureStatusesForGOOS(goos string, report guttesting.Report) []FeatureStatus {
	statuses := make([]FeatureStatus, 0, len(report.CapabilityStatuses())+15)
	for _, status := range report.CapabilityStatuses() {
		statuses = append(statuses, capabilityFeatureStatus(status, ""))
	}

	if goos == "darwin" {
		statuses = append(statuses,
			availableFeatureStatus("capture_screen", "available via the macOS safe screenshot fallback; native screen.capture is intentionally not required"),
			availableFeatureStatus("capture_region", "available via the macOS safe screenshot fallback; native screen.capture is intentionally not required"),
			availableFeatureStatus("capture_active_window", "available via the macOS safe screenshot fallback using the active window bounds"),
			availableFeatureStatus("capture_window", "available via the macOS safe screenshot fallback using a whole window's bounds"),
			availableFeatureStatus("press_special_key", "available via the macOS System Events special-key path"),
			availableFeatureStatus("tap_keys", "available via the macOS System Events safe shortcut path"),
			availableFeatureStatus("press_keys", "available through the native keyboard.toggle backend"),
			availableFeatureStatus("release_keys", "available through the native keyboard.toggle backend"),
			unavailableFeatureStatus("highlight_region", "intentionally hidden on macOS 26+; use whole-window or full-screen captures instead"),
		)
	} else {
		statuses = append(statuses,
			toolStatusFromCapability("capture_screen", report.Capabilities.Status(common.CapabilityScreenCapture), "available through the native screen.capture backend"),
			toolStatusFromCapability("capture_region", report.Capabilities.Status(common.CapabilityScreenCapture), "available through the native screen.capture backend"),
			toolStatusFromCapability("capture_active_window", report.Capabilities.Status(common.CapabilityScreenCapture), "available through the native capture backend using the active window bounds"),
			toolStatusFromCapability("capture_window", report.Capabilities.Status(common.CapabilityScreenCapture), "available through the native capture backend using a whole window's bounds"),
			unavailableFeatureStatus("press_special_key", "only exposed on darwin as the macOS safe special-key path"),
			toolStatusFromCapability("tap_keys", report.Capabilities.Status(common.CapabilityKeyboardTap), "available through the native keyboard.tap backend"),
			toolStatusFromCapability("press_keys", report.Capabilities.Status(common.CapabilityKeyboardToggle), "available through the native keyboard.toggle backend"),
			toolStatusFromCapability("release_keys", report.Capabilities.Status(common.CapabilityKeyboardToggle), "available through the native keyboard.toggle backend"),
			toolStatusFromCapability("highlight_region", report.Capabilities.Status(common.CapabilityScreenHighlight), "available through the native screen.highlight backend"),
		)
	}

	statuses = append(statuses,
		toolStatusFromCapability("get_permission_readiness", report.Capabilities.Status(common.CapabilityPermissionReadiness), "available through the native accessibility permission readiness backend"),
		toolStatusFromCapability("get_focused_window_metadata", report.Capabilities.Status(common.CapabilityAXFocusedWindowMetadata), "available through the native focused-window accessibility metadata backend"),
		toolStatusFromCapability("get_focused_element_metadata", report.Capabilities.Status(common.CapabilityAXFocusedElementMetadata), "available through the native focused-element accessibility metadata backend"),
		toolStatusFromCapability("get_element_at_point_metadata", report.Capabilities.Status(common.CapabilityAXElementAtPointMetadata), "available through the native element-at-point accessibility metadata backend"),
		toolStatusFromCapability("raise_focused_window", report.Capabilities.Status(common.CapabilityAXFocusedWindowRaise), "available through the native focused-window accessibility action backend"),
		toolStatusFromCapability("perform_focused_element_action", report.Capabilities.Status(common.CapabilityAXFocusedElementAction), "available through the native focused-element accessibility action backend"),
		toolStatusFromCapability("perform_element_action_at_point", report.Capabilities.Status(common.CapabilityAXElementActionAtPoint), "available through the native element-at-point accessibility action backend"),
		toolStatusFromCapability("focus_element_at_point", report.Capabilities.Status(common.CapabilityAXElementFocusAtPoint), "available through the native element-at-point accessibility focus backend"),
		toolStatusFromCapability("search_ax_elements", report.Capabilities.Status(common.CapabilityAXElementSearch), "available through the native AX element search backend"),
		toolStatusFromCapability("focus_ax_element", report.Capabilities.Status(common.CapabilityAXElementFocusMatch), "available through the native AX element ref focus backend"),
		toolStatusFromCapability("perform_ax_element_action", report.Capabilities.Status(common.CapabilityAXElementActionMatch), "available through the native AX element ref action backend"),
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
	feature := capabilityFeatureStatus(status, availableReason)
	feature.ID = id
	return feature
}

func capabilityFeatureStatus(status common.CapabilityStatus, availableReason string) FeatureStatus {
	reason := strings.TrimSpace(status.Reason)
	if status.Availability == common.AvailabilityAvailable && strings.TrimSpace(availableReason) != "" {
		reason = availableReason
	}
	if reason == "" {
		reason = fmt.Sprintf("native %s is %s", status.Capability, status.Availability)
	}
	return FeatureStatus{
		ID:           string(status.Capability),
		Availability: string(status.Availability),
		Reason:       reason,
	}
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

func optimizeMousePath(path []shared.Point) []shared.Point {
	switch {
	case len(path) <= 1:
		return path
	case len(path) <= agentMouseInstantPathThreshold:
		return path[len(path)-1:]
	case len(path) <= agentMouseMaxPathPoints:
		return path
	default:
		return sampleMousePath(path, agentMouseMaxPathPoints)
	}
}

func sampleMousePath(path []shared.Point, maxPoints int) []shared.Point {
	if len(path) <= maxPoints || maxPoints < 2 {
		return path
	}

	sampled := make([]shared.Point, 0, maxPoints)
	lastIndex := len(path) - 1
	for index := 0; index < maxPoints; index++ {
		position := int(math.Round(float64(index*lastIndex) / float64(maxPoints-1)))
		sampled = append(sampled, path[position])
	}
	return sampled
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

func permissionStatusResultFromCommon(status common.PermissionStatus) PermissionStatusResult {
	return PermissionStatusResult{
		Granted:   status.Granted,
		Supported: status.Supported,
		Reason:    status.Reason,
	}
}

func permissionSnapshotResultFromCommon(snapshot common.PermissionSnapshot) PermissionSnapshotResult {
	return PermissionSnapshotResult{
		Accessibility:   permissionStatusResultFromCommon(snapshot.Accessibility),
		ScreenRecording: permissionStatusResultFromCommon(snapshot.ScreenRecording),
	}
}

func focusedWindowMetadataResultFromCommon(metadata common.FocusedWindowMetadata) FocusedWindowMetadataResult {
	return FocusedWindowMetadataResult{
		Handle:      uint64(metadata.Handle),
		Title:       metadata.Title,
		Role:        metadata.Role,
		Subrole:     metadata.Subrole,
		Region:      regionFromRect(metadata.Rect),
		RegionKnown: metadata.RectKnown,
		Focused:     metadata.Focused,
		Main:        metadata.Main,
		Minimized:   metadata.Minimized,
		OwnerPID:    metadata.OwnerPID,
		OwnerName:   metadata.OwnerName,
		BundleID:    metadata.BundleID,
	}
}

func uiElementMetadataResultFromCommon(metadata common.UIElementMetadata) UIElementMetadataResult {
	actions := append([]string(nil), metadata.Actions...)
	return UIElementMetadataResult{
		Role:        metadata.Role,
		Subrole:     metadata.Subrole,
		Title:       metadata.Title,
		Description: metadata.Description,
		Value:       metadata.Value,
		Enabled:     metadata.Enabled,
		Focused:     metadata.Focused,
		Frame:       regionFromRect(metadata.Frame),
		FrameKnown:  metadata.FrameKnown,
		Actions:     actions,
	}
}

func searchAXElementsRequestFromCommon(query common.AXElementSearchQuery) SearchAXElementsRequest {
	return SearchAXElementsRequest{
		Scope:               string(query.Scope),
		WindowHandle:        uint64(query.WindowHandle),
		Role:                query.Role,
		Subrole:             query.Subrole,
		TitleContains:       query.TitleContains,
		ValueContains:       query.ValueContains,
		DescriptionContains: query.DescriptionContains,
		Action:              query.Action,
		Enabled:             query.Enabled,
		Focused:             query.Focused,
		Limit:               query.Limit,
		MaxDepth:            query.MaxDepth,
	}
}

func axElementRefResultFromCommon(ref common.AXElementRef) AXElementRefResult {
	return AXElementRefResult{
		Scope:        string(ref.Scope),
		OwnerPID:     ref.OwnerPID,
		WindowHandle: uint64(ref.WindowHandle),
		Path:         append([]int(nil), ref.Path...),
	}
}

func axElementMatchResultFromCommon(match common.AXElementMatch) AXElementMatchResult {
	return AXElementMatchResult{
		Ref:              axElementRefResultFromCommon(match.Ref),
		Metadata:         uiElementMetadataResultFromCommon(match.Metadata),
		Depth:            match.Depth,
		ActionPoint:      pointFromCommon(match.ActionPoint),
		ActionPointKnown: match.ActionPointKnown,
	}
}

func pointFromCommon(point common.Point) Point {
	return Point{X: point.X, Y: point.Y}
}

func parseAXSearchScope(value string) (common.AXSearchScope, error) {
	scope := common.AXSearchScope(strings.TrimSpace(value))
	switch scope {
	case common.AXSearchScopeFocusedWindow, common.AXSearchScopeFrontmostApplication, common.AXSearchScopeWindowHandle:
		return scope, nil
	default:
		return "", fmt.Errorf("unsupported AX search scope %q", value)
	}
}

func parseAXElementSearchQuery(req SearchAXElementsRequest) (common.AXElementSearchQuery, error) {
	scope, err := parseAXSearchScope(req.Scope)
	if err != nil {
		return common.AXElementSearchQuery{}, err
	}
	if req.Limit <= 0 {
		return common.AXElementSearchQuery{}, fmt.Errorf("limit must be > 0")
	}
	if req.MaxDepth < 0 {
		return common.AXElementSearchQuery{}, fmt.Errorf("max_depth must be >= 0")
	}
	if scope == common.AXSearchScopeWindowHandle && req.WindowHandle == 0 {
		return common.AXElementSearchQuery{}, fmt.Errorf("window_handle is required for scope %q", req.Scope)
	}
	if strings.TrimSpace(req.Action) != "" {
		if _, err := parseAXAction(req.Action); err != nil {
			return common.AXElementSearchQuery{}, err
		}
	}
	return common.AXElementSearchQuery{
		Scope:               scope,
		WindowHandle:        common.WindowHandle(req.WindowHandle),
		Role:                req.Role,
		Subrole:             req.Subrole,
		TitleContains:       req.TitleContains,
		ValueContains:       req.ValueContains,
		DescriptionContains: req.DescriptionContains,
		Action:              req.Action,
		Enabled:             req.Enabled,
		Focused:             req.Focused,
		Limit:               req.Limit,
		MaxDepth:            req.MaxDepth,
	}, nil
}

func parseAXElementRef(req AXElementRefResult) (common.AXElementRef, error) {
	scope, err := parseAXSearchScope(req.Scope)
	if err != nil {
		return common.AXElementRef{}, err
	}
	path := append([]int(nil), req.Path...)
	for _, entry := range path {
		if entry < 0 {
			return common.AXElementRef{}, fmt.Errorf("ref.path entries must be non-negative")
		}
	}
	if scope == common.AXSearchScopeWindowHandle && req.WindowHandle == 0 {
		return common.AXElementRef{}, fmt.Errorf("ref.window_handle is required for scope %q", req.Scope)
	}
	return common.AXElementRef{
		Scope:        scope,
		OwnerPID:     req.OwnerPID,
		WindowHandle: common.WindowHandle(req.WindowHandle),
		Path:         path,
	}, nil
}

func parseAXAction(value string) (common.AXAction, error) {
	action := common.AXAction(strings.TrimSpace(value))
	switch action {
	case common.AXPress, common.AXRaise, common.AXShowMenu, common.AXConfirm, common.AXPick:
		return action, nil
	default:
		return "", fmt.Errorf("unsupported AX action %q", value)
	}
}

func regionFromRect(rect common.Rect) Region {
	return Region{
		Left:   rect.X,
		Top:    rect.Y,
		Width:  rect.Width,
		Height: rect.Height,
	}
}
