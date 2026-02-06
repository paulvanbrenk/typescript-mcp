package tools

import (
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
)

func TestUTF16ColToByteOffset(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		utf16Col uint32
		want     int
	}{
		// ASCII: each character is 1 byte and 1 UTF-16 unit.
		{name: "ascii col=0", line: "hello", utf16Col: 0, want: 0},
		{name: "ascii col=5", line: "hello", utf16Col: 5, want: 5},

		// 2-byte UTF-8 character (e-acute U+00E9): 1 UTF-16 code unit.
		{name: "2byte after h", line: "h\u00e9llo", utf16Col: 1, want: 1},
		{name: "2byte after e-acute", line: "h\u00e9llo", utf16Col: 2, want: 3},

		// 3-byte UTF-8 character (CJK U+4E2D): 1 UTF-16 code unit.
		{name: "cjk col=1", line: "\u4e2d\u6587", utf16Col: 1, want: 3},

		// 4-byte UTF-8 character (emoji U+1F600): 2 UTF-16 code units (surrogate pair).
		{name: "emoji after a", line: "a\U0001F600b", utf16Col: 1, want: 1},
		{name: "emoji after emoji", line: "a\U0001F600b", utf16Col: 3, want: 5},
		{name: "emoji at b", line: "a\U0001F600b", utf16Col: 4, want: 6},

		// col=0 for any string.
		{name: "col=0 empty", line: "", utf16Col: 0, want: 0},
		{name: "col=0 nonempty", line: "abc", utf16Col: 0, want: 0},

		// col beyond end returns len(line).
		{name: "beyond end ascii", line: "abc", utf16Col: 100, want: 3},
		{name: "beyond end unicode", line: "\u4e2d", utf16Col: 100, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utf16ColToByteOffset(tt.line, tt.utf16Col)
			if got != tt.want {
				t.Errorf("utf16ColToByteOffset(%q, %d) = %d, want %d", tt.line, tt.utf16Col, got, tt.want)
			}
		})
	}
}

func TestApplyFileEdits(t *testing.T) {
	t.Run("single edit replacing greet with sayHello", func(t *testing.T) {
		content := []byte("export function greet(name: string): string {\n  return \"Hello\";\n}\n")
		edits := []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 16},
					End:   protocol.Position{Line: 0, Character: 21},
				},
				NewText: "sayHello",
			},
		}
		got, err := applyFileEdits(content, edits)
		if err != nil {
			t.Fatalf("applyFileEdits: %v", err)
		}
		want := "export function sayHello(name: string): string {\n  return \"Hello\";\n}\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", string(got), want)
		}
	})

	t.Run("multiple edits on different lines", func(t *testing.T) {
		content := []byte("const a = greet;\nconst b = greet;\n")
		edits := []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 10},
					End:   protocol.Position{Line: 0, Character: 15},
				},
				NewText: "sayHello",
			},
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 1, Character: 10},
					End:   protocol.Position{Line: 1, Character: 15},
				},
				NewText: "sayHello",
			},
		}
		got, err := applyFileEdits(content, edits)
		if err != nil {
			t.Fatalf("applyFileEdits: %v", err)
		}
		want := "const a = sayHello;\nconst b = sayHello;\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", string(got), want)
		}
	})

	t.Run("multiple edits on the same line", func(t *testing.T) {
		// "import { greet, greet2 } from './index';\n"
		content := []byte("import { greet, greet2 } from './index';\n")
		edits := []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 9},
					End:   protocol.Position{Line: 0, Character: 14},
				},
				NewText: "sayHello",
			},
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 16},
					End:   protocol.Position{Line: 0, Character: 22},
				},
				NewText: "sayHello2",
			},
		}
		got, err := applyFileEdits(content, edits)
		if err != nil {
			t.Fatalf("applyFileEdits: %v", err)
		}
		want := "import { sayHello, sayHello2 } from './index';\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", string(got), want)
		}
	})

	t.Run("edit replacing multiple characters on same line", func(t *testing.T) {
		content := []byte("function longFunctionName() {}\n")
		edits := []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 9},
					End:   protocol.Position{Line: 0, Character: 25},
				},
				NewText: "fn",
			},
		}
		got, err := applyFileEdits(content, edits)
		if err != nil {
			t.Fatalf("applyFileEdits: %v", err)
		}
		want := "function fn() {}\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", string(got), want)
		}
	})
}

func TestApplyFileEditsReverseOrder(t *testing.T) {
	// Create edits in FORWARD order (line 0 before line 2).
	// The function must internally sort to reverse order.
	content := []byte("const a = greet;\nconst b = other;\nconst c = greet;\n")
	edits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 10},
				End:   protocol.Position{Line: 0, Character: 15},
			},
			NewText: "sayHello",
		},
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 10},
				End:   protocol.Position{Line: 2, Character: 15},
			},
			NewText: "sayHello",
		},
	}

	got, err := applyFileEdits(content, edits)
	if err != nil {
		t.Fatalf("applyFileEdits: %v", err)
	}
	want := "const a = sayHello;\nconst b = other;\nconst c = sayHello;\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", string(got), want)
	}
}

func TestApplyWorkspaceEdit(t *testing.T) {
	t.Run("multi-file edit", func(t *testing.T) {
		tmpDir := t.TempDir()

		file1 := filepath.Join(tmpDir, "index.ts")
		file2 := filepath.Join(tmpDir, "consumer.ts")

		content1 := "export function greet(name: string): string {\n  return `Hello, ${name}!`;\n}\n"
		content2 := "import { greet } from './index';\nconst result = greet('world');\n"

		if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		uri1 := protocol.DocumentURI("file://" + file1)
		uri2 := protocol.DocumentURI("file://" + file2)

		edit := &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				uri1: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 16},
							End:   protocol.Position{Line: 0, Character: 21},
						},
						NewText: "sayHello",
					},
				},
				uri2: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 9},
							End:   protocol.Position{Line: 0, Character: 14},
						},
						NewText: "sayHello",
					},
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 1, Character: 15},
							End:   protocol.Position{Line: 1, Character: 20},
						},
						NewText: "sayHello",
					},
				},
			},
		}

		result, err := ApplyWorkspaceEdit(edit)
		if err != nil {
			t.Fatalf("ApplyWorkspaceEdit: %v", err)
		}

		if len(result) != 2 {
			t.Fatalf("expected 2 files in result, got %d", len(result))
		}

		// Verify file1 contents.
		got1, err := os.ReadFile(file1)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		want1 := "export function sayHello(name: string): string {\n  return `Hello, ${name}!`;\n}\n"
		if string(got1) != want1 {
			t.Errorf("file1:\ngot:  %s\nwant: %s", string(got1), want1)
		}

		// Verify file2 contents.
		got2, err := os.ReadFile(file2)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		want2 := "import { sayHello } from './index';\nconst result = sayHello('world');\n"
		if string(got2) != want2 {
			t.Errorf("file2:\ngot:  %s\nwant: %s", string(got2), want2)
		}
	})

	t.Run("rollback on write failure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Names chosen so "aaa_writable.ts" sorts before "zzz_readonly.ts",
		// guaranteeing the writable file is written first and must be rolled back.
		writableFile := filepath.Join(tmpDir, "aaa_writable.ts")
		readonlyFile := filepath.Join(tmpDir, "zzz_readonly.ts")

		writableContent := "const a = greet;\n"
		readonlyContent := "const b = greet;\n"

		if err := os.WriteFile(writableFile, []byte(writableContent), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(readonlyFile, []byte(readonlyContent), 0444); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		// Ensure cleanup can remove the read-only file.
		t.Cleanup(func() { _ = os.Chmod(readonlyFile, 0644) })

		writableURI := protocol.DocumentURI("file://" + writableFile)
		readonlyURI := protocol.DocumentURI("file://" + readonlyFile)

		edit := &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				writableURI: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 10},
							End:   protocol.Position{Line: 0, Character: 15},
						},
						NewText: "sayHello",
					},
				},
				readonlyURI: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 10},
							End:   protocol.Position{Line: 0, Character: 15},
						},
						NewText: "sayHello",
					},
				},
			},
		}

		_, err := ApplyWorkspaceEdit(edit)
		if err == nil {
			t.Fatal("expected error due to read-only file, got nil")
		}

		// Verify the writable file was rolled back to original.
		got, err := os.ReadFile(writableFile)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != writableContent {
			t.Errorf("writable file not rolled back:\ngot:  %s\nwant: %s", string(got), writableContent)
		}
	})
}
