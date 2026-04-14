package mailer

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean ASCII", "Hello, World!", "Hello, World!"},
		{"tab replaced", "col1\tcol2", "col1 col2"},
		{"newline replaced", "line1\nline2", "line1 line2"},
		{"carriage return replaced", "a\rb", "a b"},
		{"null byte replaced", "a\x00b", "a b"},
		{"escape sequence stripped", "\x1b[2J\x1b[HHello", " [2J [HHello"},
		{"BEL stripped", "ding\x07dong", "ding dong"},
		{"DEL stripped", "a\x7Fb", "a b"},
		{"C1 range stripped", "a\u0080\u009Fb", "a  b"},
		{"printable unicode preserved", "日本語 café résumé", "日本語 café résumé"},
		{"mixed controls and text", "\x01\x1b[31mRED\x1b[0m\x07", "  [31mRED [0m "},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.in)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
