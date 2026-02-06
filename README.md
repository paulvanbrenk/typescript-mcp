# typescript-mcp

TypeScript type checking and code navigation for AI coding agents, powered by tsgo.

## Architecture

```
Coding Agent (Claude Code, etc.)
     |  MCP (stdio)
typescript-mcp (Go)
     |  LSP JSON-RPC (stdio)
tsgo (TypeScript 7, native Go compiler)
```

`typescript-mcp` is a Go MCP server that bridges
[Model Context Protocol](https://modelcontextprotocol.io/) tool calls to
[tsgo](https://github.com/microsoft/typescript-go)'s built-in LSP
server. It gives coding agents real TypeScript type checking and code navigation
without relying on regex or heuristics.

The server spawns `tsgo --lsp --stdio` as a child process, communicates over
JSON-RPC, and translates LSP responses into concise, agent-friendly JSON.

## Prerequisites

- **Go 1.24+**
- **tsgo** -- install globally or use npx:
  ```bash
  npm install -g @typescript/native-preview
  # or run without installing:
  npx @typescript/native-preview --version
  ```

## Installation

```bash
go install github.com/pvanbrenk/typescript-mcp/cmd/typescript-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/pvanbrenk/typescript-mcp.git
cd typescript-mcp
go build -o typescript-mcp ./cmd/typescript-mcp
```

## Configuration for Claude Code

Add the following to your MCP configuration (`.mcp.json` or Claude Code settings):

```json
{
  "mcpServers": {
    "typescript": {
      "command": "typescript-mcp",
      "args": []
    }
  }
}
```

If `typescript-mcp` is not on your `PATH`, use the full path to the binary.

## Tools Reference

All tools are read-only and non-destructive. Line and column numbers are **1-based**.

### ts_diagnostics

Get TypeScript errors and warnings for a file.

| Parameter    | Type   | Required | Description                                  |
|-------------|--------|----------|----------------------------------------------|
| `file`      | string | yes      | Absolute path to check a single file         |
| `tsconfig`  | string | no       | Path to tsconfig.json (auto-detected if omitted) |
| `maxResults`| number | no       | Maximum errors to return (default 50)        |

**Example request:**

```json
{
  "file": "/home/user/project/src/index.ts",
  "maxResults": 10
}
```

**Example response:**

```json
{
  "diagnostics": [
    {
      "file": "/home/user/project/src/index.ts",
      "line": 12,
      "column": 5,
      "severity": "error",
      "code": 2322,
      "message": "Type 'string' is not assignable to type 'number'."
    }
  ],
  "totalCount": 1,
  "truncated": false
}
```

### ts_definition

Go to the definition of a symbol. Returns the file and position where the symbol
is defined, with a preview of the source line.

| Parameter  | Type   | Required | Description                  |
|-----------|--------|----------|------------------------------|
| `file`    | string | yes      | Absolute file path           |
| `line`    | number | yes      | Line number (1-based)        |
| `column`  | number | yes      | Column number (1-based)      |
| `tsconfig`| string | no       | Path to tsconfig.json        |

**Example request:**

```json
{
  "file": "/home/user/project/src/index.ts",
  "line": 10,
  "column": 15
}
```

**Example response:**

```json
[
  {
    "file": "/home/user/project/src/utils.ts",
    "line": 3,
    "column": 17,
    "preview": "export function formatDate(date: Date): string {"
  }
]
```

### ts_hover

Get type information and documentation for a symbol at a position. Returns the
resolved type signature.

| Parameter  | Type   | Required | Description                  |
|-----------|--------|----------|------------------------------|
| `file`    | string | yes      | Absolute file path           |
| `line`    | number | yes      | Line number (1-based)        |
| `column`  | number | yes      | Column number (1-based)      |
| `tsconfig`| string | no       | Path to tsconfig.json        |

**Example request:**

```json
{
  "file": "/home/user/project/src/index.ts",
  "line": 5,
  "column": 10
}
```

**Example response (plain text):**

```
(property) AppConfig.port: number
```

The response is the extracted type signature from the hover content. Markdown
code fences are stripped to return just the type information.

### ts_references

Find all references to a symbol across the project. Returns every location where
the symbol is used, including the declaration.

| Parameter    | Type   | Required | Description                              |
|-------------|--------|----------|------------------------------------------|
| `file`      | string | yes      | Absolute file path                       |
| `line`      | number | yes      | Line number (1-based)                    |
| `column`    | number | yes      | Column number (1-based)                  |
| `maxResults`| number | no       | Maximum references to return (default 50)|
| `tsconfig`  | string | no       | Path to tsconfig.json                    |

**Example request:**

```json
{
  "file": "/home/user/project/src/utils.ts",
  "line": 3,
  "column": 17
}
```

**Example response:**

```json
{
  "references": [
    {
      "file": "/home/user/project/src/utils.ts",
      "line": 3,
      "column": 17,
      "preview": "export function formatDate(date: Date): string {"
    },
    {
      "file": "/home/user/project/src/index.ts",
      "line": 10,
      "column": 15,
      "preview": "const result = formatDate(new Date());"
    }
  ],
  "totalCount": 2,
  "truncated": false
}
```

### ts_document_symbols

Get the symbol outline of a file. Returns a tree of all functions, classes,
interfaces, and variables with their types.

| Parameter  | Type   | Required | Description                  |
|-----------|--------|----------|------------------------------|
| `file`    | string | yes      | Absolute file path           |
| `tsconfig`| string | no       | Path to tsconfig.json        |

**Example request:**

```json
{
  "file": "/home/user/project/src/utils.ts"
}
```

**Example response:**

```json
[
  {
    "name": "formatDate",
    "kind": "function",
    "line": 3,
    "detail": "(date: Date) => string"
  },
  {
    "name": "AppConfig",
    "kind": "interface",
    "line": 8,
    "children": [
      {
        "name": "port",
        "kind": "property",
        "line": 9,
        "detail": "number"
      },
      {
        "name": "host",
        "kind": "property",
        "line": 10,
        "detail": "string"
      }
    ]
  }
]
```

### ts_project_info

Get TypeScript project configuration info. Returns the tsconfig path and project
root directory.

| Parameter  | Type   | Required | Description                                |
|-----------|--------|----------|--------------------------------------------|
| `tsconfig`| string | no       | Path to tsconfig.json                      |
| `cwd`     | string | no       | Working directory for tsconfig discovery   |

**Example request:**

```json
{
  "cwd": "/home/user/project"
}
```

**Example response:**

```json
{
  "tsconfigPath": "/home/user/project/tsconfig.json",
  "projectRoot": "/home/user/project"
}
```

## Workflow Examples

### Edit-check-fix cycle

After editing TypeScript files, use `ts_diagnostics` to check for type errors,
fix them, and verify again:

1. Edit a file (e.g., change a function signature)
2. Call `ts_diagnostics` with the file path to get errors
3. Fix the reported errors
4. Call `ts_diagnostics` again to confirm zero errors

### Code exploration

Navigate unfamiliar code using symbols, hover, and go-to-definition:

1. Call `ts_document_symbols` to get an overview of a file's structure
2. Call `ts_hover` on an interesting symbol to see its type signature
3. Call `ts_definition` to jump to the symbol's source definition

### Safe refactoring

Before renaming or restructuring, find all usages first:

1. Call `ts_references` on the symbol you want to change
2. Edit all returned locations
3. Call `ts_diagnostics` on affected files to verify correctness

## Environment Variables

| Variable                 | Description                                      |
|-------------------------|--------------------------------------------------|
| `TYPESCRIPT_MCP_DEBUG`  | Set to `1` to enable verbose debug logging (uses zap development logger) |

## Development

### Build

```bash
go build ./cmd/typescript-mcp
```

### Test

```bash
go test ./...
```

### Run locally

```bash
# Ensure tsgo is available
tsgo --version

# Run the MCP server (communicates over stdio)
./typescript-mcp
```

### Project structure

```
cmd/typescript-mcp/     Entry point and MCP server setup
internal/
  lsp/                  LSP client and tsgo process management
    client.go           JSON-RPC connection, LSP method wrappers
    process.go          tsgo process lifecycle (spawn, stop, resolve)
  docsync/              Document synchronization with the LSP server
    sync.go             Open/change/close notifications
    uri.go              File path <-> URI conversion
  tools/                MCP tool handlers
    tools.go            Tool registration (schemas and descriptions)
    diagnostics.go      ts_diagnostics handler
    definition.go       ts_definition handler
    hover.go            ts_hover handler
    references.go       ts_references handler
    symbols.go          ts_document_symbols handler
    project.go          ts_project_info handler
    util.go             Shared utilities (readLine)
```

## License

See [LICENSE](LICENSE) for details.
