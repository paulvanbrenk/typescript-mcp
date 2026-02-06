package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/paulvanbrenk/typescript-mcp/internal/docsync"
	"github.com/paulvanbrenk/typescript-mcp/internal/lsp"
	"go.lsp.dev/protocol"
)

type editInfo struct {
	File    string `json:"file"`
	Edits   int    `json:"edits"`
	Preview string `json:"preview,omitempty"`
}

type renameResult struct {
	NewName    string     `json:"newName"`
	TotalEdits int        `json:"totalEdits"`
	Changes    []editInfo `json:"changes"`
}

func makeRenameHandler(client *lsp.Client, docs *docsync.Manager) server.ToolHandlerFunc {
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
		newName, err := request.RequireString("newName")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := docs.SyncFile(ctx, client.Conn(), file); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sync error: %v", err)), nil
		}

		edit, err := client.Rename(ctx, file, line, col, newName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("rename error: %v", err)), nil
		}

		if edit == nil || (len(edit.Changes) == 0 && len(edit.DocumentChanges) == 0) {
			return mcp.NewToolResultError("rename produced no changes"), nil
		}

		changes, err := applyWorkspaceEdit(edit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("apply error: %v", err)), nil
		}

		// Re-sync all modified files so the LSP server sees the new content.
		for filePath := range changes {
			if syncErr := docs.SyncFile(ctx, client.Conn(), filePath); syncErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("re-sync error for %s: %v", filePath, syncErr)), nil
			}
		}

		ClearFileCache()

		totalEdits := 0
		var changeList []editInfo
		for _, infos := range changes {
			for _, info := range infos {
				totalEdits += info.Edits
				changeList = append(changeList, info)
			}
		}

		result := renameResult{
			NewName:    newName,
			TotalEdits: totalEdits,
			Changes:    changeList,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// applyWorkspaceEdit applies a WorkspaceEdit to disk. It returns a map from
// file path to the edit info for that file. On any write failure, previously
// written files are rolled back to their original content.
func applyWorkspaceEdit(edit *protocol.WorkspaceEdit) (map[string][]editInfo, error) {
	// Normalize: merge DocumentChanges into the Changes map so we have a
	// single representation to process.
	merged := make(map[protocol.DocumentURI][]protocol.TextEdit)
	for docURI, edits := range edit.Changes {
		merged[docURI] = append(merged[docURI], edits...)
	}
	for _, dc := range edit.DocumentChanges {
		docURI := dc.TextDocument.URI
		merged[docURI] = append(merged[docURI], dc.Edits...)
	}

	// Read originals, compute new contents.
	type fileWork struct {
		path     string
		original []byte
		updated  []byte
		edits    []protocol.TextEdit
	}
	var work []fileWork

	for docURI, edits := range merged {
		filePath := docsync.URIToFile(string(docURI))
		original, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filePath, err)
		}
		updated, err := applyFileEdits(original, edits)
		if err != nil {
			return nil, fmt.Errorf("applying edits to %s: %w", filePath, err)
		}
		work = append(work, fileWork{
			path:     filePath,
			original: original,
			updated:  updated,
			edits:    edits,
		})
	}

	// Write all files; rollback on failure.
	var written []fileWork
	for _, w := range work {
		if err := os.WriteFile(w.path, w.updated, 0644); err != nil {
			// Rollback previously written files.
			for _, prev := range written {
				_ = os.WriteFile(prev.path, prev.original, 0644)
			}
			return nil, fmt.Errorf("writing %s: %w", w.path, err)
		}
		written = append(written, w)
	}

	// Build result info.
	result := make(map[string][]editInfo)
	for _, w := range work {
		preview := ""
		if lines := strings.SplitN(string(w.updated), "\n", int(firstEditLine(w.edits))+2); len(lines) > int(firstEditLine(w.edits)) {
			preview = strings.TrimSpace(lines[firstEditLine(w.edits)])
		}
		result[w.path] = append(result[w.path], editInfo{
			File:    w.path,
			Edits:   len(w.edits),
			Preview: preview,
		})
	}
	return result, nil
}

// firstEditLine returns the smallest line number from a set of edits.
func firstEditLine(edits []protocol.TextEdit) uint32 {
	if len(edits) == 0 {
		return 0
	}
	min := edits[0].Range.Start.Line
	for _, e := range edits[1:] {
		if e.Range.Start.Line < min {
			min = e.Range.Start.Line
		}
	}
	return min
}

// applyFileEdits applies a set of TextEdits to file content. Edits are applied
// in reverse order (bottom-up) so that earlier byte offsets remain valid.
func applyFileEdits(content []byte, edits []protocol.TextEdit) ([]byte, error) {
	// Sort edits in reverse order: by line descending, then character descending.
	sorted := make([]protocol.TextEdit, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Range.Start.Line != sorted[j].Range.Start.Line {
			return sorted[i].Range.Start.Line > sorted[j].Range.Start.Line
		}
		return sorted[i].Range.Start.Character > sorted[j].Range.Start.Character
	})

	lines := splitLines(content)

	for _, edit := range sorted {
		startLine := int(edit.Range.Start.Line)
		endLine := int(edit.Range.End.Line)

		if startLine >= len(lines) || endLine >= len(lines) {
			return nil, fmt.Errorf("edit range out of bounds: start line %d, end line %d, file has %d lines", startLine, endLine, len(lines))
		}

		startByte := utf16ColToByteOffset(lines[startLine], edit.Range.Start.Character)
		endByte := utf16ColToByteOffset(lines[endLine], edit.Range.End.Character)

		// Calculate absolute byte offsets within content.
		absStart := lineOffset(lines, startLine) + startByte
		absEnd := lineOffset(lines, endLine) + endByte

		if absStart > len(content) || absEnd > len(content) || absStart > absEnd {
			return nil, fmt.Errorf("computed byte offsets out of range: start=%d end=%d len=%d", absStart, absEnd, len(content))
		}

		var buf []byte
		buf = append(buf, content[:absStart]...)
		buf = append(buf, []byte(edit.NewText)...)
		buf = append(buf, content[absEnd:]...)
		content = buf

		// Recompute lines after mutation since offsets shift.
		lines = splitLines(content)
	}

	return content, nil
}

// splitLines splits content into lines, preserving line endings.
// Each element includes its trailing \n (or \r\n) except possibly the last.
func splitLines(content []byte) []string {
	if len(content) == 0 {
		return []string{""}
	}
	s := string(content)
	var lines []string
	for {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			lines = append(lines, s)
			break
		}
		lines = append(lines, s[:idx+1])
		s = s[idx+1:]
	}
	return lines
}

// lineOffset returns the byte offset of the start of a given line.
func lineOffset(lines []string, line int) int {
	off := 0
	for i := 0; i < line && i < len(lines); i++ {
		off += len(lines[i])
	}
	return off
}

// utf16ColToByteOffset converts a UTF-16 column offset to a byte offset within
// a line string. LSP positions use UTF-16 code units.
func utf16ColToByteOffset(line string, utf16Col uint32) int {
	utf16Count := uint32(0)
	byteOff := 0
	for byteOff < len(line) {
		if utf16Count >= utf16Col {
			break
		}
		r, size := utf8.DecodeRuneInString(line[byteOff:])
		if r <= 0xFFFF {
			utf16Count++
		} else {
			// Supplementary character: 2 UTF-16 code units.
			utf16Count += 2
		}
		byteOff += size
	}
	return byteOff
}
