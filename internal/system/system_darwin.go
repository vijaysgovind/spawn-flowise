//go:build darwin

package system

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func totalRAM() (uint64, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, fmt.Errorf("running sysctl hw.memsize: %w", err)
	}
	v := strings.TrimSpace(string(out))
	bytes, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing hw.memsize: %w", err)
	}
	return bytes, nil
}
