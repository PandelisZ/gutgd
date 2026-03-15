package backend

import "testing"

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
