package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
)

type tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// readOnly marks every tool here. MCP clients use it to decide what needs
// confirmation, and nothing in this server writes.
var readOnly = map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true}

func obj(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
func flag(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

const idHelp = "Node id. A function is \"<file>::<lowercased name>\", e.g. " +
	"\"packages/tass/core/user.cfc::checkpermslist\". A file is its path relative to the map root. " +
	"Use search_symbols to find one rather than guessing."

func (s *Server) tools() []tool {
	list := []tool{
		{
			Name:  "search_symbols",
			Title: "Search symbols",
			Description: "Find functions and files by name, path or component. Start here: every other " +
				"tool takes a node id, and this is how you get one. A blank query with filters set " +
				"lists by those filters alone (for example every unreachable function under a path).",
			InputSchema: obj(map[string]any{
				"query":  str("Text to match against name, file and component. Substring matching, so \"perms\" finds checkPermsList."),
				"kind":   map[string]any{"type": "string", "enum": []string{"function", "file", "package", "external"}, "description": "Restrict to one kind of node."},
				"file":   str("Restrict to nodes whose path starts with this prefix."),
				"island": num("Restrict to one connected component of the graph."),
				"only_unreachable": flag("Only nodes no entry point reaches. Deletion candidates, " +
					"not proven dead code — see get_stats for how many calls went unresolved."),
				"sort_by_fan_in": flag("Order by how many things depend on the node, most first, rather than by name."),
				"limit":          num("Maximum results (default 50)."),
			}),
			Annotations: readOnly,
		},
		{
			Name:        "get_symbol",
			Title:       "Get one symbol",
			Description: "Full detail for one node: file, line, access, island, and how many things depend on it.",
			InputSchema: obj(map[string]any{"id": str(idHelp)}, "id"),
			Annotations: readOnly,
		},
		{
			Name:  "get_callers",
			Title: "Get callers",
			Description: "What depends on this node, and how. An empty result on a function that " +
				"clearly runs usually means the call was made through a receiver the resolver could " +
				"not type, not that nothing calls it.",
			InputSchema: obj(map[string]any{
				"id":    str(idHelp),
				"limit": num("Maximum results (default 200)."),
			}, "id"),
			Annotations: readOnly,
		},
		{
			Name:        "get_callees",
			Title:       "Get callees",
			Description: "What this node depends on: what it calls, instantiates, extends or includes.",
			InputSchema: obj(map[string]any{
				"id":    str(idHelp),
				"limit": num("Maximum results (default 200)."),
			}, "id"),
			Annotations: readOnly,
		},
		{
			Name:  "find_path",
			Title: "Find a dependency path",
			Description: "The shortest chain of dependencies from one node to another, or nothing if " +
				"there is none within the depth limit. Use it to answer \"how does this page reach that " +
				"component\" without reading the files in between.",
			InputSchema: obj(map[string]any{
				"from":      str("Starting node id."),
				"to":        str("Target node id."),
				"max_depth": num("How many hops to search (default 12)."),
			}, "from", "to"),
			Annotations: readOnly,
		},
		{
			Name:  "list_islands",
			Title: "List islands",
			Description: "The disconnected pieces of the graph, largest first. The biggest is the " +
				"application; the rest are detached subsystems — either genuinely dead, or live code " +
				"reached through a call the resolver could not follow.",
			InputSchema: obj(map[string]any{
				"detached_only": flag("Only islands no entry point reaches."),
				"limit":         num("Maximum results (default 50)."),
			}),
			Annotations: readOnly,
		},
		{
			Name:  "list_orphans",
			Title: "List unreferenced symbols",
			Description: "Nodes nothing depends on and no entry point reaches. Deletion candidates. " +
				"Always read get_stats alongside: an unresolved call is an edge the map does not have, " +
				"so a high unresolved count makes this list longer than the truth.",
			InputSchema: obj(map[string]any{
				"kind":  map[string]any{"type": "string", "enum": []string{"function", "file"}, "description": "Restrict to one kind."},
				"limit": num("Maximum results (default 50)."),
			}),
			Annotations: readOnly,
		},
		{
			Name:  "get_stats",
			Title: "Map statistics",
			Description: "Counts for the whole map, including how many call sites resolved to a " +
				"definition. That ratio is the map's confidence in itself: at 50%, half the call graph " +
				"is missing and an empty caller list proves much less than it looks like it does.",
			InputSchema: obj(map[string]any{}),
			Annotations: readOnly,
		},
	}

	if s.Explain != nil {
		list = append(list, tool{
			Name:  "explain_call",
			Title: "Explain a call site",
			Description: "Trace, step by step, how a call site's receiver was typed and why the method " +
				"check passed or failed. This is the tool for \"why is this reported unresolved\" and for " +
				"\"where did that component path come from\" — it names the exact componentResolver that " +
				"fired. It re-parses the file, so it reflects what is on disk now, not what the map holds.",
			InputSchema: obj(map[string]any{
				"file":  str("Path to the CFML file, absolute or relative to the working directory."),
				"line":  num("1-based line number of the call site."),
				"match": str("Optional substring to pick one call on that line."),
			}, "file", "line"),
			Annotations: readOnly,
		})
	}

	return list
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// callTool runs a tool and wraps the result.
//
// A tool that fails returns isError on a successful JSON-RPC response rather than
// a protocol-level error. That is what MCP specifies, and the reason is practical:
// a protocol error is the transport's problem and many clients drop the
// conversation over it, where a tool error is information the model should see and
// can act on.
func (s *Server) callTool(raw json.RawMessage) (any, *rpcError) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{codeInvalidParams, "bad tools/call params: " + err.Error()}
	}

	payload, err := s.run(p.Name, p.Arguments)
	if err != nil {
		return map[string]any{
			"content": []any{map[string]any{"type": "text", "text": err.Error()}},
			"isError": true,
		}, nil
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, &rpcError{codeInternal, "encoding result: " + err.Error()}
	}

	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(encoded)}},
		"structuredContent": payload,
	}, nil
}

type toolArgs struct {
	Query           string `json:"query"`
	Kind            string `json:"kind"`
	File            string `json:"file"`
	Island          *int   `json:"island"`
	OnlyUnreachable bool   `json:"only_unreachable"`
	SortByFanIn     bool   `json:"sort_by_fan_in"`
	Limit           int    `json:"limit"`
	ID              string `json:"id"`
	From            string `json:"from"`
	To              string `json:"to"`
	MaxDepth        int    `json:"max_depth"`
	DetachedOnly    bool   `json:"detached_only"`
	Line            int    `json:"line"`
	Match           string `json:"match"`
}

func (s *Server) run(name string, raw json.RawMessage) (any, error) {
	var a toolArgs

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("bad arguments for %s: %w", name, err)
		}
	}

	switch name {
	case "search_symbols":
		found, err := s.Store.SearchSymbols(&store.SearchOptions{
			Query: a.Query, Kind: a.Kind, File: a.File, Island: a.Island,
			OnlyDead: a.OnlyUnreachable, SortByFan: a.SortByFanIn, Limit: a.Limit,
		})

		return wrap("symbols", found), err

	case "get_symbol":
		if a.ID == "" {
			return nil, errMissing("id")
		}

		return s.Store.Node(a.ID)

	case "get_callers":
		if a.ID == "" {
			return nil, errMissing("id")
		}

		found, err := s.Store.Callers(a.ID, a.Limit)

		return wrap("callers", found), err

	case "get_callees":
		if a.ID == "" {
			return nil, errMissing("id")
		}

		found, err := s.Store.Callees(a.ID, a.Limit)

		return wrap("callees", found), err

	case "find_path":
		if a.From == "" || a.To == "" {
			return nil, errMissing("from and to")
		}

		path, err := s.Store.Path(a.From, a.To, a.MaxDepth)
		if err != nil {
			return nil, err
		}

		if path == nil {
			return map[string]any{
				"found": false,
				"note": "No dependency path within the depth limit. That can mean there is none, " +
					"or that a call on the way resolved to nothing — check get_stats.",
			}, nil
		}

		return map[string]any{"found": true, "hops": len(path) - 1, "path": path}, nil

	case "list_islands":
		found, err := s.Store.Islands(a.DetachedOnly, a.Limit)

		return wrap("islands", found), err

	case "list_orphans":
		found, err := s.Store.Orphans(a.Kind, a.Limit)

		return wrap("orphans", found), err

	case "get_stats":
		return s.Store.Stats()

	case "explain_call":
		if s.Explain == nil {
			return nil, errors.New("explain_call is not available: this server was started without a workspace resolver")
		}

		if a.File == "" || a.Line <= 0 {
			return nil, errMissing("file and a 1-based line")
		}

		text, err := s.Explain(a.File, a.Line, a.Match)
		if err != nil {
			return nil, err
		}

		return map[string]any{"explanation": text}, nil

	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

// wrap gives a list result a named field and a count. A bare JSON array tells a
// reader nothing about whether it was truncated; "count" next to the list does.
func wrap[T any](field string, items []T) map[string]any {
	if items == nil {
		items = []T{}
	}

	return map[string]any{field: items, "count": len(items)}
}

func errMissing(what string) error {
	return fmt.Errorf("missing required argument: %s", strings.TrimSpace(what))
}
