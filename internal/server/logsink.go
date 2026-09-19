package server

import (
	"context"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/protocol"
)

// logSink forwards the server's own log records to the client as
// window/logMessage notifications.
//
// Without it every line the server logs reaches the editor as server stderr,
// which vscode-languageclient reports at error level whatever it says — so the
// version banner, "indexing complete" and a genuine failure were indistinguish-
// able in the Output panel, all three prefixed [error]. window/logMessage
// carries the severity, so the client can render each one honestly.
type logSink struct {
	s *Server
}

// Log sends one record. Errors are dropped: a log line is not worth failing a
// request over, and the record is on stderr regardless.
func (l logSink) Log(level int, msg string) {
	// context.Background rather than a request's: a log line can be written from
	// any goroutine, including one that outlives the call that started it, and a
	// cancelled context would silently drop exactly the records written while
	// something was going wrong.
	l.s.notify(context.Background(), protocol.MethodWindowLogMessage, &protocol.LogMessageParams{
		Type:    protocol.MessageType(level),
		Message: msg,
	})
}

// attachLogSink starts forwarding this server's logs to its client, if the
// logger supports it.
//
// Called once the connection exists — before that there is nowhere to send
// them, which is why stderr stays in place as well as this rather than being
// replaced by it.
func (s *Server) attachLogSink() {
	if teeable, ok := s.log.(cflog.Teeable); ok {
		teeable.Attach(logSink{s: s})
	}
}

// detachLogSink stops forwarding, so a notification is never written to a
// connection that has gone away.
func (s *Server) detachLogSink() {
	if teeable, ok := s.log.(cflog.Teeable); ok {
		teeable.Attach(nil)
	}
}
