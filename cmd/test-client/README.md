# test-client

A CLI for manually testing the typescript-mcp MCP server against real TypeScript projects.

## Usage

```bash
go run ./cmd/test-client \
  -project /path/to/ts-project \
  -tool <tool-name> \
  -args '<json-arguments>'
```

## Flags

| Flag | Required | Description |
|------|----------|-------------|
| `-project` | yes | Path to the TypeScript project |
| `-tool` | yes | MCP tool name to call |
| `-args` | no | Tool arguments as a JSON object (default: `{}`) |
| `-binary` | no | Path to a pre-built `typescript-mcp` binary. If omitted, builds from source automatically |

## Examples

### Rename a symbol

```bash
go run ./cmd/test-client \
  -project ~/src/my-project \
  -tool ts_rename \
  -args '{"file":"/absolute/path/to/file.ts","line":10,"column":5,"newName":"newSymbolName"}'
```

### Get hover info

```bash
go run ./cmd/test-client \
  -project ~/src/my-project \
  -tool ts_hover \
  -args '{"file":"/absolute/path/to/file.ts","line":10,"column":5}'
```

### Find references

```bash
go run ./cmd/test-client \
  -project ~/src/my-project \
  -tool ts_references \
  -args '{"file":"/absolute/path/to/file.ts","line":10,"column":5}'
```

## Notes

- The `-args` JSON values for `file` must be absolute paths.
- `line` and `column` are 1-based.
- The server binary is built to a temp directory automatically unless `-binary` is provided.
- Write tools like `ts_rename` modify files on disk. Use `git restore` to revert.
