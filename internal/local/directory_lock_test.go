package local

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestDataDirectoryLockExcludesOtherOwners(t *testing.T) {
	dir := t.TempDir()
	owner, err := LockDataDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { owner.Close() })
	if other, err := LockDataDirectory(dir); !errors.Is(err, ErrDataDirectoryInUse) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("second writer allowed: %v", err)
	}
	different, err := LockDataDirectory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	different.Close()
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := LockDataDirectory(dir)
	if err != nil {
		t.Fatal("lock remained after close:", err)
	}
	reopened.Close()
}

func TestDataDirectoryLockSurvivesProcessBoundaryAndReleasesAfterCrash(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, exe, "-test.run=^TestDataDirectoryLockChild$")
	child.Env = append(os.Environ(), "NGA_READER_LOCK_TEST_DIR="+dir)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("child did not lock: %q %v", line, err)
	}
	if lock, err := LockDataDirectory(dir); !errors.Is(err, ErrDataDirectoryInUse) {
		if lock != nil {
			lock.Close()
		}
		t.Fatalf("cross-process lock failed: %v", err)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	lock, err := LockDataDirectory(dir)
	if err != nil {
		t.Fatal("crash left a stale lock:", err)
	}
	lock.Close()
}

func TestDataDirectoryLockChild(t *testing.T) {
	dir := os.Getenv("NGA_READER_LOCK_TEST_DIR")
	if dir == "" {
		return
	}
	lock, err := LockDataDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	fmt.Println("locked")
	_, _ = os.Stdin.Read(make([]byte, 1))
}
