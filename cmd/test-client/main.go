// test-client is a CLI for manually testing the typescript-mcp MCP server.
// It builds the server, connects via stdio, and calls a specified tool.
//
// Usage:
//
//	go run ./cmd/test-client -project /path/to/ts-project -tool ts_rename \
//	  -args '{"file":"/path/to/file.ts","line":332,"column":14,"newName":"movieRepository"}'
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	project := flag.String("project", "", "path to the TypeScript project (required)")
	tool := flag.String("tool", "", "tool name to call (required)")
	args := flag.String("args", "{}", "tool arguments as JSON object")
	binary := flag.String("binary", "", "path to typescript-mcp binary (default: build from source)")
	flag.Parse()

	if *project == "" || *tool == "" {
		flag.Usage()
		os.Exit(1)
	}

	var toolArgs map[string]any
	if err := json.Unmarshal([]byte(*args), &toolArgs); err != nil {
		log.Fatalf("Invalid -args JSON: %v", err)
	}

	bin := *binary
	if bin == "" {
		bin = buildServer()
	}

	ctx := context.Background()

	c, err := client.NewStdioMCPClientWithOptions(
		bin,
		nil,
		nil,
		transport.WithCommandFunc(func(ctx context.Context, command string, env []string, cmdArgs []string) (*exec.Cmd, error) {
			cmd := exec.CommandContext(ctx, command, cmdArgs...)
			cmd.Dir = *project
			cmd.Env = append(os.Environ(), env...)
			return cmd, nil
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	initResult, err := c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ClientInfo:      mcp.Implementation{Name: "test-client", Version: "1.0.0"},
			ProtocolVersion: "2024-11-05",
		},
	})
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Server: %s %s\n", initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	result, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      *tool,
			Arguments: toolArgs,
		},
	})
	if err != nil {
		log.Fatalf("CallTool failed: %v", err)
	}

	for _, content := range result.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			fmt.Println(tc.Text)
		}
	}
}

func buildServer() string {
	fmt.Fprintln(os.Stderr, "Building typescript-mcp...")
	bin := os.TempDir() + "/typescript-mcp"
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/typescript-mcp")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build server: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Built:", bin)
	return bin
}
