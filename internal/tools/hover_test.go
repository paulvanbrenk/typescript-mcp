package tools

import "testing"

func TestExtractConciseHover(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{
			name: "code block",
			md:   "```typescript\nfunction greet(name: string): string\n```\nSome docs",
			want: "function greet(name: string): string",
		},
		{
			name: "no code block",
			md:   "Just plain text hover",
			want: "Just plain text hover",
		},
		{
			name: "multiline code block",
			md:   "```ts\ninterface Foo {\n  bar: string;\n}\n```",
			want: "interface Foo {\n  bar: string;\n}",
		},
		{
			name: "empty code block",
			md:   "```\n```\nFallback text",
			want: "```\n```\nFallback text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractConciseHover(tt.md)
			if got != tt.want {
				t.Errorf("extractConciseHover() = %q, want %q", got, tt.want)
			}
		})
	}
}
