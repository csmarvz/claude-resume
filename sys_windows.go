package main

import (
	"errors"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

func enableVT() {
	h := windows.Handle(os.Stdout.Fd())
	var m uint32
	if windows.GetConsoleMode(h, &m) == nil {
		windows.SetConsoleMode(h, m|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

func execClaude(id string) {
	path, err := exec.LookPath("claude")
	if err != nil {
		die("claude was not found on your PATH")
	}
	cmd := exec.Command(path, "--resume", id)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) {
			os.Exit(e.ExitCode())
		}
		die("could not start claude: %v", err)
	}
	os.Exit(0)
}

func isClaude(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	p.Release()
	return true
}

// replace moves the running program aside first, because Windows does not let a running .exe be overwritten.
func replace(tmp, exe string) error {
	os.Remove(exe + ".old")
	if err := os.Rename(exe, exe+".old"); err != nil {
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Rename(exe+".old", exe)
		return err
	}
	return nil
}

func removeOld() {
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}
