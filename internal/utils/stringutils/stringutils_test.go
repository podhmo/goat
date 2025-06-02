package stringutils

import "testing"

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"lowercase", "test", "test"},
		{"camelCase", "testString", "test-string"},
		{"PascalCase", "TestString", "test-string"},
		{"withNumber", "testString123", "test-string123"},
		{"numberInMiddle", "test123String", "test123-string"},
		{"allCaps", "TEST", "test"},
		{"mixedCaps", "TestHTTPResponse", "test-http-response"},
		{"singleWordCaps", "URL", "url"},
		{"leadingCaps", "HTTPRequest", "http-request"},
		{"snake_case_input", "test_string", "test_string"}, // No change for snake_case
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToKebabCase(tt.input); got != tt.want {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}