package resolve

import (
	"testing"
	"time"
)

// TestADelegatedMethodIsFound: WireBox mixes a delegate's methods into the
// component holding it, under its prefix and suffix, so a call on the host —
// or a bare call inside it, or in a subclass — reaches the delegate's method.
// A method the host has is never replaced, a method the delegation does not
// name is still reported, and two components delegating to each other do not
// recurse without end. ColdBox's own specs of this were 22 findings.
func TestADelegatedMethodIsFound(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Memory.cfc": `component { function read() {} function write() {} }`,
		"Worker.cfc": `component { function work() {} function vacation() {} }`,
		"Loop.cfc":   `component delegates="Computer" { function spin() {} }`,
		"Computer.cfc": `component delegates="Worker=vacation, Loop" {
	property name="memory" inject delegate delegatePrefix;
	property name="disk" inject="Memory" delegate delegateSuffix="Disk";
	function readDisk() {}
	function boot() {
		memoryRead();
		vacation();
		work();
	}
}`,
		"Laptop.cfc": `component extends="Computer" {
	function f() {
		memoryWrite();
	}
}`,
		"User.cfc": `component {
	function f() {
		var c = new Computer();
		c.memoryRead();
		c.writeDisk();
		c.readDisk();
		c.vacation();
		c.work();
		c.memoryErase();
		c.nothingAnywhere();
		var l = new Laptop();
		l.memoryWrite();
	}
}`,
	})

	done := make(chan map[string]string)

	go func() { done <- reasonsWith(t, &Resolver{}, dir, "User.cfc") }()

	select {
	case got := <-done:
		expectReasons(t, got, map[string]string{
			"c.memoryRead":      "",
			"c.writeDisk":       "",
			"c.readDisk":        "",
			"c.vacation":        "",
			"c.work":            "method 'work' not found in Computer",
			"c.memoryErase":     "method 'memoryErase' not found in Computer",
			"c.nothingAnywhere": "method 'nothingAnywhere' not found in Computer",
			"l.memoryWrite":     "",
		})
	case <-time.After(10 * time.Second):
		t.Fatal("delegates that delegate to each other did not terminate")
	}

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Computer.cfc"), map[string]string{
		"memoryRead": "",
		"vacation":   "",
		"work":       "no qualifier, not in file",
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Laptop.cfc"), map[string]string{
		"memoryWrite": "",
	})
}
