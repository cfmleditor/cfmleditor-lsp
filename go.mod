module github.com/cfmleditor/cfmleditor-lsp

go 1.23

require (
	github.com/cfmleditor/tree-sitter-cfml v0.26.9
	github.com/tree-sitter/go-tree-sitter v0.25.0
	go.lsp.dev/jsonrpc2 v0.10.0
	go.lsp.dev/protocol v0.12.0
	go.lsp.dev/uri v0.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.3.4 // indirect
	go.lsp.dev/pkg v0.0.0-20210717090340-384b27a52fb2 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.0.0-20220319134239-a9b59b0215f8 // indirect
)

replace github.com/cfmleditor/tree-sitter-cfml => ../tree-sitter-cfml
