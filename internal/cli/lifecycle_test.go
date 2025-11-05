package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestApplyEnvOverrides(t *testing.T) {
	key := "K0RDENT_TEST_ENV"
	defer os.Unsetenv(key)

	if err := ApplyEnvOverrides([]string{key + "=value"}); err != nil {
		t.Fatalf("ApplyEnvOverrides returned error: %v", err)
	}
	if got := os.Getenv(key); got != "value" {
		t.Fatalf("expected env override to set %s=value, got %q", key, got)
	}
}

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.pid")
	pid := os.Getpid()

	if err := WritePID(path, pid); err != nil {
		t.Fatalf("WritePID returned error: %v", err)
	}

	readPID, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID returned error: %v", err)
	}
	if readPID != pid {
		t.Fatalf("expected pid %d, got %d", pid, readPID)
	}

	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID returned error: %v", err)
	}
}

func TestWaitForExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals not supported on Windows")
	}

	cmd := helperCommand("sleep", "500ms")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	if err := WaitForExit(cmd.Process.Pid, 2*time.Second); err != nil {
		t.Fatalf("WaitForExit returned error: %v", err)
	}
	<-done
}

func TestSignalProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals not supported on Windows")
	}

	cmd := helperCommand("sleep", "5s")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	defer cmd.Process.Kill()

	if err := SignalProcess(cmd.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatalf("SignalProcess returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		t.Fatalf("process did not terminate after signal")
	case err := <-done:
		if err != nil && !wasTerminated(err) {
			t.Fatalf("process exited with unexpected error: %v", err)
		}
	}
}

func wasTerminated(err error) bool {
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.Exited() {
			return true
		}
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.Signaled()
		}
	}
	return false
}

func TestApplyEnvOverridesInvalid(t *testing.T) {
	if err := ApplyEnvOverrides([]string{"INVALID"}); err == nil {
		t.Fatalf("expected error for invalid override")
	}
}

func TestReadPIDInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pid")
	if err := os.WriteFile(path, []byte("not-a-number"), 0o644); err != nil {
		t.Fatalf("failed to write pid file: %v", err)
	}
	if _, err := ReadPID(path); err == nil {
		t.Fatalf("expected error for invalid pid contents")
	}
}

func TestSignalProcessInvalidPID(t *testing.T) {
	if err := SignalProcess(-1, syscall.SIGTERM); err == nil {
		t.Fatalf("expected error for invalid pid")
	}
}

func TestWaitForExitTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals not supported on Windows")
	}
	cmd := helperCommand("sleep", "5s")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	defer cmd.Process.Kill()

	err := WaitForExit(cmd.Process.Pid, 250*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not exit") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func helperCommand(action, value string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		panic("helperCommand should not be called on Windows")
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", action, value)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	idx := 0
	for ; idx < len(args); idx++ {
		if args[idx] == "--" {
			idx++
			break
		}
	}
	if idx >= len(args) {
		os.Exit(1)
	}
	if idx >= len(args) {
		s := "missing helper arguments"
		fmt.Fprintln(os.Stderr, s)
		os.Exit(3)
	}
	action := args[idx]
	var value string
	if idx+1 < len(args) {
		value = args[idx+1]
	}
	switch action {
	case "sleep":
		duration, err := time.ParseDuration(value)
		if err != nil {
			os.Exit(2)
		}
		time.Sleep(duration)
	default:
		fmt.Fprintln(os.Stderr, "unknown helper action")
		os.Exit(3)
	}
	os.Exit(0)
}
