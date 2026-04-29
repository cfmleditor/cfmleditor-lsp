package main

import (
	"context"
	"os"

	"github.com/garethedwards/cfmleditor-lsp/server"
	"go.lsp.dev/jsonrpc2"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx := context.Background()
	stream := jsonrpc2.NewStream(newStdio())
	conn := jsonrpc2.NewConn(stream)

	srv := server.NewServer(conn, logger)
	conn.Go(ctx, srv.Handler())

	<-conn.Done()
}

type stdio struct{}

func newStdio() stdio { return stdio{} }

func (s stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (s stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (s stdio) Close() error                { return nil }
