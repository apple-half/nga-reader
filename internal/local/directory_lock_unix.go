//go:build !windows

package local

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func lockDirectoryFile(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return ErrDataDirectoryInUse
	}
	return fmt.Errorf("无法独占锁定数据目录：%w", err)
}
