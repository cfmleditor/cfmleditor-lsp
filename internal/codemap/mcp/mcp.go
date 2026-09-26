// Package mcp serves a code map over the Model Context Protocol, so an assistant
// can ask questions about a codebase's structure instead of grepping for them.
//
// It is read-only by construction: every tool is a query against an already-built
// map, there is no tool that writes a file or runs a command, and the one tool
// that touches source (explain_call) parses it and reports. A server that can only
// answer questions is one that can be pointed at a production checkout without a
// conversation about blast radius.
//
// The transport is JSON-RPC 2.0 over stdio with newline-delimited messages, which
// is what MCP specifies. That is deliberately not the Content-Length framing the
// LSP side of this binary uses; the two protocols are not the same wire format and
// sharing the jsonrpc2 package between them would mean one of them being subtly
// wrong.
package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
)

// protocolVersion is the MCP revision this server implements. A client asking for
// a different one still gets served: the tool surface here is the stable part of
// the protocol and has not changed across revisions, so refusing would break
// clients for no benefit.
const protocolVersion = "2025-06-18"

// Explainer traces how a call site resolved. It is supplied by the caller rather
// than built here, because it needs the whole workspace config and resolver chain
// and this package should not grow a second copy of that wiring.
type Explainer func(file string, line int, match string) (string, error)

// Server answers MCP requests against a store.
type Server struct {
	Store   *store.Store
	Name    string
	Version string

	// Explain is optional. When nil the explain_call tool is not advertised, so a
	// client sees the tools that actually work rather than one that always errors.
	Explain Explainer
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes, as the specification names them.
const (
	codeParse         = -32700
	codeInvalidReq    = -32600
	codeMethodMissing = -32601
	codeInvalidParams = -32602
	codeInternal      = -32603
)

// Serve reads requests until the input ends.
//
// A notification — a request with no id — gets no reply, not even an error. That
// is not a nicety: a client that sent notifications/initialized and received a
// response for it would see an unsolicited message with a null id and, depending
// on the client, drop the connection.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	reader := bufio.NewReaderSize(in, 1<<20)
	writer := bufio.NewWriter(out)
	enc := json.NewEncoder(writer)

	for {
		line, err := reader.ReadBytes('\n')
		if strings.TrimSpace(string(line)) != "" {
			if writeErr := s.handleLine(line, enc, writer); writeErr != nil {
				return writeErr
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return writer.Flush()
			}

			return fmt.Errorf("reading request: %w", err)
		}
	}
}

func (s *Server) handleLine(line []byte, enc *json.Encoder, writer *bufio.Writer) error {
	var req request

	if err := json.Unmarshal(line, &req); err != nil {
		return send(enc, writer, &response{
			JSONRPC: "2.0", ID: json.RawMessage("null"),
			Error: &rpcError{codeParse, "malformed JSON: " + err.Error()},
		})
	}

	if len(req.ID) == 0 {
		// Notification: act on it, answer nothing.
		return nil
	}

	result, rpcErr := s.dispatch(req.Method, req.Params)

	return send(enc, writer, &response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr})
}

func send(enc *json.Encoder, writer *bufio.Writer, r *response) error {
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("writing response: %w", err)
	}

	return writer.Flush()
}

func (s *Server) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions": "A structural map of a CFML codebase: every function and file, " +
				"and the calls, instantiations, inheritance and includes between them. " +
				"Search for a symbol, then follow callers and callees rather than reading whole files. " +
				"Unreachable code is kept and labelled, not dropped — check get_stats for how many " +
				"call sites went unresolved before treating an empty caller list as proof of dead code.",
		}, nil

	case "ping":
		return map[string]any{}, nil

	case "tools/list":
		return map[string]any{"tools": s.tools()}, nil

	case "tools/call":
		return s.callTool(params)

	default:
		return nil, &rpcError{codeMethodMissing, "unknown method: " + method}
	}
}
