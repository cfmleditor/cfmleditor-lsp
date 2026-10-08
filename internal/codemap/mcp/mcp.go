// Package mcp serves a CFML workspace over the Model Context Protocol, so an
// assistant can ask questions about a codebase instead of grepping for them:
// its structure from a code map, and the checks the CLI runs (unresolved calls,
// references, CFLint).
//
// It writes nothing. The map tools query an already-built map, and the task
// tools parse source and report. The one tool that runs another program, lint,
// is offered only when the server is started with it switched on. A server that
// can only answer questions is one that can be pointed at a production checkout
// without a conversation about blast radius.
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

// Unresolver reports the calls under paths that do not resolve. limit caps the
// list it returns; the result says how many there were in all.
type Unresolver func(paths []string, globalDefs bool, limit int) (any, error)

// RefFinder finds the references to a component (a dot-path) or a function (a
// bare name) under paths.
type RefFinder func(target string, paths []string, limit int) (any, error)

// Linter runs CFLint over paths.
type Linter func(paths []string, limit int) (any, error)

// Server answers MCP requests. Like Explain, every hook is supplied by the
// caller, since each needs the workspace config and resolver chain.
type Server struct {
	// Store is the code map. When nil the map tools are not advertised: the
	// task tools need no map, and a server started without one still has them.
	Store   *store.Store
	Name    string
	Version string

	// Each hook is optional. When one is nil its tool is not advertised, so a
	// client sees the tools that actually work rather than one that always errors.
	Explain    Explainer
	Unresolved Unresolver
	FindRefs   RefFinder
	Lint       Linter
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

func (s *Server) instructions() string {
	text := "Checks over a CFML workspace: find_unresolved_calls reports calls that do not " +
		"resolve and explain_call says why one did or did not; find_references finds the uses " +
		"of a component or function."

	if s.Lint != nil {
		text += " lint runs CFLint over files or directories."
	}

	if s.Store == nil {
		return text + " No code map is loaded, so the structural tools (search_symbols, " +
			"get_callers and the rest) are not offered: build one with " +
			"`cfmleditor-lsp graph --db <file> <dir>` and start this server with --db <file>."
	}

	return text + " A structural map is loaded too: every function and file, and the calls, " +
		"instantiations, inheritance and includes between them. Search for a symbol, then follow " +
		"callers and callees rather than reading whole files. Unreachable code is kept and " +
		"labelled, not dropped — check get_stats for how many call sites went unresolved before " +
		"treating an empty caller list as proof of dead code."
}

func (s *Server) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    s.instructions(),
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
