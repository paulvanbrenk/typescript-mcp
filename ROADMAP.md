# Roadmap

## v0.2 — More Tools

- [ ] `ts_rename` — rename symbol across project (first write tool)
- [ ] `ts_completions` — code completions at a position
- [ ] `ts_workspace_symbols` — search symbols across the whole project by name
- [ ] `ts_code_actions` — quick fixes and refactors (auto-import, extract function, etc.)

## v0.3 — Smarter Diagnostics

- [ ] `ts_diagnostics` with `includeDependents` — check files that import the changed file
- [ ] Watch mode — automatically re-check after file changes
- [ ] Diagnostic severity filtering (errors only, warnings only)

## v0.4 — Configuration & DX

- [ ] `--tsgo-path` flag to specify a custom tsgo binary
- [ ] `--root` flag to override workspace root
- [ ] `ts_project_info` backed by LSP instead of filesystem stub
- [ ] Multi-project support (monorepos with multiple tsconfigs)

## v0.5 — Performance

- [ ] Connection pooling for multiple concurrent tool calls
- [ ] Incremental document sync (send diffs instead of full content)
- [ ] Response streaming for large results

## Future

- [ ] Publish to Homebrew
- [ ] Pre-built binaries via GitHub Releases (goreleaser)
- [ ] Semantic search over symbols (fuzzy matching)
- [ ] Integration with other LSP servers (eslint, css, etc.)
