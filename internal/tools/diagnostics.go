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

type diagnosticEntry struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Code     any    `json:"code,omitempty"`
	Message  string `json:"message"`
}

type diagnosticsResult struct {
	Diagnostics []diagnosticEntry `json:"diagnostics"`
	TotalCount  int               `json:"totalCount"`
	Truncated   bool              `json:"truncated"`
}

func makeDiagnosticsHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		file := request.GetString("file", "")
		if file == "" {
			return mcp.NewToolResultError("file parameter is required"), nil
		}

		maxResults := request.GetInt("maxResults", 50)

		// Sync file before requesting diagnostics
		if err := docs.SyncFile(ctx, client.Conn(), file); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sync error: %v", err)), nil
		}

		diags, err := client.Diagnostic(ctx, file)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("diagnostic error: %v", err)), nil
		}

		totalCount := len(diags)
		truncated := totalCount > maxResults
		if truncated {
			diags = diags[:maxResults]
		}

		entries := make([]diagnosticEntry, len(diags))
		for i, d := range diags {
			sev := "error"
			switch d.Severity {
			case protocol.DiagnosticSeverityWarning:
				sev = "warning"
			case protocol.DiagnosticSeverityInformation:
				sev = "information"
			case protocol.DiagnosticSeverityHint:
				sev = "hint"
			}
			entries[i] = diagnosticEntry{
				File:     file,
				Line:     int(d.Range.Start.Line) + 1,
				Column:   int(d.Range.Start.Character) + 1,
				Severity: sev,
				Code:     d.Code,
				Message:  d.Message,
			}
		}

		result := diagnosticsResult{
			Diagnostics: entries,
			TotalCount:  totalCount,
			Truncated:   truncated,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
