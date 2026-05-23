//go:build !wasm

package vfs

import (
	"io"
	"os"
)

// Transport is a bidirectional stream for LSP communication.
type Transport interface {
	io.ReadWriteCloser
}

// Stdio returns a transport backed by os.Stdin and os.Stdout.
func Stdio() Transport {
	return &stdioTransport{}
}

type stdioTransport struct{}

// Read reads from stdin.
func (s *stdioTransport) Read(p []byte) (int, error) { return os.Stdin.Read(p) }

// Write writes to stdout.
func (s *stdioTransport) Write(p []byte) (int, error) { return os.Stdout.Write(p) }

// Close closes stdin to unblock pending reads.
func (s *stdioTransport) Close() error { return os.Stdin.Close() }
