package scraper

import (
	"testing"
)

// TestEscapeXPathText tests the XPath escaping function for security
func TestEscapeXPathText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no quotes",
			input:    "simple text",
			expected: "'simple text'",
		},
		{
			name:     "single quotes only",
			input:    "text with 'quotes'",
			expected: "\"text with 'quotes'\"",
		},
		{
			name:     "double quotes only",
			input:    "text with \"quotes\"",
			expected: "'text with \"quotes\"'",
		},
		{
			name:     "both single and double quotes",
			input:    "text with 'single' and \"double\" quotes",
			expected: "concat('text with ', \"'\", 'single', \"'\", ' and \"double\" quotes')",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "only single quote",
			input:    "'",
			expected: "\"'\"",
		},
		{
			name:     "only double quote",
			input:    "\"",
			expected: "'\"'",
		},
		{
			name:     "multiple single quotes",
			input:    "a'b'c",
			expected: "\"a'b'c\"",
		},
		{
			name:     "XPath injection attempt",
			input:    "'] | //script[@src",
			expected: "\"'] | //script[@src\"",
		},
		{
			name:     "complex injection attempt",
			input:    "test'] and @onclick='alert(\"XSS\")'",
			expected: "concat('test', \"'\", '] and @onclick=', \"'\", 'alert(\"XSS\")', \"'\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeXPathText(tt.input)
			if result != tt.expected {
				t.Errorf("escapeXPathText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestXPathInjectionPrevention tests that the escaping prevents actual injection
func TestXPathInjectionPrevention(t *testing.T) {
	maliciousInputs := []string{
		"'] | //script[@src='evil.js",
		"test' or '1'='1",
		"'] and contains(@onclick, 'alert(",
		"']/following::*[@id='admin",
		"'] union select * from users where '1'='1",
	}

	for _, input := range maliciousInputs {
		t.Run("injection_attempt_"+input[:min(10, len(input))], func(t *testing.T) {
			escaped := escapeXPathText(input)

			// Verify that the escaped text doesn't contain unescaped quotes that could break out of the XPath
			// The escaped result should either be fully quoted or use concat()
			if !((escaped[0] == '\'' && escaped[len(escaped)-1] == '\'') ||
				(escaped[0] == '"' && escaped[len(escaped)-1] == '"') ||
				(len(escaped) >= 7 && escaped[:7] == "concat(")) {
				t.Errorf("escapeXPathText(%q) = %q, doesn't appear to be properly escaped", input, escaped)
			}
		})
	}
}

// Helper function for min (Go doesn't have built-in min for int in older versions)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
