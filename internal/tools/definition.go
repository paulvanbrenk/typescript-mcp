package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/paulvanbrenk/typescript-mcp/internal/docsync"
	"github.com/paulvanbrenk/typescript-mcp/internal/lsp"
)

type definitionEntry struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview,omitempty"`
}

func makeDefinitionHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		file, err := request.RequireString("file")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		line, err := request.RequireInt("line")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		col, err := request.RequireInt("column")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := docs.SyncFile(ctx, client.Conn(), file); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sync error: %v", err)), nil
		}

		locs, err := client.Definition(ctx, file, line, col)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("definition error: %v", err)), nil
		}

		if len(locs) == 0 {
			return mcp.NewToolResultText("No definition found"), nil
		}

		entries := make([]definitionEntry, len(locs))
		for i, loc := range locs {
			defFile := docsync.URIToFile(string(loc.URI))
			defLine := int(loc.Range.Start.Line) + 1
			defCol := int(loc.Range.Start.Character) + 1

			entry := definitionEntry{
				File:   defFile,
				Line:   defLine,
				Column: defCol,
			}

			// Read the preview line from the target file
			if preview, err := readLine(defFile, defLine); err == nil {
				entry.Preview = strings.TrimSpace(preview)
			}

			entries[i] = entry
		}

		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
