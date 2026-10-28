//go:build !windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func enableVT() {}

func execClaude(id string) {
	path, err := exec.LookPath("claude")
	if err != nil {
		die("claude was not found on your PATH")
	}
	err = syscall.Exec(path, []string{"claude", "--resume", id}, os.Environ())
	die("could not start claude: %v", err)
}

func isClaude(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	out, err := exec.Command("ps", "-ww", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "claude")
}

func replace(tmp, exe string) error { return os.Rename(tmp, exe) }

func removeOld() {}
