package docsync

import (
	"go.lsp.dev/uri"
)

// FileToURI converts an absolute file path to a file:// URI.
func FileToURI(path string) string {
	return string(uri.File(path))
}

// URIToFile converts a file:// URI to a file path.
func URIToFile(u string) string {
	return uri.URI(u).Filename()
}
