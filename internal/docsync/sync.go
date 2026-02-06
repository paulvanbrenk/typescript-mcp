package docsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// trackedDoc holds the state for a document that has been opened with the LSP server.
type trackedDoc struct {
	version int32
	content string
}

// Manager tracks open documents and synchronizes them with the LSP server.
type Manager struct {
	mu   sync.Mutex
	docs map[string]*trackedDoc // URI -> tracked state
}

// NewManager creates a new document manager.
func NewManager() *Manager {
	return &Manager{
		docs: make(map[string]*trackedDoc),
	}
}

// SyncFile ensures the LSP server has the current content for the given file path.
// It reads the file from disk and sends textDocument/didOpen if the file is new,
// or textDocument/didChange if the content has changed.
func (m *Manager) SyncFile(ctx context.Context, conn jsonrpc2.Conn, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}

	docURI := FileToURI(filePath)
	text := string(content)

	// Determine what notification to send while holding the lock,
	// then release before doing network I/O.
	type notification struct {
		method string
		params interface{}
	}

	var notif *notification

	m.mu.Lock()
	tracked, exists := m.docs[docURI]
	if !exists {
		m.docs[docURI] = &trackedDoc{version: 1, content: text}
		notif = &notification{
			method: protocol.MethodTextDocumentDidOpen,
			params: &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:        protocol.DocumentURI(docURI),
					LanguageID: languageIDFromPath(filePath),
					Version:    1,
					Text:       text,
				},
			},
		}
	} else if tracked.content != text {
		tracked.version++
		tracked.content = text
		notif = &notification{
			method: protocol.MethodTextDocumentDidChange,
			params: &protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: protocol.DocumentURI(docURI),
					},
					Version: tracked.version,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					{Text: text},
				},
			},
		}
	}
	m.mu.Unlock()

	if notif == nil {
		return nil
	}
	return conn.Notify(ctx, notif.method, notif.params)
}

// languageIDFromPath returns the LSP language identifier for a file path.
func languageIDFromPath(filePath string) protocol.LanguageIdentifier {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".tsx":
		return protocol.TypeScriptReactLanguage
	case ".js":
		return protocol.JavaScriptLanguage
	case ".jsx":
		return protocol.JavaScriptReactLanguage
	default:
		return protocol.TypeScriptLanguage
	}
}

// SyncFiles synchronizes multiple files with the LSP server.
func (m *Manager) SyncFiles(ctx context.Context, conn jsonrpc2.Conn, paths []string) error {
	for _, p := range paths {
		if err := m.SyncFile(ctx, conn, p); err != nil {
			return err
		}
	}
	return nil
}

// Close sends textDocument/didClose for all tracked documents.
func (m *Manager) Close(ctx context.Context, conn jsonrpc2.Conn) error {
	m.mu.Lock()
	uris := make([]string, 0, len(m.docs))
	for u := range m.docs {
		uris = append(uris, u)
	}
	m.docs = make(map[string]*trackedDoc)
	m.mu.Unlock()

	for _, u := range uris {
		if err := conn.Notify(ctx, protocol.MethodTextDocumentDidClose, &protocol.DidCloseTextDocumentParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(u),
			},
		}); err != nil {
			return err
		}
	}
	return nil
}
