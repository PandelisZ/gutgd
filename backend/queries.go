package backend

import (
	"fmt"
	"regexp"
	"strings"

	"gut/shared"
)

func buildWindowQuery(req WindowQueryRequest) (shared.WindowQuery, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return shared.WindowQuery{}, fmt.Errorf("window title is required")
	}
	if req.UseRegex {
		pattern, err := regexp.Compile(title)
		if err != nil {
			return shared.WindowQuery{}, err
		}
		return shared.NewWindowQuery(title, shared.MatchPattern(pattern)), nil
	}
	return shared.NewWindowQuery(title, shared.MatchString(title)), nil
}

func parseButton(value string) (shared.Button, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "left":
		return shared.ButtonLeft, nil
	case "middle":
		return shared.ButtonMiddle, nil
	case "right":
		return shared.ButtonRight, nil
	default:
		return 0, fmt.Errorf("unsupported mouse button %q", value)
	}
}

func parseKey(value string) (shared.Key, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return 0, fmt.Errorf("empty key value")
	}
	if len(normalized) == 1 {
		switch normalized[0] {
		case 'a':
			return shared.KeyA, nil
		case 'b':
			return shared.KeyB, nil
		case 'c':
			return shared.KeyC, nil
		case 'd':
			return shared.KeyD, nil
		case 'e':
			return shared.KeyE, nil
		case 'f':
			return shared.KeyF, nil
		case 'g':
			return shared.KeyG, nil
		case 'h':
			return shared.KeyH, nil
		case 'i':
			return shared.KeyI, nil
		case 'j':
			return shared.KeyJ, nil
		case 'k':
			return shared.KeyK, nil
		case 'l':
			return shared.KeyL, nil
		case 'm':
			return shared.KeyM, nil
		case 'n':
			return shared.KeyN, nil
		case 'o':
			return shared.KeyO, nil
		case 'p':
			return shared.KeyP, nil
		case 'q':
			return shared.KeyQ, nil
		case 'r':
			return shared.KeyR, nil
		case 's':
			return shared.KeyS, nil
		case 't':
			return shared.KeyT, nil
		case 'u':
			return shared.KeyU, nil
		case 'v':
			return shared.KeyV, nil
		case 'w':
			return shared.KeyW, nil
		case 'x':
			return shared.KeyX, nil
		case 'y':
			return shared.KeyY, nil
		case 'z':
			return shared.KeyZ, nil
		case '0':
			return shared.KeyNum0, nil
		case '1':
			return shared.KeyNum1, nil
		case '2':
			return shared.KeyNum2, nil
		case '3':
			return shared.KeyNum3, nil
		case '4':
			return shared.KeyNum4, nil
		case '5':
			return shared.KeyNum5, nil
		case '6':
			return shared.KeyNum6, nil
		case '7':
			return shared.KeyNum7, nil
		case '8':
			return shared.KeyNum8, nil
		case '9':
			return shared.KeyNum9, nil
		}
	}

	if key, ok := namedKeys[normalized]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unsupported key %q", value)
}

var namedKeys = map[string]shared.Key{
	"alt":            shared.KeyLeftAlt,
	"backspace":      shared.KeyBackspace,
	"comma":          shared.KeyComma,
	"ctrl":           shared.KeyLeftControl,
	"delete":         shared.KeyDelete,
	"down":           shared.KeyDown,
	"end":            shared.KeyEnd,
	"enter":          shared.KeyEnter,
	"escape":         shared.KeyEscape,
	"esc":            shared.KeyEscape,
	"f1":             shared.KeyF1,
	"f2":             shared.KeyF2,
	"f3":             shared.KeyF3,
	"f4":             shared.KeyF4,
	"f5":             shared.KeyF5,
	"f6":             shared.KeyF6,
	"f7":             shared.KeyF7,
	"f8":             shared.KeyF8,
	"f9":             shared.KeyF9,
	"f10":            shared.KeyF10,
	"f11":            shared.KeyF11,
	"f12":            shared.KeyF12,
	"home":           shared.KeyHome,
	"left":           shared.KeyLeft,
	"leftalt":        shared.KeyLeftAlt,
	"leftctrl":       shared.KeyLeftControl,
	"leftshift":      shared.KeyLeftShift,
	"minus":          shared.KeyMinus,
	"pagedown":       shared.KeyPageDown,
	"pageup":         shared.KeyPageUp,
	"period":         shared.KeyPeriod,
	"quote":          shared.KeyQuote,
	"return":         shared.KeyReturn,
	"right":          shared.KeyRight,
	"rightalt":       shared.KeyRightAlt,
	"rightctrl":      shared.KeyRightControl,
	"rightshift":     shared.KeyRightShift,
	"semicolon":      shared.KeySemicolon,
	"shift":          shared.KeyLeftShift,
	"slash":          shared.KeySlash,
	"space":          shared.KeySpace,
	"tab":            shared.KeyTab,
	"up":             shared.KeyUp,
	"win":            shared.KeyLeftWin,
	"leftwin":        shared.KeyLeftWin,
	"rightwin":       shared.KeyRightWin,
}
