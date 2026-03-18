package utils

import "testing"

func TestValidateDocPath(t *testing.T) {
	cases := []struct {
		path  string
		valid bool
	}{
		{"README.md", true},
		{"subdir/doc.md", true},
		{"../etc/passwd", false},
		{"/absolute/path.md", false},
		{"file.exe", false},
		{"", false},
		{"docs/file.txt", true},
	}
	for _, tc := range cases {
		got := ValidateDocPath(tc.path)
		if got != tc.valid {
			t.Errorf("ValidateDocPath(%q) = %v, want %v", tc.path, got, tc.valid)
		}
	}
}

func TestTruncateText(t *testing.T) {
	s := "hello world"
	if got := TruncateText(s, 100); got != s {
		t.Errorf("expected no truncation")
	}
	got := TruncateText(s, 5)
	if got != "he..." {
		t.Errorf("got %q", got)
	}
}
