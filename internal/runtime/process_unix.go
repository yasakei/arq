//go:build linux

package runtime

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func killMatching(pattern string) error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		cmd, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		if strings.Contains(strings.ReplaceAll(string(cmd), "\x00", " "), pattern) && pid != os.Getpid() {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
	return nil
}
