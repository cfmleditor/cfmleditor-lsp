//go:build !wasm

package daemon

import (
	"io"
	"net"
	"os"
	"sync"
)

// Proxy connects to an existing daemon socket and bridges it to stdio.
// Returns nil on clean shutdown.
func Proxy(sockPath string) error {
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return err
	}

	var once sync.Once
	closeConn := func() { once.Do(func() { _ = conn.Close() }) }
	defer closeConn()

	var wg sync.WaitGroup
	wg.Add(2)

	// stdin → socket
	go func() {
		defer wg.Done()
		_, _ = io.Copy(conn, os.Stdin)
		closeConn()
	}()

	// socket → stdout
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, conn)
		// Daemon closed the socket; close stdin to unblock the other goroutine
		_ = os.Stdin.Close()
	}()

	wg.Wait()
	return nil
}
