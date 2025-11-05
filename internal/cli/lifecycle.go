package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ApplyEnvOverrides sets environment variables based on KEY=VALUE pairs.
func ApplyEnvOverrides(overrides []string) error {
	for _, override := range overrides {
		parts := strings.SplitN(override, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid env override %q", override)
		}
		if err := os.Setenv(parts[0], parts[1]); err != nil {
			return fmt.Errorf("set %s: %w", parts[0], err)
		}
	}
	return nil
}

// WritePID writes the provided PID to the given file path.
func WritePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

// ReadPID reads a PID from the specified file.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// RemovePID removes a PID file, ignoring errors.
func RemovePID(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SignalProcess sends the provided signal to the PID.
func SignalProcess(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	if err := process.Signal(sig); err != nil {
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return fmt.Errorf("signal process: %w", err)
	}
	return nil
}

// WaitForExit polls the process state until it exits or the timeout elapses.
func WaitForExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if !processExists(pid) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("process %d did not exit within %s", pid, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil || err == syscall.EPERM
}
