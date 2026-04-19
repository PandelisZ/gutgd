package backend

func (s *Service) ensureWindowFocusedForRawInput(handle uint64) (string, error) {
	if handle == 0 {
		return "foreground_existing", nil
	}

	active, err := s.GetActiveWindow()
	if err == nil && active.Handle == handle {
		return "foreground_existing", nil
	}

	if _, err := s.FocusWindow(WindowHandleRequest{Handle: handle}); err != nil {
		return "", err
	}
	return "foreground_fallback", nil
}

func (s *Service) runWithFocusedWindowForRawInput(handle uint64, action func() (ActionResult, error)) (ActionResult, string, error) {
	mode, err := s.ensureWindowFocusedForRawInput(handle)
	if err != nil {
		return ActionResult{}, "", err
	}

	result, err := action()
	if err != nil {
		return ActionResult{}, "", err
	}
	return result, mode, nil
}

func (s *Service) ensureAgentWindowTargetFocused(state *agentCoordinateState) error {
	if state == nil || state.Mode != "window" || state.Window == nil {
		return nil
	}
	_, err := s.ensureWindowFocusedForRawInput(state.Window.Handle)
	return err
}
