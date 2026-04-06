package jira

import "testing"

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
