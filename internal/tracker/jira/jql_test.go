package jira

import (
	"testing"
)

func TestQuoteJQL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  `""`,
		},
		{
			name:  "plain project key",
			input: "ABC",
			want:  `"ABC"`,
		},
		{
			name:  "string with double quote",
			input: `say "hi"`,
			want:  `"say \"hi\""`,
		},
		{
			name:  "string with backslash",
			input: `a\b`,
			want:  `"a\\b"`,
		},
		{
			name:  "string with both backslash and double quote",
			input: `a\"b`,
			want:  `"a\\\"b"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteJQL(tc.input)
			if got != tc.want {
				t.Errorf("quoteJQL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestValidateProjectKey(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ABC", input: "ABC", wantErr: false},
		{name: "single letter", input: "A", wantErr: false},
		{name: "with underscore and digit", input: "PROJ_1", wantErr: false},
		{name: "letters and digits", input: "AB123", wantErr: false},
		{name: "empty string", input: "", wantErr: true},
		{name: "lowercase", input: "abc", wantErr: true},
		{name: "starts with digit", input: "1AB", wantErr: true},
		{name: "hyphen", input: "AB-CD", wantErr: true},
		{name: "space", input: "AB CD", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateProjectKey(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("validateProjectKey(%q): expected error, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateProjectKey(%q): unexpected error: %v", tc.input, err)
			}
		})
	}
}
