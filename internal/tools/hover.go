package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pvanbrenk/typescript-mcp/internal/docsync"
	"github.com/pvanbrenk/typescript-mcp/internal/lsp"
)

func makeHoverHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
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

		hover, err := client.Hover(ctx, file, line, col)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("hover error: %v", err)), nil
		}

		if hover == nil {
			return mcp.NewToolResultText("No type information available"), nil
		}

		// Extract the content, keeping it concise
		content := hover.Contents.Value
		// If markdown, trim to just the type signature (first code block or first paragraph)
		if hover.Contents.Kind == "markdown" {
			content = extractConciseHover(content)
		}

		return mcp.NewToolResultText(content), nil
	}
}

// extractConciseHover extracts the type signature from markdown hover content.
// Returns the first code block content if present, otherwise the first paragraph.
func extractConciseHover(md string) string {
	lines := strings.Split(md, "\n")
	var inCodeBlock bool
	var codeLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				if len(codeLines) > 0 {
					return strings.Join(codeLines, "\n")
				}
				inCodeBlock = false
				continue
			}
			inCodeBlock = true
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
		}
	}

	// Unclosed code block — return what we accumulated
	if inCodeBlock && len(codeLines) > 0 {
		return strings.Join(codeLines, "\n")
	}

	// No code block found, return as-is
	return md
}
