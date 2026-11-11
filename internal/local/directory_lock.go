package local

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrDataDirectoryInUse = errors.New("这个数据目录已由另一个 NGA Reader 使用，请返回已打开的窗口")

type DirectoryLock struct{ file *os.File }

// The lock file stays in place. Ownership belongs to the open OS handle, so a
// crash releases it and cannot leave a stale marker that needs manual removal.
func LockDataDirectory(dir string) (*DirectoryLock, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("无法创建数据目录：%w", err)
	}
	file, err := os.OpenFile(filepath.Join(dir, ".nga-reader.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("无法锁定数据目录：%w", err)
	}
	if err := lockDirectoryFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &DirectoryLock{file: file}, nil
}

func (l *DirectoryLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	return file.Close()
}
