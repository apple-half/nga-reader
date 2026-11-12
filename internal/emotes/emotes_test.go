package emotes

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/png"
	"testing"
)

func TestOfficialCatalogAndBuiltInResourcesAreComplete(t *testing.T) {
	entries := Entries()
	if len(entries) != 265 {
		t.Fatalf("official entries=%d", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.File] {
			t.Fatalf("duplicate resource %s", e.File)
		}
		seen[e.File] = true
		info, data, ok := Asset(e.File)
		if !ok {
			t.Fatal("missing asset", e)
		}
		if len(data) != info.Bytes || len(data) > 256<<10 || fmt.Sprintf("%x", sha256.Sum256(data)) != info.SHA256 {
			t.Fatal("corrupt/unbounded asset", e.File)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != info.Width || cfg.Height != info.Height || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 256 || cfg.Height > 256 || format != "png" && format != "gif" {
			t.Fatal("bad image dimensions/type", e.File, cfg, format, err)
		}
		if info.Frames < 1 || info.Frames > 128 {
			t.Fatal("bad frame count", e.File, info.Frames)
		}
		if round, ok := FromDestination(Destination(e)); !ok || round.File != e.File {
			t.Fatal("internal destination failed", e)
		}
	}
	_, data, _ := Asset("ac15.png")
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "028a210064071c576c3a0840f658201074506c78f85c49db404ffa31a1234015" {
		t.Fatal("official browser-verified cry/laugh sample changed")
	}
}

func TestEmoteLookupSupportsLegacyAndNamedGroups(t *testing.T) {
	for input, file := range map[string]string{"ac:哭笑": "ac15.png", "1": "smile.gif", ":1": "smile.gif", "0:1": "smile.gif", "ng:呲牙笑": "ng_1.png", "pg:吃瓜": "pg08.png"} {
		if e, ok := Lookup(input); !ok || e.File != file || e.Name == "" {
			t.Fatalf("lookup %q: %+v %t", input, e, ok)
		}
	}
	for _, input := range []string{"ac:新加的表情", "ac:哭笑:extra", "../ac15.png", "1evil", "ac:\n哭笑", "999999999999999999999", ""} {
		if _, ok := Lookup(input); ok {
			t.Fatalf("unsafe/unknown tag accepted %q", input)
		}
	}
}

func TestOnlyKnownEmbeddedDestinationsAndOfficialImagePathsResolve(t *testing.T) {
	for _, address := range []string{"ngareader-emote://asset/../ac15.png", "ngareader-emote://asset/%61c15.png", "ngareader-emote://asset/ac15.png?x=1", "ngareader-emote://user@asset/ac15.png", "ngareader-emote://asset/not-known.png", "file:///C:/ac15.png", "https://example.com/ac15.png"} {
		if _, ok := FromDestination(address); ok {
			t.Fatalf("unsafe destination %q", address)
		}
	}
	for _, file := range []string{"../ac15.png", "ac15.png/../../data/settings.json", "/ac15.png", "no.png"} {
		if _, _, ok := Asset(file); ok {
			t.Fatalf("unsafe file %q", file)
		}
	}
	for _, address := range []string{"https://img4.nga.cn/ngabbs/post/smile/ac15.png", "//img4.nga.178.com/ngabbs/post/smile/ac15.png"} {
		if e, ok := FromOfficialURL(address); !ok || e.File != "ac15.png" {
			t.Fatal("official URL rejected", address)
		}
	}
	for _, address := range []string{"https://img4.nga.cn.evil.test/ngabbs/post/smile/ac15.png", "https://img4.nga.cn/attachments/ac15.png", "https://img4.nga.cn/ngabbs/post/smile/../ac15.png", "https://user@img4.nga.cn/ngabbs/post/smile/ac15.png", "https://img4.nga.cn:8443/ngabbs/post/smile/ac15.png", "data:image/png;base64,abc"} {
		if _, ok := FromOfficialURL(address); ok {
			t.Fatalf("non-official path accepted %q", address)
		}
	}
}
