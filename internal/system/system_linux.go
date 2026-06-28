//go:build linux

package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func totalRAM() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("opening /proc/meminfo: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected MemTotal line: %s", line)
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing MemTotal: %w", err)
		}
		// MemTotal is in kB (1024 bytes).
		return kb * 1024, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("reading /proc/meminfo: %w", err)
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}
