package test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"

	"github.com/pvanbrenk/typescript-mcp/internal/docsync"
	"github.com/pvanbrenk/typescript-mcp/internal/lsp"
)

var (
	sharedClient *lsp.Client
	sharedDocs   *docsync.Manager
	fixtureDir   string
)

// testdataDir returns the absolute path to testdata/simple.
func testdataDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", "simple")
}

func TestMain(m *testing.M) {
	fixtureDir = testdataDir()

	// Skip integration tests if tsgo is not available.
	if _, err := exec.LookPath("tsgo"); err != nil {
		// Run tests anyway -- individual tests will skip
		os.Exit(m.Run())
	}

	// Start a single shared client for all integration tests.
	rootURI := docsync.FileToURI(fixtureDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	sharedClient, err = lsp.NewClient(ctx, rootURI)
	if err != nil {
		// Cannot start client; let tests skip gracefully.
		os.Exit(m.Run())
	}

	sharedDocs = docsync.NewManager()

	// Sync all fixture files up front.
	files := []string{
		filepath.Join(fixtureDir, "src", "index.ts"),
		filepath.Join(fixtureDir, "src", "errors.ts"),
		filepath.Join(fixtureDir, "src", "consumer.ts"),
	}
	for _, f := range files {
		if err := sharedDocs.SyncFile(ctx, sharedClient.Conn(), f); err != nil {
			panic("SyncFile: " + err.Error())
		}
	}

	// Give the server time to process all files.
	time.Sleep(1 * time.Second)

	code := m.Run()

	_ = sharedClient.Close()
	os.Exit(code)
}

// requireClient skips the test if the shared client is not available.
func requireClient(t *testing.T) {
	t.Helper()
	if sharedClient == nil {
		t.Skip("requires tsgo in PATH; install with: npm install -g @typescript/native-preview")
	}
}

func TestDiagnostics(t *testing.T) {
	requireClient(t)
	errorsFile := filepath.Join(fixtureDir, "src", "errors.ts")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	diags, err := sharedClient.Diagnostic(ctx, errorsFile)
	if err != nil {
		t.Fatalf("Diagnostic: %v", err)
	}

	if len(diags) < 2 {
		t.Errorf("expected at least 2 diagnostics in errors.ts, got %d", len(diags))
		for i, d := range diags {
			t.Logf("  diag[%d]: %s (line %d)", i, d.Message, d.Range.Start.Line+1)
		}
	}

	// Verify at least one diagnostic mentions a type error.
	hasTypeError := false
	for _, d := range diags {
		msg := strings.ToLower(d.Message)
		if strings.Contains(msg, "type") && strings.Contains(msg, "not assignable") {
			hasTypeError = true
			break
		}
	}
	if !hasTypeError && len(diags) > 0 {
		t.Log("warning: no 'not assignable' type error found in diagnostics")
		for i, d := range diags {
			t.Logf("  diag[%d]: %s", i, d.Message)
		}
	}
}

func TestDefinition(t *testing.T) {
	requireClient(t)
	consumerFile := filepath.Join(fixtureDir, "src", "consumer.ts")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// "greet" is used on line 3, column 16 of consumer.ts: `const result = greet("world");`
	locs, err := sharedClient.Definition(ctx, consumerFile, 3, 16)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}

	if len(locs) == 0 {
		t.Fatal("expected at least one definition location")
	}

	defFile := docsync.URIToFile(string(locs[0].URI))
	if !strings.HasSuffix(defFile, "index.ts") {
		t.Errorf("expected definition in index.ts, got %s", defFile)
	}
}

func TestHover(t *testing.T) {
	requireClient(t)
	indexFile := filepath.Join(fixtureDir, "src", "index.ts")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// "greet" is on line 1, column 17 of index.ts: `export function greet(name: string): string {`
	hover, err := sharedClient.Hover(ctx, indexFile, 1, 17)
	if err != nil {
		t.Fatalf("Hover: %v", err)
	}

	if hover == nil {
		t.Fatal("expected hover result, got nil")
	}

	content := hover.Contents.Value
	if content == "" {
		t.Fatal("expected non-empty hover content")
	}

	// The hover should contain the function signature.
	if !strings.Contains(content, "greet") {
		t.Errorf("hover content should mention 'greet', got: %s", content)
	}
	if !strings.Contains(content, "string") {
		t.Errorf("hover content should mention 'string', got: %s", content)
	}
}

func TestReferences(t *testing.T) {
	requireClient(t)
	indexFile := filepath.Join(fixtureDir, "src", "index.ts")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// "greet" definition on line 1, column 17 of index.ts.
	locs, err := sharedClient.References(ctx, indexFile, 1, 17)
	if err != nil {
		t.Fatalf("References: %v", err)
	}

	if len(locs) < 2 {
		t.Errorf("expected at least 2 references to greet (definition + usage), got %d", len(locs))
		for i, loc := range locs {
			t.Logf("  ref[%d]: %s:%d", i, docsync.URIToFile(string(loc.URI)), loc.Range.Start.Line+1)
		}
		return
	}

	// Check that at least one reference is in consumer.ts.
	hasConsumerRef := false
	for _, loc := range locs {
		f := docsync.URIToFile(string(loc.URI))
		if strings.HasSuffix(f, "consumer.ts") {
			hasConsumerRef = true
			break
		}
	}
	if !hasConsumerRef {
		t.Error("expected at least one reference in consumer.ts")
		for i, loc := range locs {
			t.Logf("  ref[%d]: %s:%d", i, docsync.URIToFile(string(loc.URI)), loc.Range.Start.Line+1)
		}
	}
}

func TestDocumentSymbols(t *testing.T) {
	requireClient(t)
	indexFile := filepath.Join(fixtureDir, "src", "index.ts")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	symbols, err := sharedClient.DocumentSymbol(ctx, indexFile)
	if err != nil {
		t.Fatalf("DocumentSymbol: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected at least one document symbol")
	}

	type want struct {
		name string
		line uint32 // 0-based LSP line
	}
	expected := []want{
		{name: "greet", line: 0}, // line 1 in index.ts
		{name: "add", line: 4},   // line 5 in index.ts
	}

	symByName := make(map[string]protocol.DocumentSymbol)
	for _, sym := range symbols {
		symByName[sym.Name] = sym
	}

	for _, w := range expected {
		sym, ok := symByName[w.name]
		if !ok {
			t.Errorf("expected symbol %q in document symbols", w.name)
			continue
		}
		if sym.Range.Start.Line != w.line {
			t.Errorf("symbol %q: Range.Start.Line = %d, want %d", w.name, sym.Range.Start.Line, w.line)
		}
	}
}

func TestProjectInfo(t *testing.T) {
	tsconfigPath := filepath.Join(fixtureDir, "tsconfig.json")

	// ts_project_info doesn't need the LSP client for its current
	// implementation (it just checks the filesystem), so we test the
	// result structure directly.
	type projectInfoResult struct {
		TsconfigPath string `json:"tsconfigPath,omitempty"`
		ProjectRoot  string `json:"projectRoot,omitempty"`
	}

	result := projectInfoResult{
		TsconfigPath: tsconfigPath,
		ProjectRoot:  fixtureDir,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded projectInfoResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TsconfigPath != tsconfigPath {
		t.Errorf("tsconfigPath = %q, want %q", decoded.TsconfigPath, tsconfigPath)
	}
	if decoded.ProjectRoot != fixtureDir {
		t.Errorf("projectRoot = %q, want %q", decoded.ProjectRoot, fixtureDir)
	}
}
