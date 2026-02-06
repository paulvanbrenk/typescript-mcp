package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

// Client wraps a JSON-RPC connection to tsgo's LSP server.
type Client struct {
	conn    jsonrpc2.Conn
	server  protocol.Server
	process *TsgoProcess
	rootURI string

	// diagnostics stores push diagnostics received from the server.
	diagMu      sync.Mutex
	diagnostics map[string][]protocol.Diagnostic // URI -> diagnostics
}

// NewClient spawns tsgo and establishes an LSP connection.
// rootURI is the workspace root URI (e.g. "file:///path/to/project").
// If empty, the current working directory is used.
func NewClient(ctx context.Context, rootURI string) (*Client, error) {
	if rootURI == "" {
		if cwd, err := os.Getwd(); err == nil {
			rootURI = string(uri.File(cwd))
		}
	}

	proc, err := StartTsgo(ctx)
	if err != nil {
		return nil, fmt.Errorf("start tsgo: %w", err)
	}

	rwc := &readWriteCloser{
		reader: proc.stdout,
		writer: proc.stdin,
	}
	stream := jsonrpc2.NewStream(rwc)

	c := &Client{
		process:     proc,
		rootURI:     rootURI,
		diagnostics: make(map[string][]protocol.Diagnostic),
	}

	var logger *zap.Logger
	if os.Getenv("TYPESCRIPT_MCP_DEBUG") != "" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger = zap.NewNop()
	}

	// NewClient creates a connection where:
	// - We are the "client" handling server-initiated notifications (publishDiagnostics, etc.)
	// - We get back a "server" dispatcher to send requests to tsgo
	_, conn, server := protocol.NewClient(ctx, c, stream, logger)
	c.conn = conn
	c.server = server

	if err := c.initialize(ctx); err != nil {
		_ = proc.Stop()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	return c, nil
}

// Conn returns the underlying JSON-RPC connection for sending notifications.
func (c *Client) Conn() jsonrpc2.Conn {
	return c.conn
}

// initialize performs the LSP initialize handshake.
func (c *Client) initialize(ctx context.Context) error {
	pid := int32(os.Getpid())

	result, err := c.server.Initialize(ctx, &protocol.InitializeParams{
		ProcessID: pid,
		RootURI:   protocol.DocumentURI(c.rootURI),
		ClientInfo: &protocol.ClientInfo{
			Name:    "typescript-mcp",
			Version: "0.1.0",
		},
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Synchronization: &protocol.TextDocumentSyncClientCapabilities{
					DynamicRegistration: false,
					DidSave:             true,
					WillSave:            false,
					WillSaveWaitUntil:   false,
				},
				Hover: &protocol.HoverTextDocumentClientCapabilities{
					ContentFormat: []protocol.MarkupKind{protocol.Markdown, protocol.PlainText},
				},
				PublishDiagnostics: &protocol.PublishDiagnosticsClientCapabilities{
					RelatedInformation: true,
				},
				DocumentSymbol: &protocol.DocumentSymbolClientCapabilities{
					HierarchicalDocumentSymbolSupport: true,
				},
				Rename: &protocol.RenameClientCapabilities{
					PrepareSupport: false,
				},
			},
			Workspace: &protocol.WorkspaceClientCapabilities{
				WorkspaceEdit: &protocol.WorkspaceClientCapabilitiesWorkspaceEdit{
					DocumentChanges: false,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}
	_ = result // Server capabilities available if needed later

	if err := c.server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}

	return nil
}

// Hover returns hover information for a position in a file.
// Line and column are 1-based (converted to 0-based for LSP).
func (c *Client) Hover(ctx context.Context, file string, line, col int) (*protocol.Hover, error) {
	if line < 1 || col < 1 {
		return nil, fmt.Errorf("line and column must be >= 1, got line=%d col=%d", line, col)
	}
	return c.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: makePosition(file, line, col),
	})
}

// Definition returns the definition location(s) for a symbol.
// Line and column are 1-based (converted to 0-based for LSP).
func (c *Client) Definition(ctx context.Context, file string, line, col int) ([]protocol.Location, error) {
	if line < 1 || col < 1 {
		return nil, fmt.Errorf("line and column must be >= 1, got line=%d col=%d", line, col)
	}
	return c.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: makePosition(file, line, col),
	})
}

// References returns all reference locations for a symbol.
// Line and column are 1-based (converted to 0-based for LSP).
func (c *Client) References(ctx context.Context, file string, line, col int) ([]protocol.Location, error) {
	if line < 1 || col < 1 {
		return nil, fmt.Errorf("line and column must be >= 1, got line=%d col=%d", line, col)
	}
	return c.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: makePosition(file, line, col),
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	})
}

// Rename renames a symbol at the given position.
// Line and column are 1-based (converted to 0-based for LSP).
func (c *Client) Rename(ctx context.Context, file string, line, col int, newName string) (*protocol.WorkspaceEdit, error) {
	if line < 1 || col < 1 {
		return nil, fmt.Errorf("line and column must be >= 1, got line=%d col=%d", line, col)
	}
	return c.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: makePosition(file, line, col),
		NewName:                    newName,
	})
}

// DocumentSymbol returns the document symbols for a file.
func (c *Client) DocumentSymbol(ctx context.Context, file string) ([]protocol.DocumentSymbol, error) {
	docURI := uri.File(file)
	raw, err := c.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(docURI),
		},
	})
	if err != nil {
		return nil, err
	}

	// The response can be DocumentSymbol[] (hierarchical, has Range) or
	// SymbolInformation[] (flat, has Location.Range). We request hierarchical
	// via capabilities, but handle both formats as a safety net.
	var symbols []protocol.DocumentSymbol
	for _, item := range raw {
		if sym, ok := parseDocumentSymbolItem(item); ok {
			symbols = append(symbols, sym)
		}
	}
	return symbols, nil
}

// Diagnostic returns diagnostics for a file.
// It first tries pull diagnostics (textDocument/diagnostic), then falls back
// to any push diagnostics received via publishDiagnostics.
func (c *Client) Diagnostic(ctx context.Context, file string) ([]protocol.Diagnostic, error) {
	docURI := uri.File(file)

	// Try pull diagnostics via raw JSON-RPC call.
	type documentDiagnosticParams struct {
		TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	}
	type fullDocumentDiagnosticReport struct {
		Kind  string                `json:"kind"`
		Items []protocol.Diagnostic `json:"items"`
	}

	var report fullDocumentDiagnosticReport
	_, err := c.conn.Call(ctx, "textDocument/diagnostic", &documentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(docURI),
		},
	}, &report)
	if err == nil {
		return report.Items, nil
	}

	// Fall back to push diagnostics.
	c.diagMu.Lock()
	diags := c.diagnostics[string(docURI)]
	c.diagMu.Unlock()
	return diags, nil
}

// Close shuts down the LSP connection and tsgo process.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send shutdown request (best effort - still try to stop the process).
	_ = c.server.Shutdown(ctx)

	// Send exit notification.
	_ = c.server.Exit(ctx)

	// Close the JSON-RPC connection.
	_ = c.conn.Close()

	// Stop the process.
	return c.process.Stop()
}

// --- protocol.Client implementation (server-initiated callbacks) ---

func (c *Client) Progress(_ context.Context, _ *protocol.ProgressParams) error {
	return nil
}

func (c *Client) WorkDoneProgressCreate(_ context.Context, _ *protocol.WorkDoneProgressCreateParams) error {
	return nil
}

func (c *Client) LogMessage(_ context.Context, _ *protocol.LogMessageParams) error {
	return nil
}

func (c *Client) PublishDiagnostics(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
	c.diagMu.Lock()
	c.diagnostics[string(params.URI)] = params.Diagnostics
	c.diagMu.Unlock()
	return nil
}

func (c *Client) ShowMessage(_ context.Context, _ *protocol.ShowMessageParams) error {
	return nil
}

func (c *Client) ShowMessageRequest(_ context.Context, _ *protocol.ShowMessageRequestParams) (*protocol.MessageActionItem, error) {
	return nil, nil
}

func (c *Client) Telemetry(_ context.Context, _ interface{}) error {
	return nil
}

func (c *Client) RegisterCapability(_ context.Context, _ *protocol.RegistrationParams) error {
	return nil
}

func (c *Client) UnregisterCapability(_ context.Context, _ *protocol.UnregistrationParams) error {
	return nil
}

func (c *Client) ApplyEdit(_ context.Context, _ *protocol.ApplyWorkspaceEditParams) (bool, error) {
	return false, nil
}

func (c *Client) Configuration(_ context.Context, _ *protocol.ConfigurationParams) ([]interface{}, error) {
	return nil, nil
}

func (c *Client) WorkspaceFolders(_ context.Context) ([]protocol.WorkspaceFolder, error) {
	return nil, nil
}

// parseDocumentSymbolItem parses a single item from the textDocument/documentSymbol response.
// It handles both DocumentSymbol (hierarchical) and SymbolInformation (flat) formats.
func parseDocumentSymbolItem(item interface{}) (protocol.DocumentSymbol, bool) {
	b, err := json.Marshal(item)
	if err != nil {
		slog.Debug("DocumentSymbol: failed to marshal item", "error", err)
		return protocol.DocumentSymbol{}, false
	}

	// Detect format: SymbolInformation has a "location" key,
	// DocumentSymbol has "range" and "selectionRange" keys.
	var probe struct {
		Location *protocol.Location `json:"location"`
	}
	if err := json.Unmarshal(b, &probe); err == nil && probe.Location != nil {
		// SymbolInformation format — extract range from location.
		var si protocol.SymbolInformation
		if err := json.Unmarshal(b, &si); err != nil {
			slog.Debug("DocumentSymbol: failed to unmarshal SymbolInformation", "error", err)
			return protocol.DocumentSymbol{}, false
		}
		return protocol.DocumentSymbol{
			Name:           si.Name,
			Kind:           si.Kind,
			Tags:           si.Tags,
			Deprecated:     si.Deprecated,
			Range:          si.Location.Range,
			SelectionRange: si.Location.Range,
		}, true
	}

	// DocumentSymbol format.
	var sym protocol.DocumentSymbol
	if err := json.Unmarshal(b, &sym); err != nil {
		slog.Debug("DocumentSymbol: failed to unmarshal DocumentSymbol", "error", err)
		return protocol.DocumentSymbol{}, false
	}
	return sym, true
}

// --- helpers ---

// makePosition creates a TextDocumentPositionParams converting 1-based line/col to 0-based.
func makePosition(file string, line, col int) protocol.TextDocumentPositionParams {
	docURI := uri.File(file)
	return protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(docURI),
		},
		Position: protocol.Position{
			Line:      uint32(line - 1),
			Character: uint32(col - 1),
		},
	}
}

// readWriteCloser combines separate reader and writer into io.ReadWriteCloser.
type readWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (rwc *readWriteCloser) Read(p []byte) (int, error) {
	return rwc.reader.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (int, error) {
	return rwc.writer.Write(p)
}

func (rwc *readWriteCloser) Close() error {
	rerr := rwc.reader.Close()
	werr := rwc.writer.Close()
	if rerr != nil {
		return rerr
	}
	return werr
}
