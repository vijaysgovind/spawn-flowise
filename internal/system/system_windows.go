//go:build windows

package system

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func totalRAM() (uint64, error) {
	out, err := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory", "/value").Output()
	if err != nil {
		return 0, fmt.Errorf("running wmic: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TotalPhysicalMemory=") {
			v := strings.TrimPrefix(line, "TotalPhysicalMemory=")
			bytes, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parsing TotalPhysicalMemory: %w", err)
			}
			return bytes, nil
		}
	}
	return 0, fmt.Errorf("TotalPhysicalMemory not found in wmic output")
}
