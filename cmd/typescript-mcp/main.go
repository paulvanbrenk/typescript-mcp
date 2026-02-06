package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/paulvanbrenk/typescript-mcp/internal/docsync"
	"github.com/paulvanbrenk/typescript-mcp/internal/lsp"
	"github.com/paulvanbrenk/typescript-mcp/internal/tools"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Spawn tsgo LSP server
	lspClient, err := lsp.NewClient(ctx, "")
	if err != nil {
		return fmt.Errorf("starting LSP client: %w", err)
	}
	var closeOnce sync.Once
	closeLSP := func() { closeOnce.Do(func() { lspClient.Close() }) }
	defer closeLSP()

	// Shut down the LSP client when the context is cancelled.
	go func() {
		<-ctx.Done()
		closeLSP()
	}()

	// Create document manager
	docMgr := docsync.NewManager()

	// Create MCP server
	s := server.NewMCPServer(
		"typescript-mcp",
		"0.1.0",
		server.WithInstructions(serverInstructions),
	)

	// Register all tools
	tools.Register(s, lspClient, docMgr)

	// Serve over stdio
	return server.ServeStdio(s)
}

const serverInstructions = `TypeScript type-checking and code navigation tools powered by tsgo.

Available tools:
- ts_diagnostics: Get TypeScript errors and warnings for a file
- ts_definition: Go to the definition of a symbol
- ts_hover: Get type information and documentation for a symbol
- ts_references: Find all references to a symbol across the project
- ts_rename: Rename a symbol across the project (writes changes to disk)
- ts_document_symbols: Get the symbol outline of a file
- ts_project_info: Get TypeScript project configuration info

Workflow:
1. After editing TypeScript files, use ts_diagnostics to check for type errors
2. Use ts_hover to understand types and ts_definition to navigate code
3. Use ts_references before renaming or refactoring to find all usages
4. Use ts_rename to rename symbols — it applies all changes across the project
5. Use ts_document_symbols to get a file overview without reading the full source`
