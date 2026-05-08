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

	defer func() { _ = conn.Close() }()

	var wg sync.WaitGroup
	wg.Add(2)

	// stdin → socket
	go func() {
		defer wg.Done()
		_, _ = io.Copy(conn, os.Stdin)
		// Editor closed stdin; signal the socket we're done writing
		if uc, ok := conn.(*net.UnixConn); ok {
			_ = uc.CloseWrite()
		}
	}()

	// socket → stdout
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, conn)
	}()

	wg.Wait()
	return nil
}
