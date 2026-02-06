package lsp

import (
	"encoding/json"
	"testing"

	"go.lsp.dev/protocol"
)

func TestParseDocumentSymbolItem_SymbolInformation(t *testing.T) {
	// SymbolInformation format: flat with "location" containing the range.
	siJSON := `{
		"name": "greet",
		"kind": 12,
		"location": {
			"uri": "file:///test/index.ts",
			"range": {
				"start": {"line": 4, "character": 0},
				"end": {"line": 6, "character": 1}
			}
		}
	}`

	var raw interface{}
	if err := json.Unmarshal([]byte(siJSON), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	sym, ok := parseDocumentSymbolItem(raw)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if sym.Name != "greet" {
		t.Errorf("Name = %q, want %q", sym.Name, "greet")
	}
	if sym.Kind != protocol.SymbolKindFunction {
		t.Errorf("Kind = %v, want Function (%v)", sym.Kind, protocol.SymbolKindFunction)
	}
	if sym.Range.Start.Line != 4 {
		t.Errorf("Range.Start.Line = %d, want 4", sym.Range.Start.Line)
	}
	if sym.Range.End.Line != 6 {
		t.Errorf("Range.End.Line = %d, want 6", sym.Range.End.Line)
	}
	if sym.SelectionRange.Start.Line != 4 {
		t.Errorf("SelectionRange.Start.Line = %d, want 4", sym.SelectionRange.Start.Line)
	}
}

func TestParseDocumentSymbolItem_DocumentSymbol(t *testing.T) {
	// DocumentSymbol format: hierarchical with "range" and "selectionRange".
	dsJSON := `{
		"name": "MyClass",
		"kind": 5,
		"range": {
			"start": {"line": 10, "character": 0},
			"end": {"line": 20, "character": 1}
		},
		"selectionRange": {
			"start": {"line": 10, "character": 6},
			"end": {"line": 10, "character": 13}
		},
		"children": [
			{
				"name": "constructor",
				"kind": 9,
				"range": {
					"start": {"line": 11, "character": 2},
					"end": {"line": 13, "character": 3}
				},
				"selectionRange": {
					"start": {"line": 11, "character": 2},
					"end": {"line": 11, "character": 13}
				}
			}
		]
	}`

	var raw interface{}
	if err := json.Unmarshal([]byte(dsJSON), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	sym, ok := parseDocumentSymbolItem(raw)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if sym.Name != "MyClass" {
		t.Errorf("Name = %q, want %q", sym.Name, "MyClass")
	}
	if sym.Kind != protocol.SymbolKindClass {
		t.Errorf("Kind = %v, want Class (%v)", sym.Kind, protocol.SymbolKindClass)
	}
	if sym.Range.Start.Line != 10 {
		t.Errorf("Range.Start.Line = %d, want 10", sym.Range.Start.Line)
	}
	if sym.SelectionRange.Start.Line != 10 {
		t.Errorf("SelectionRange.Start.Line = %d, want 10", sym.SelectionRange.Start.Line)
	}
	if sym.SelectionRange.Start.Character != 6 {
		t.Errorf("SelectionRange.Start.Character = %d, want 6", sym.SelectionRange.Start.Character)
	}
	if len(sym.Children) != 1 {
		t.Fatalf("Children count = %d, want 1", len(sym.Children))
	}
	if sym.Children[0].Name != "constructor" {
		t.Errorf("Children[0].Name = %q, want %q", sym.Children[0].Name, "constructor")
	}
	if sym.Children[0].Range.Start.Line != 11 {
		t.Errorf("Children[0].Range.Start.Line = %d, want 11", sym.Children[0].Range.Start.Line)
	}
}
