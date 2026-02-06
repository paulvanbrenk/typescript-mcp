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

type referenceEntry struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview,omitempty"`
}

type referencesResult struct {
	References []referenceEntry `json:"references"`
	TotalCount int              `json:"totalCount"`
	Truncated  bool             `json:"truncated"`
}

func makeReferencesHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
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
		maxResults := request.GetInt("maxResults", 50)

		if err := docs.SyncFile(ctx, client.Conn(), file); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sync error: %v", err)), nil
		}

		locs, err := client.References(ctx, file, line, col)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("references error: %v", err)), nil
		}

		totalCount := len(locs)
		truncated := totalCount > maxResults
		if truncated {
			locs = locs[:maxResults]
		}

		entries := make([]referenceEntry, len(locs))
		for i, loc := range locs {
			refFile := docsync.URIToFile(string(loc.URI))
			refLine := int(loc.Range.Start.Line) + 1
			refCol := int(loc.Range.Start.Character) + 1

			entry := referenceEntry{
				File:   refFile,
				Line:   refLine,
				Column: refCol,
			}

			if preview, err := readLine(refFile, refLine); err == nil {
				entry.Preview = strings.TrimSpace(preview)
			}

			entries[i] = entry
		}

		result := referencesResult{
			References: entries,
			TotalCount: totalCount,
			Truncated:  truncated,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
