package appupdate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxManagedListBytes = 4096

// The SDK removes old managed entries absent from the new archive. Never let a
// damaged managed list turn that cleanup into removal of user-owned data.
func validateManagedList(raw []byte) error {
	if len(raw) > maxManagedListBytes {
		return fmt.Errorf("程序文件清单过大，请使用便携包手动更新")
	}
	for _, name := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		switch name {
		case "", "NGAReader.exe", "LICENSE", "LICENSES", "README.zh-CN.txt", "update-public.txt":
		default:
			return fmt.Errorf("程序文件清单含有未知路径，请使用便携包手动更新")
		}
	}
	return nil
}

func ValidateWindowsManagedFiles(directory string) error {
	f, err := os.Open(filepath.Join(directory, ".mygo-files"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxManagedListBytes+1))
	if err != nil {
		return err
	}
	return validateManagedList(raw)
}
