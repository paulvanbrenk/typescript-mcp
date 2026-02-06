package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/paulvanbrenk/typescript-mcp/internal/docsync"
	"github.com/paulvanbrenk/typescript-mcp/internal/lsp"
)

// Register adds all TypeScript tool handlers to the MCP server.
func Register(s *server.MCPServer, client *lsp.Client, docs *docsync.Manager) {
	s.AddTool(mcp.NewTool("ts_diagnostics",
		mcp.WithDescription("Get TypeScript errors and warnings. Use after editing code to check for type errors."),
		mcp.WithString("file", mcp.Description("Absolute path to check a single file")),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json (auto-detected if omitted)")),
		mcp.WithNumber("maxResults", mcp.Description("Maximum errors to return (default 50)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), makeDiagnosticsHandler(client, docs))

	s.AddTool(mcp.NewTool("ts_definition",
		mcp.WithDescription("Go to definition of a symbol. Returns file and position where the symbol is defined, with a preview of the source line."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Absolute file path")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line number (1-based)")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column number (1-based)")),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), makeDefinitionHandler(client, docs))

	s.AddTool(mcp.NewTool("ts_hover",
		mcp.WithDescription("Get type information and documentation for a symbol at a position. Returns the resolved type signature."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Absolute file path")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line number (1-based)")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column number (1-based)")),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), makeHoverHandler(client, docs))

	s.AddTool(mcp.NewTool("ts_references",
		mcp.WithDescription("Find all references to a symbol across the project. Returns every location where the symbol is used."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Absolute file path")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line number (1-based)")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column number (1-based)")),
		mcp.WithNumber("maxResults", mcp.Description("Maximum references to return (default 50)")),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), makeReferencesHandler(client, docs))

	s.AddTool(mcp.NewTool("ts_document_symbols",
		mcp.WithDescription("Get the symbol outline of a file. Returns a tree of all functions, classes, interfaces, and variables with their types."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Absolute file path")),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), makeDocumentSymbolsHandler(client, docs))

	s.AddTool(mcp.NewTool("ts_rename",
		mcp.WithDescription("Rename a symbol across the project. Applies all changes to disk and returns a summary of modified files."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Absolute file path containing the symbol")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line number (1-based)")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column number (1-based)")),
		mcp.WithString("newName", mcp.Required(), mcp.Description("New name for the symbol")),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
	), makeRenameHandler(client, docs))

	s.AddTool(mcp.NewTool("ts_project_info",
		mcp.WithDescription("Get TypeScript project configuration info. Returns tsconfig path and project root directory."),
		mcp.WithString("tsconfig", mcp.Description("Path to tsconfig.json")),
		mcp.WithString("cwd", mcp.Description("Working directory for tsconfig discovery")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), makeProjectInfoHandler(client, docs))
}
