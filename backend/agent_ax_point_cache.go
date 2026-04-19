package backend

import (
	"strconv"
	"strings"

	"github.com/PandelisZ/gut/native/common"
)

type agentPointerState struct {
	LastScreenPoint *Point
	Pressed         bool
	AXActionPoints  map[string]Point
}

func newAgentPointerState() *agentPointerState {
	return &agentPointerState{
		AXActionPoints: make(map[string]Point),
	}
}

func cloneAgentPointerState(state agentPointerState) agentPointerState {
	copy := agentPointerState{
		Pressed:        state.Pressed,
		AXActionPoints: make(map[string]Point, len(state.AXActionPoints)),
	}
	if state.LastScreenPoint != nil {
		point := *state.LastScreenPoint
		copy.LastScreenPoint = &point
	}
	for key, point := range state.AXActionPoints {
		copy.AXActionPoints[key] = point
	}
	return copy
}

func (s *Service) loadAgentPointerState(previousResponseID string) *agentPointerState {
	state := newAgentPointerState()
	key := strings.TrimSpace(previousResponseID)
	if key == "" {
		return state
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.agentPointerStates[key]
	if !ok {
		return state
	}
	cloned := cloneAgentPointerState(existing)
	return &cloned
}

func (s *Service) saveAgentPointerState(responseID string, state *agentPointerState) {
	key := strings.TrimSpace(responseID)
	if key == "" || state == nil {
		return
	}

	next := cloneAgentPointerState(*state)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentPointerStates == nil {
		s.agentPointerStates = make(map[string]agentPointerState)
	}
	s.agentPointerStates[key] = next
}

func (state *agentPointerState) rememberLast(point Point) {
	if state == nil {
		return
	}
	copy := point
	state.LastScreenPoint = &copy
}

func (state *agentPointerState) rememberAXPointForResult(ref AXElementRefResult, point Point) {
	if state == nil {
		return
	}
	if state.AXActionPoints == nil {
		state.AXActionPoints = make(map[string]Point)
	}
	state.AXActionPoints[agentAXResultRefKey(ref)] = point
}

func (state *agentPointerState) rememberAXPoint(ref common.AXElementRef, point Point) {
	if state == nil {
		return
	}
	if state.AXActionPoints == nil {
		state.AXActionPoints = make(map[string]Point)
	}
	state.AXActionPoints[agentAXRefKey(ref)] = point
}

func (state *agentPointerState) lookupAXPointForResult(ref AXElementRefResult) (Point, bool) {
	if state == nil || state.AXActionPoints == nil {
		return Point{}, false
	}
	point, ok := state.AXActionPoints[agentAXResultRefKey(ref)]
	return point, ok
}

func (state *agentPointerState) lookupAXPoint(ref common.AXElementRef) (Point, bool) {
	if state == nil || state.AXActionPoints == nil {
		return Point{}, false
	}
	point, ok := state.AXActionPoints[agentAXRefKey(ref)]
	return point, ok
}

func agentAXResultRefKey(ref AXElementRefResult) string {
	return agentAXKey(ref.Scope, ref.OwnerPID, ref.WindowHandle, ref.Path)
}

func agentAXRefKey(ref common.AXElementRef) string {
	return agentAXKey(string(ref.Scope), ref.OwnerPID, uint64(ref.WindowHandle), ref.Path)
}

func agentAXKey(scope string, ownerPID int, windowHandle uint64, path []int) string {
	var builder strings.Builder
	builder.Grow(48 + len(path)*4)
	builder.WriteString(strings.TrimSpace(scope))
	builder.WriteByte(':')
	builder.WriteString(strconv.Itoa(ownerPID))
	builder.WriteByte(':')
	builder.WriteString(strconv.FormatUint(windowHandle, 10))
	builder.WriteByte(':')
	for index, part := range path {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.Itoa(part))
	}
	return builder.String()
}
