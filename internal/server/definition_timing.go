package server

import (
	"time"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
)

// slowDefinition is when a go-to-definition is worth explaining rather than
// just counting. The handler's own slow-request warning fires at 100ms and says
// only that the method was slow; this says which part of it was.
const slowDefinition = 250 * time.Millisecond

// defTimer attributes a definition request's time to the stage that spent it.
//
// handleDefinition tries eight things in order — routes, a resolver argument, a
// component path, a file path, a cfinvoke, a variable, a qualified call, a bare
// name — and several of them read files or scan the whole document. A warning
// that says "textDocument/definition took 3s" narrows that to nothing: every
// stage is a plausible culprit and the only way to tell has been to guess and
// re-measure. Recording a timestamp per stage costs a few nanoseconds on the
// fast path and turns the next slow request into an answer.
type defTimer struct {
	start  time.Time
	last   time.Time
	stages []cflog.Field
}

func newDefTimer() *defTimer {
	now := time.Now()

	return &defTimer{start: now, last: now}
}

// mark records the time since the previous mark against name.
func (d *defTimer) mark(name string) {
	if d == nil {
		return
	}

	now := time.Now()

	if since := now.Sub(d.last); since > time.Millisecond {
		d.stages = append(d.stages, cflog.Duration(name, since))
	}

	d.last = now
}

// report logs the breakdown when the request was slow enough to want one.
//
// Only stages over a millisecond are listed, so a normal slow request names the
// one or two things that actually cost something rather than eight lines of
// zeros.
func (d *defTimer) report(log cflog.Logger, outcome string, uri string, line, char int) {
	if d == nil {
		return
	}

	total := time.Since(d.start)
	if total < slowDefinition {
		return
	}

	fields := make([]any, 0, len(d.stages)*2+8)
	fields = append(fields,
		cflog.Duration("total", total),
		cflog.String("outcome", outcome),
		cflog.String("uri", uri),
		cflog.Int("line", line),
		cflog.Int("char", char))

	for _, f := range d.stages {
		fields = append(fields, f)
	}

	log.Warn("slow definition", fields...)
}
