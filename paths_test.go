package main

import (
	"path/filepath"
	"testing"
)

func TestMacDataDirectoryStaysOutsideAppBundle(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "AppTranslocation", "NGAReader.app", "Contents", "MacOS", "NGAReader")
	config := filepath.Join(t.TempDir(), "Library", "Application Support")
	got, err := defaultDataDirectory(exe, "darwin", config)
	want := filepath.Join(config, "NGAReader")
	if err != nil || got != want {
		t.Fatalf("Mac app must not write into translocated bundle: got %q, err %v; want %q", got, err, want)
	}
}

func TestMacDataDirectoryRequiresUserConfigRoot(t *testing.T) {
	if _, err := defaultDataDirectory("NGAReader.app/Contents/MacOS/NGAReader", "darwin", ""); err == nil {
		t.Fatal("missing Mac configuration directory silently used the app bundle")
	}
}

func TestOtherPlatformsKeepPortableDataDirectory(t *testing.T) {
	for _, platform := range []string{"windows", "linux"} {
		exe := filepath.Join(t.TempDir(), "NGAReader")
		got, err := defaultDataDirectory(exe, platform, "unused")
		if err != nil || got != filepath.Join(filepath.Dir(exe), "data") {
			t.Fatalf("%s changed portable settings: %q %v", platform, got, err)
		}
	}
}
