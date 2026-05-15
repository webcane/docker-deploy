package filetransfer

import "testing"

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "''",
		},
		{
			name:  "normal path",
			input: "/opt/normal",
			want:  "'/opt/normal'",
		},
		{
			name:  "path with embedded single quote",
			input: "/opt/foo'bar",
			want:  "'/opt/foo'\\''bar'",
		},
		{
			name:  "string with apostrophe",
			input: "it's a test",
			want:  "'it'\\''s a test'",
		},
		{
			name:  "multiple single quotes",
			input: "a'b'c",
			want:  "'a'\\''b'\\''c'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShellQuote(tt.input)
			if got != tt.want {
				t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
