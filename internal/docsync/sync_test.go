package docsync

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestLanguageIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want protocol.LanguageIdentifier
	}{
		{"file.ts", protocol.TypeScriptLanguage},
		{"file.tsx", protocol.TypeScriptReactLanguage},
		{"file.js", protocol.JavaScriptLanguage},
		{"file.jsx", protocol.JavaScriptReactLanguage},
		{"file.d.ts", protocol.TypeScriptLanguage}, // .d.ts extension is .ts
		{"/path/to/deep/file.ts", protocol.TypeScriptLanguage},
		{"/path/to/deep/file.tsx", protocol.TypeScriptReactLanguage},
		{"FILE.TS", protocol.TypeScriptLanguage},   // case insensitive
		{"FILE.TSX", protocol.TypeScriptReactLanguage},
		{"FILE.JS", protocol.JavaScriptLanguage},
		{"FILE.JSX", protocol.JavaScriptReactLanguage},
		{"unknown.go", protocol.TypeScriptLanguage}, // default fallback
		{"noext", protocol.TypeScriptLanguage},      // no extension defaults to TS
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := languageIDFromPath(tt.path)
			if got != tt.want {
				t.Errorf("languageIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
