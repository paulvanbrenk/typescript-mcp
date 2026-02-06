package tools

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

// readLine reads a specific 1-based line number from a file.
func readLine(file string, lineNum int) (string, error) {
	lines, err := cachedReadLines(file)
	if err != nil {
		return "", err
	}
	if lineNum < 1 || lineNum > len(lines) {
		return "", fmt.Errorf("line %d out of range in %s (1-%d)", lineNum, file, len(lines))
	}
	return lines[lineNum-1], nil
}

// fileLineCache caches file contents for the duration of a tool call batch.
// This avoids re-reading the same file for each reference/definition preview.
var (
	fileLineCacheMu sync.Mutex
	fileLineCache   = make(map[string][]string)
)

// cachedReadLines returns all lines of a file, caching the result.
func cachedReadLines(file string) ([]string, error) {
	fileLineCacheMu.Lock()
	if lines, ok := fileLineCache[file]; ok {
		fileLineCacheMu.Unlock()
		return lines, nil
	}
	fileLineCacheMu.Unlock()

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	fileLineCacheMu.Lock()
	fileLineCache[file] = lines
	fileLineCacheMu.Unlock()

	return lines, nil
}

// ClearFileCache clears the file line cache. Call between tool invocations
// if freshness is needed, though typically files don't change mid-batch.
func ClearFileCache() {
	fileLineCacheMu.Lock()
	fileLineCache = make(map[string][]string)
	fileLineCacheMu.Unlock()
}
