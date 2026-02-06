package docsync

import (
	"runtime"
	"testing"
)

func TestFileToURIAndBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path tests are unix-specific")
	}

	tests := []struct {
		name string
		path string
	}{
		{"simple path", "/home/user/project/file.ts"},
		{"root path", "/file.ts"},
		{"nested path", "/a/b/c/d/e/f.tsx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := FileToURI(tt.path)
			if uri == "" {
				t.Fatal("FileToURI returned empty string")
			}
			got := URIToFile(uri)
			if got != tt.path {
				t.Errorf("round-trip failed: got %q, want %q", got, tt.path)
			}
		})
	}
}

func TestFileToURIScheme(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path tests are unix-specific")
	}

	uri := FileToURI("/tmp/test.ts")
	if len(uri) < 7 || uri[:7] != "file://" {
		t.Errorf("URI should start with file://, got %q", uri)
	}
}

func TestPathWithSpaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path tests are unix-specific")
	}

	path := "/home/user/my project/src/file name.ts"
	uri := FileToURI(path)
	if uri == "" {
		t.Fatal("FileToURI returned empty string for path with spaces")
	}

	got := URIToFile(uri)
	if got != path {
		t.Errorf("round-trip with spaces failed: got %q, want %q", got, path)
	}
}

func TestPathWithSpecialCharacters(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path tests are unix-specific")
	}

	tests := []struct {
		name string
		path string
	}{
		{"parentheses", "/home/user/project (copy)/file.ts"},
		{"hash", "/home/user/project#1/file.ts"},
		{"unicode", "/home/user/\u00e9t\u00e9/file.ts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := FileToURI(tt.path)
			got := URIToFile(uri)
			if got != tt.path {
				t.Errorf("round-trip with special chars failed: got %q, want %q", got, tt.path)
			}
		})
	}
}
