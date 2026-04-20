package backend

import (
	"context"
	"time"

	"github.com/PandelisZ/gut"
	"github.com/PandelisZ/gut/native/common"
	"github.com/PandelisZ/gut/provider"
	"github.com/PandelisZ/gut/shared"
)

type fakeBackendWindowProvider struct {
	windows    []shared.WindowHandle
	active     shared.WindowHandle
	titles     map[shared.WindowHandle]string
	regions    map[shared.WindowHandle]shared.Region
	focusCalls []shared.WindowHandle
}

func (f *fakeBackendWindowProvider) GetWindows(context.Context) ([]shared.WindowHandle, error) {
	return append([]shared.WindowHandle(nil), f.windows...), nil
}

func (f *fakeBackendWindowProvider) GetActiveWindow(context.Context) (shared.WindowHandle, error) {
	return f.active, nil
}

func (f *fakeBackendWindowProvider) GetWindowTitle(_ context.Context, handle shared.WindowHandle) (string, error) {
	return f.titles[handle], nil
}

func (f *fakeBackendWindowProvider) GetWindowRegion(_ context.Context, handle shared.WindowHandle) (shared.Region, error) {
	return f.regions[handle], nil
}

func (f *fakeBackendWindowProvider) FocusWindow(_ context.Context, handle shared.WindowHandle) (bool, error) {
	f.focusCalls = append(f.focusCalls, handle)
	f.active = handle
	return true, nil
}

func (f *fakeBackendWindowProvider) MoveWindow(context.Context, shared.WindowHandle, shared.Point) (bool, error) {
	return true, nil
}

func (f *fakeBackendWindowProvider) ResizeWindow(context.Context, shared.WindowHandle, shared.Size) (bool, error) {
	return true, nil
}

func (f *fakeBackendWindowProvider) MinimizeWindow(context.Context, shared.WindowHandle) (bool, error) {
	return true, nil
}

func (f *fakeBackendWindowProvider) RestoreWindow(context.Context, shared.WindowHandle) (bool, error) {
	return true, nil
}

type fakeBackendScreenProvider struct {
	size shared.Region
}

type fakeBackendMouseProvider struct {
	position        shared.Point
	positionErr     error
	setPositions    []shared.Point
	clicks          []shared.Button
	doubleClicks    []shared.Button
	scrollUp        []int
	scrollDown      []int
	scrollLeft      []int
	scrollRight     []int
	pressedButtons  []shared.Button
	releasedButtons []shared.Button
}

type fakeBackendKeyboardProvider struct {
	typedText    []string
	tappedKeys   [][]shared.Key
	pressedKeys  [][]shared.Key
	releasedKeys [][]shared.Key
}

type fakeBackendAccessibilityProvider struct {
	permissionSnapshot             common.PermissionSnapshot
	focusedWindow                  common.FocusedWindowMetadata
	focusedElement                 common.UIElementMetadata
	elementAtPoint                 common.UIElementMetadata
	searchAXElementsMatches        []common.AXElementMatch
	capabilities                   common.CapabilitySet
	lastPoint                      shared.Point
	lastFocusedElementAction       common.AXAction
	lastElementActionAtPoint       shared.Point
	lastElementActionAtPointAction common.AXAction
	lastFocusElementAtPoint        shared.Point
	lastSearchAXQuery              common.AXElementSearchQuery
	lastFocusAXRef                 common.AXElementRef
	lastPerformAXRef               common.AXElementRef
	lastPerformAXAction            common.AXAction
	performAXElementActionHook     func(common.AXElementRef, common.AXAction)
	raiseFocusedWindowCalls        int
	permissionErr                  error
	focusedWindowErr               error
	focusedElementErr              error
	elementAtPointErr              error
	searchAXElementsErr            error
	raiseFocusedWindowErr          error
	performFocusedElementActionErr error
	performElementActionAtPointErr error
	focusElementAtPointErr         error
	focusAXElementErr              error
	performAXElementActionErr      error
}

type fakeBackendElementInspectionProvider struct {
	root           shared.WindowElement
	lastHandle     shared.WindowHandle
	lastMax        int
	getElementsErr error
}

func (f *fakeBackendScreenProvider) GrabScreen(context.Context) (shared.Image, error) {
	return shared.Image{}, nil
}

func (f *fakeBackendScreenProvider) GrabScreenRegion(context.Context, shared.Region) (shared.Image, error) {
	return shared.Image{}, nil
}

func (f *fakeBackendScreenProvider) HighlightScreenRegion(context.Context, shared.Region, time.Duration, float64) error {
	return nil
}

func (f *fakeBackendScreenProvider) ScreenWidth(context.Context) (int, error) {
	return f.size.Width, nil
}

func (f *fakeBackendScreenProvider) ScreenHeight(context.Context) (int, error) {
	return f.size.Height, nil
}

func (f *fakeBackendScreenProvider) ScreenSize(context.Context) (shared.Region, error) {
	return f.size, nil
}

func (f *fakeBackendMouseProvider) SetMouseDelay(time.Duration) {}

func (f *fakeBackendMouseProvider) SetMousePosition(_ context.Context, point shared.Point) error {
	f.position = point
	f.setPositions = append(f.setPositions, point)
	return nil
}

func (f *fakeBackendMouseProvider) CurrentMousePosition(context.Context) (shared.Point, error) {
	if f.positionErr != nil {
		return shared.Point{}, f.positionErr
	}
	return f.position, nil
}

func (f *fakeBackendMouseProvider) Click(_ context.Context, button shared.Button) error {
	f.clicks = append(f.clicks, button)
	return nil
}

func (f *fakeBackendMouseProvider) DoubleClick(_ context.Context, button shared.Button) error {
	f.doubleClicks = append(f.doubleClicks, button)
	return nil
}

func (f *fakeBackendMouseProvider) ScrollUp(_ context.Context, amount int) error {
	f.scrollUp = append(f.scrollUp, amount)
	return nil
}

func (f *fakeBackendMouseProvider) ScrollDown(_ context.Context, amount int) error {
	f.scrollDown = append(f.scrollDown, amount)
	return nil
}

func (f *fakeBackendMouseProvider) ScrollLeft(_ context.Context, amount int) error {
	f.scrollLeft = append(f.scrollLeft, amount)
	return nil
}

func (f *fakeBackendMouseProvider) ScrollRight(_ context.Context, amount int) error {
	f.scrollRight = append(f.scrollRight, amount)
	return nil
}

func (f *fakeBackendMouseProvider) PressButton(_ context.Context, button shared.Button) error {
	f.pressedButtons = append(f.pressedButtons, button)
	return nil
}

func (f *fakeBackendMouseProvider) ReleaseButton(_ context.Context, button shared.Button) error {
	f.releasedButtons = append(f.releasedButtons, button)
	return nil
}

func (f *fakeBackendKeyboardProvider) SetKeyboardDelay(time.Duration) {}

func (f *fakeBackendKeyboardProvider) Type(_ context.Context, input string) error {
	f.typedText = append(f.typedText, input)
	return nil
}

func (f *fakeBackendKeyboardProvider) Click(_ context.Context, keys ...shared.Key) error {
	f.tappedKeys = append(f.tappedKeys, append([]shared.Key(nil), keys...))
	return nil
}

func (f *fakeBackendKeyboardProvider) PressKey(_ context.Context, keys ...shared.Key) error {
	f.pressedKeys = append(f.pressedKeys, append([]shared.Key(nil), keys...))
	return nil
}

func (f *fakeBackendKeyboardProvider) ReleaseKey(_ context.Context, keys ...shared.Key) error {
	f.releasedKeys = append(f.releasedKeys, append([]shared.Key(nil), keys...))
	return nil
}

func (f *fakeBackendAccessibilityProvider) GetPermissionSnapshot(context.Context) (common.PermissionSnapshot, error) {
	if f.permissionErr != nil {
		return common.PermissionSnapshot{}, f.permissionErr
	}
	return f.permissionSnapshot, nil
}

func (f *fakeBackendAccessibilityProvider) GetFocusedWindow(context.Context) (common.FocusedWindowMetadata, error) {
	if f.focusedWindowErr != nil {
		return common.FocusedWindowMetadata{}, f.focusedWindowErr
	}
	return f.focusedWindow, nil
}

func (f *fakeBackendAccessibilityProvider) GetFocusedElement(context.Context) (common.UIElementMetadata, error) {
	if f.focusedElementErr != nil {
		return common.UIElementMetadata{}, f.focusedElementErr
	}
	return f.focusedElement, nil
}

func (f *fakeBackendAccessibilityProvider) GetElementAtPoint(_ context.Context, point shared.Point) (common.UIElementMetadata, error) {
	f.lastPoint = point
	if f.elementAtPointErr != nil {
		return common.UIElementMetadata{}, f.elementAtPointErr
	}
	return f.elementAtPoint, nil
}

func (f *fakeBackendAccessibilityProvider) RaiseFocusedWindow(context.Context) error {
	f.raiseFocusedWindowCalls++
	return f.raiseFocusedWindowErr
}

func (f *fakeBackendAccessibilityProvider) PerformFocusedElementAction(_ context.Context, action common.AXAction) error {
	f.lastFocusedElementAction = action
	return f.performFocusedElementActionErr
}

func (f *fakeBackendAccessibilityProvider) PerformElementActionAtPoint(_ context.Context, point shared.Point, action common.AXAction) error {
	f.lastElementActionAtPoint = point
	f.lastElementActionAtPointAction = action
	return f.performElementActionAtPointErr
}

func (f *fakeBackendAccessibilityProvider) FocusElementAtPoint(_ context.Context, point shared.Point) error {
	f.lastFocusElementAtPoint = point
	return f.focusElementAtPointErr
}

func (f *fakeBackendAccessibilityProvider) SearchAXElements(_ context.Context, query common.AXElementSearchQuery) ([]common.AXElementMatch, error) {
	f.lastSearchAXQuery = query
	if f.searchAXElementsErr != nil {
		return nil, f.searchAXElementsErr
	}
	return append([]common.AXElementMatch(nil), f.searchAXElementsMatches...), nil
}

func (f *fakeBackendAccessibilityProvider) FocusAXElement(_ context.Context, ref common.AXElementRef) error {
	f.lastFocusAXRef = ref
	return f.focusAXElementErr
}

func (f *fakeBackendAccessibilityProvider) PerformAXElementAction(_ context.Context, ref common.AXElementRef, action common.AXAction) error {
	f.lastPerformAXRef = ref
	f.lastPerformAXAction = action
	if f.performAXElementActionHook != nil {
		f.performAXElementActionHook(ref, action)
	}
	return f.performAXElementActionErr
}

func (f *fakeBackendAccessibilityProvider) Capabilities() common.CapabilitySet {
	if f.capabilities == nil {
		return common.NewCapabilitySet()
	}
	return f.capabilities
}

func (f *fakeBackendElementInspectionProvider) GetElements(_ context.Context, windowHandle shared.WindowHandle, maxElements int) (shared.WindowElement, error) {
	f.lastHandle = windowHandle
	f.lastMax = maxElements
	if f.getElementsErr != nil {
		return shared.WindowElement{}, f.getElementsErr
	}
	return f.root, nil
}

func (f *fakeBackendElementInspectionProvider) FindElement(context.Context, shared.WindowHandle, shared.WindowElementDescription) (shared.WindowElement, error) {
	return shared.WindowElement{}, nil
}

func (f *fakeBackendElementInspectionProvider) FindElements(context.Context, shared.WindowHandle, shared.WindowElementDescription) ([]shared.WindowElement, error) {
	return nil, nil
}

func newTestServiceWithWindows(summaries ...WindowSummary) *Service {
	return newTestServiceWithWindowsAndAccessibility(nil, summaries...)
}

func newTestServiceWithWindowsAndAccessibility(accessibility provider.AccessibilityProvider, summaries ...WindowSummary) *Service {
	return newTestServiceWithWindowsAccessibilityAndElements(accessibility, nil, summaries...)
}

func newTestServiceWithWindowsAccessibilityAndElements(accessibility provider.AccessibilityProvider, inspection provider.ElementInspectionProvider, summaries ...WindowSummary) *Service {
	registry := provider.NewRegistry()
	windowProvider := &fakeBackendWindowProvider{
		titles:  make(map[shared.WindowHandle]string, len(summaries)),
		regions: make(map[shared.WindowHandle]shared.Region, len(summaries)),
	}
	for idx, summary := range summaries {
		handle := shared.WindowHandle(summary.Handle)
		windowProvider.windows = append(windowProvider.windows, handle)
		if idx == 0 {
			windowProvider.active = handle
		}
		windowProvider.titles[handle] = summary.Title
		windowProvider.regions[handle] = shared.Region{
			Left:   summary.Region.Left,
			Top:    summary.Region.Top,
			Width:  summary.Region.Width,
			Height: summary.Region.Height,
		}
	}
	registry.RegisterWindow(windowProvider)
	registry.RegisterScreen(&fakeBackendScreenProvider{size: shared.Region{Left: 0, Top: 0, Width: 2560, Height: 1440}})
	registry.RegisterMouse(&fakeBackendMouseProvider{})
	registry.RegisterKeyboard(&fakeBackendKeyboardProvider{})
	if accessibility != nil {
		registry.RegisterAccessibility(accessibility)
	}
	if inspection != nil {
		registry.RegisterElementInspection(inspection)
	}

	return &Service{
		nut:                    gut.New(registry),
		artifactDir:            ".",
		agentCoordinateStates:  make(map[string]agentCoordinateState),
		agentComputerStates:    make(map[string]agentComputerState),
		agentPointerStates:     make(map[string]agentPointerState),
		accessibilitySnapshots: make(map[string]windowAccessibilitySnapshotCache),
		agentLuaSessions:       make(map[string]*agentLuaSession),
		agentTranscriptStates:  make(map[string][]AgentTranscriptItem),
	}
}
