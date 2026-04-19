package backend

func agentRawInputDescription(description string) string {
	return description + " When window space targets a background window, this tool first focuses that window because raw mouse and keyboard input cannot safely stay in the background."
}

func (s *Service) runAgentRawInputTool(state *agentCoordinateState, run func() (any, error)) (any, error) {
	if err := s.ensureAgentWindowTargetFocused(state); err != nil {
		return nil, err
	}
	return run()
}
