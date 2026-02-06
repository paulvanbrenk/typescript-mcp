package lsp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TsgoProcess manages a running tsgo --lsp --stdio process.
type TsgoProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// StartTsgo spawns tsgo --lsp --stdio and returns a handle to the process.
func StartTsgo(ctx context.Context) (*TsgoProcess, error) {
	bin, err := resolveTsgo()
	if err != nil {
		return nil, fmt.Errorf("resolve tsgo: %w", err)
	}

	cmd := exec.CommandContext(ctx, bin, "--lsp", "--stdio")
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tsgo: %w", err)
	}

	p := &TsgoProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	// Drain stderr to logger in background.
	go p.drainStderr()

	return p, nil
}

// Stop gracefully shuts down the tsgo process.
// It closes stdin and waits for the process to exit, killing it after a timeout.
func (p *TsgoProcess) Stop() error {
	// Close stdin to signal EOF.
	_ = p.stdin.Close()

	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		select {
		case err := <-done:
			return err
		case <-time.After(2 * time.Second):
			return fmt.Errorf("tsgo process did not exit after kill")
		}
	}
}

func (p *TsgoProcess) drainStderr() {
	buf := make([]byte, 4096)
	for {
		n, err := p.stderr.Read(buf)
		if n > 0 {
			slog.Debug("tsgo stderr", "output", string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

// resolveTsgo finds the tsgo binary, checking PATH first then common locations.
func resolveTsgo() (string, error) {
	// Check PATH first.
	if path, err := exec.LookPath("tsgo"); err == nil {
		return path, nil
	}

	// Try common install locations.
	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".npm", "_npx", "tsgo"),
			filepath.Join(home, ".local", "bin", "tsgo"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}

	return "", fmt.Errorf("tsgo not found in PATH or common locations; install with: npm install -g @typescript/native-preview")
}
