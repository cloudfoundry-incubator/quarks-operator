package testhelper

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// WaitForPort tests and waits on the availability of a TCP host and port
func WaitForPort(host, port string, timeOut time.Duration) error {
	var depChan = make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		go func(address string) {
			defer wg.Done()
			for {
				_, err := net.Dial("tcp", address)
				if err == nil {
					return
				}
				time.Sleep(1 * time.Second)
			}
		}(net.JoinHostPort(host, port))

		wg.Wait()
		close(depChan)
	}()

	select {
	case <-depChan: // ready
		return nil
	case <-time.After(timeOut):
		return fmt.Errorf("%s not ready in %s", net.JoinHostPort(host, port), timeOut)
	}
}
