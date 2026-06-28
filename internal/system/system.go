// Package system provides platform-specific resource discovery for spawn-flowise.
package system

import (
	"fmt"
	"net"
)

// TotalRAM returns the total physical memory in bytes.
func TotalRAM() (uint64, error) {
	return totalRAM()
}

// IsPortAvailable reports whether a TCP port is free on the host.
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
