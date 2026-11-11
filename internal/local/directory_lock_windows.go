//go:build windows

package local

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var lockDataFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func lockDirectoryFile(file *os.File) error {
	var overlapped syscall.Overlapped
	result, _, err := lockDataFileEx.Call(file.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if result != 0 {
		return nil
	}
	if err == syscall.Errno(33) || err == syscall.Errno(32) {
		return ErrDataDirectoryInUse
	}
	return fmt.Errorf("无法独占锁定数据目录：%w", err)
}
