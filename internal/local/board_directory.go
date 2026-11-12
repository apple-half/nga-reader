package local

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"ngareader/internal/nga"
)

type boardDirectoryCache struct {
	Version   int       `json:"version"`
	FetchedAt time.Time `json:"fetchedAt"`
	Data      []byte    `json:"data"`
}

func BoardDirectoryFresh(at, now time.Time) bool {
	return !at.IsZero() && !at.After(now) && now.Sub(at) < 24*time.Hour
}

func LoadBoardDirectoryCache(dir string, now time.Time) ([]nga.DirectoryEntry, time.Time, error) {
	file, err := os.Open(filepath.Join(dir, "board-directory.json"))
	if err != nil {
		return nil, time.Time{}, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 2*nga.MaxDirectoryBytes+1))
	if err != nil {
		return nil, time.Time{}, err
	}
	var cache boardDirectoryCache
	if len(raw) > 2*nga.MaxDirectoryBytes || json.Unmarshal(raw, &cache) != nil || cache.Version != 1 || cache.FetchedAt.IsZero() || cache.FetchedAt.After(now.Add(5*time.Minute)) {
		return nil, time.Time{}, fmt.Errorf("公开板块缓存无效")
	}
	entries, err := nga.ParseBoardDirectory(cache.Data)
	return entries, cache.FetchedAt, err
}
func SaveBoardDirectoryCache(dir string, raw []byte, at time.Time) error {
	if _, err := nga.ParseBoardDirectory(raw); err != nil {
		return err
	}
	if at.IsZero() {
		return fmt.Errorf("公开板块缓存时间无效")
	}
	data, err := json.Marshal(boardDirectoryCache{Version: 1, FetchedAt: at, Data: raw})
	if err != nil {
		return err
	}
	return atomicWrite(dir, "board-directory.json", data)
}
