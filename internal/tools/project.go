package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pvanbrenk/typescript-mcp/internal/docsync"
	"github.com/pvanbrenk/typescript-mcp/internal/lsp"
)

type projectInfoResult struct {
	TsconfigPath string `json:"tsconfigPath,omitempty"`
	ProjectRoot  string `json:"projectRoot,omitempty"`
}

func makeProjectInfoHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tsconfig := request.GetString("tsconfig", "")
		cwd := request.GetString("cwd", "")

		_ = client // client will be used once LSP provides project info capabilities
		_ = docs

		// If tsconfig is not specified, try to discover it
		if tsconfig == "" {
			if cwd == "" {
				var err error
				cwd, err = os.Getwd()
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("cannot determine working directory: %v", err)), nil
				}
			}
			candidate := filepath.Join(cwd, "tsconfig.json")
			if _, err := os.Stat(candidate); err == nil {
				tsconfig = candidate
			}
		}

		result := projectInfoResult{
			TsconfigPath: tsconfig,
		}

		if tsconfig != "" {
			result.ProjectRoot = filepath.Dir(tsconfig)
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
