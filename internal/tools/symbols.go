package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.lsp.dev/protocol"

	"github.com/pvanbrenk/typescript-mcp/internal/docsync"
	"github.com/pvanbrenk/typescript-mcp/internal/lsp"
)

type symbolEntry struct {
	Name     string        `json:"name"`
	Kind     string        `json:"kind"`
	Line     int           `json:"line"`
	Detail   string        `json:"detail,omitempty"`
	Children []symbolEntry `json:"children,omitempty"`
}

func makeDocumentSymbolsHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		file, err := request.RequireString("file")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := docs.SyncFile(ctx, client.Conn(), file); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sync error: %v", err)), nil
		}

		symbols, err := client.DocumentSymbol(ctx, file)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("document symbols error: %v", err)), nil
		}

		if len(symbols) == 0 {
			return mcp.NewToolResultText("No symbols found"), nil
		}

		entries := convertSymbols(symbols)

		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func convertSymbols(symbols []protocol.DocumentSymbol) []symbolEntry {
	entries := make([]symbolEntry, len(symbols))
	for i, sym := range symbols {
		entry := symbolEntry{
			Name:   sym.Name,
			Kind:   symbolKindName(sym.Kind),
			Line:   int(sym.Range.Start.Line) + 1,
			Detail: sym.Detail,
		}
		if len(sym.Children) > 0 {
			entry.Children = convertSymbols(sym.Children)
		}
		entries[i] = entry
	}
	return entries
}

func symbolKindName(k protocol.SymbolKind) string {
	switch k {
	case protocol.SymbolKindFile:
		return "file"
	case protocol.SymbolKindModule:
		return "module"
	case protocol.SymbolKindNamespace:
		return "namespace"
	case protocol.SymbolKindPackage:
		return "package"
	case protocol.SymbolKindClass:
		return "class"
	case protocol.SymbolKindMethod:
		return "method"
	case protocol.SymbolKindProperty:
		return "property"
	case protocol.SymbolKindField:
		return "field"
	case protocol.SymbolKindConstructor:
		return "constructor"
	case protocol.SymbolKindEnum:
		return "enum"
	case protocol.SymbolKindInterface:
		return "interface"
	case protocol.SymbolKindFunction:
		return "function"
	case protocol.SymbolKindVariable:
		return "variable"
	case protocol.SymbolKindConstant:
		return "constant"
	case protocol.SymbolKindString:
		return "string"
	case protocol.SymbolKindNumber:
		return "number"
	case protocol.SymbolKindBoolean:
		return "boolean"
	case protocol.SymbolKindArray:
		return "array"
	case protocol.SymbolKindObject:
		return "object"
	case protocol.SymbolKindKey:
		return "key"
	case protocol.SymbolKindEnumMember:
		return "enum_member"
	case protocol.SymbolKindStruct:
		return "struct"
	case protocol.SymbolKindEvent:
		return "event"
	case protocol.SymbolKindOperator:
		return "operator"
	case protocol.SymbolKindTypeParameter:
		return "type_parameter"
	default:
		return fmt.Sprintf("kind(%d)", int(k))
	}
}
