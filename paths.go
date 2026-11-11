package main

import (
	"errors"
	"path/filepath"
)

// A downloaded macOS app may run from a read-only App Translocation directory.
// Keep its writable settings outside the bundle; other platforms stay portable.
func defaultDataDirectory(executable, platform, configRoot string) (string, error) {
	if platform == "darwin" {
		if configRoot == "" {
			return "", errors.New("无法确定 macOS 用户数据目录")
		}
		return filepath.Join(configRoot, "NGAReader"), nil
	}
	return filepath.Join(filepath.Dir(executable), "data"), nil
}
