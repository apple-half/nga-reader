package appupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}
func testManifest(t *testing.T, priv ed25519.PrivateKey, target string) Manifest {
	t.Helper()
	sum := sha256.Sum256([]byte("synthetic archive"))
	m := Manifest{Version: "0.1.8", Target: target, Notes: "新版本说明", Date: "2026-11-11T08:00:00Z", URL: AssetURL("0.1.8", TarName("0.1.8", target)), Size: 17, SHA256: hex.EncodeToString(sum[:]), Signature: signDigest(priv, sum[:])}
	if err := SignManifest(&m, priv); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestUpdateSemanticVersionOrdering(t *testing.T) {
	for _, pair := range [][2]string{{"0.1.9", "0.1.10"}, {"1.0.0-beta.2", "1.0.0-beta.10"}, {"1.0.0-beta.10", "1.0.0"}, {"0.9.9", "1.0.0"}} {
		n, err := CompareVersions(pair[0], pair[1])
		if err != nil || n >= 0 {
			t.Fatalf("%v = %d %v", pair, n, err)
		}
	}
	for _, value := range []string{"1.2", "01.2.3", "1.2.3/../../x", "1.2.3-beta..1", "1.2.3-01"} {
		if _, err := CompareVersions(value, "1.0.0"); err == nil {
			t.Fatalf("invalid version accepted: %s", value)
		}
	}
}

func TestUpdateMetadataSignatureBindsVersionPlatformAndURL(t *testing.T) {
	pub, priv := testKeys(t)
	m := testManifest(t, priv, "windows-amd64")
	raw, _ := json.Marshal(m)
	if _, err := DecodeManifest(raw, pub, "windows-amd64"); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Manifest){func(m *Manifest) { m.Version = "9.9.9" }, func(m *Manifest) { m.Target = "darwin-arm64" }, func(m *Manifest) { m.URL = "https://example.invalid/update.tar.gz" }, func(m *Manifest) { m.Notes = "被更改" }, func(m *Manifest) { m.Size++ }} {
		bad := m
		change(&bad)
		raw, _ := json.Marshal(bad)
		if _, err := DecodeManifest(raw, pub, "windows-amd64"); err == nil {
			t.Fatal("tampered metadata accepted")
		}
	}
	bad := append(raw, []byte("garbage")...)
	if _, err := DecodeManifest(bad, pub, "windows-amd64"); err == nil {
		t.Fatal("trailing bytes accepted")
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUpdateCatalogSkipsIncompleteAndWrongPlatformAndRejectsTampering(t *testing.T) {
	pub, priv := testKeys(t)
	m := testManifest(t, priv, "windows-amd64")
	raw, _ := json.Marshal(m)
	sum := sha256.Sum256(raw)
	releases := []Release{{TagName: "v9.0.0", Draft: true}, {TagName: "v0.1.10", Assets: []Asset{{Name: "Source code.zip"}}}, {TagName: "v0.1.9", Assets: []Asset{{Name: ManifestName("darwin-arm64")}}}, {TagName: "v0.1.8", Prerelease: true, Assets: []Asset{{Name: ManifestName(m.Target), URL: AssetURL(m.Version, ManifestName(m.Target)), Size: int64(len(raw)), Digest: "sha256:" + hex.EncodeToString(sum[:])}, {Name: TarName(m.Version, m.Target), URL: m.URL, Size: m.Size, Digest: "sha256:" + m.SHA256}}}}
	requests := 0
	tamper := false
	c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials leaked to GitHub")
		}
		body := raw
		if r.URL.Host == "api.github.com" {
			body, _ = json.Marshal(releases)
		} else if tamper {
			body = bytes.ReplaceAll(raw, []byte("新版本说明"), []byte("改过的说明"))
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: ioCloser(body)}, nil
	})}, pub)
	offer, err := c.latestAPI(context.Background(), "0.1.7", "windows-amd64")
	if err != nil || offer == nil || offer.Manifest.Version != "0.1.8" || requests != 2 {
		t.Fatalf("offer=%+v requests=%d err=%v", offer, requests, err)
	}
	tamper = true
	if _, err := c.latestAPI(context.Background(), "0.1.7", "windows-amd64"); err == nil {
		t.Fatal("tampered download accepted")
	}
	tamper = false
	if offer, err := c.latestAPI(context.Background(), "0.1.8", "windows-amd64"); err != nil || offer != nil {
		t.Fatalf("downgrade/same version offered: %+v %v", offer, err)
	}
}

func TestUpdateCatalogRejectsForeignAssetAndHTTPFailure(t *testing.T) {
	pub, _ := testKeys(t)
	for _, status := range []int{403, 404, 500} {
		c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: ioCloser([]byte("error"))}, nil
		})}, pub)
		if _, err := c.latestAPI(context.Background(), "0.1.7", "windows-amd64"); err == nil {
			t.Fatal("HTTP error became success")
		}
	}
	if SafeAssetURL("https://github.com/other/repo/releases/download/v0.1.8/a") || SafeAssetURL("http://github.com/apple-half/nga-reader/releases/download/v0.1.8/a") {
		t.Fatal("foreign/unsecured asset URL accepted")
	}
}

func ioCloser(data []byte) io.ReadCloser { return io.NopCloser(bytes.NewReader(data)) }

func makeUpdateZIP(t *testing.T, path, target string, extra map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(file)
	entries := map[string][]byte{}
	if target == "windows-amd64" {
		pe := make([]byte, 160)
		copy(pe, "MZ")
		pe[60] = 128
		copy(pe[128:], "PE\x00\x00\x64\x86")
		entries["NGAReader/NGAReader.exe"] = pe
		entries["NGAReader/LICENSE"] = []byte("MIT License")
		entries["NGAReader/LICENSES/mygo.txt"] = []byte("MIT")
	} else {
		cpu := byte(12)
		if target == "darwin-amd64" {
			cpu = 7
		}
		bin := make([]byte, 64)
		copy(bin, []byte{0xcf, 0xfa, 0xed, 0xfe, cpu, 0, 0, 1})
		entries["NGAReader.app/Contents/MacOS/NGAReader"] = bin
		entries["NGAReader.app/Contents/Info.plist"] = []byte("<plist><dict><key>CFBundleIdentifier</key><string>io.github.apple-half.ngareader</string></dict></plist>")
		entries["NGAReader.app/Contents/Resources/LICENSE"] = []byte("MIT License")
		entries["NGAReader.app/Contents/_CodeSignature/CodeResources"] = []byte("fixture")
	}
	for name, value := range extra {
		entries[name] = []byte(value)
	}
	for name, value := range entries {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0644)
		if strings.HasSuffix(name, "/MacOS/NGAReader") {
			h.SetMode(0755)
		}
		entry, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = entry.Write(value)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateOfflineSigningAndVerificationAcrossPlatforms(t *testing.T) {
	pub, priv := testKeys(t)
	dir := t.TempDir()
	for _, target := range Targets {
		makeUpdateZIP(t, filepath.Join(dir, PortableName("0.1.8", target)), target, nil)
	}
	if err := SignDirectory(dir, "0.1.8", "更新说明", "2026-11-11T08:00:00Z", priv); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDirectory(dir, "0.1.8", pub); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, TarName("0.1.8", "windows-amd64"))
	f, err := os.OpenFile(p, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteAt([]byte("x"), 25)
	f.Close()
	if err := VerifyDirectory(dir, "0.1.8", pub); err == nil {
		t.Fatal("changed archive accepted")
	}
}

func TestUpdateArchiveRejectsTraversalPrivateFilesAndWrongBinary(t *testing.T) {
	for _, extra := range []map[string]string{{"NGAReader/../outside": "x"}, {"NGAReader/data/session.enc": "secret"}, {"NGAReader/drafts.json": "private"}, {"NGAReader/NGAReader.exe": "not PE"}, {"NGAReader/extra.exe": "extra"}} {
		dir := t.TempDir()
		z := filepath.Join(dir, "input.zip")
		makeUpdateZIP(t, z, "windows-amd64", extra)
		if err := ArchiveFromZIP(z, filepath.Join(dir, "update.tar.gz"), "windows-amd64"); err == nil {
			t.Fatalf("bad archive accepted: %v", extra)
		}
	}
}

func TestUpdateArchiveAcceptsSdkManagedListWithoutShippingIt(t *testing.T) {
	dir := t.TempDir()
	z := filepath.Join(dir, "input.zip")
	makeUpdateZIP(t, z, "windows-amd64", map[string]string{"NGAReader/.mygo-files": "LICENSE\nLICENSES\nNGAReader.exe\n"})
	if err := ArchiveFromZIP(z, filepath.Join(dir, "update.tar.gz"), "windows-amd64"); err != nil {
		t.Fatal("official SDK file list rejected:", err)
	}
	f, err := os.Open(filepath.Join(dir, "update.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Name == ".mygo-files" {
			t.Fatal("SDK must regenerate the managed list")
		}
	}
}

func TestUpdateManagedListCannotRemovePrivateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := ValidateWindowsManagedFiles(dir); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"NGAReader.exe\nLICENSE\nLICENSES\n", "data\n", "drafts.json\n", "..\n", "LICENSES/../../data\n", strings.Repeat("x", 4097)} {
		if err := os.WriteFile(filepath.Join(dir, ".mygo-files"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		err := ValidateWindowsManagedFiles(dir)
		if strings.HasPrefix(content, "NGAReader.exe") {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("unsafe existing managed list accepted: %.60q", content)
		}
		z := filepath.Join(t.TempDir(), "input.zip")
		makeUpdateZIP(t, z, "windows-amd64", map[string]string{"NGAReader/.mygo-files": content})
		if err := ArchiveFromZIP(z, z+".tar.gz", "windows-amd64"); err == nil {
			t.Fatal("unsafe ZIP managed list accepted")
		}
	}
}

func TestUpdateCatalogSortsVersionsAcrossInvalidTags(t *testing.T) {
	pub, priv := testKeys(t)
	var releases []Release
	bodies := map[string][]byte{}
	for _, version := range []string{"0.1.8", "invalid", "0.1.9"} {
		if version == "invalid" {
			releases = append(releases, Release{TagName: version})
			continue
		}
		m := testManifest(t, priv, "windows-amd64")
		m.Version = version
		m.URL = AssetURL(version, TarName(version, m.Target))
		if err := SignManifest(&m, priv); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(m)
		sum := sha256.Sum256(raw)
		url := AssetURL(version, ManifestName(m.Target))
		bodies[url] = raw
		releases = append(releases, Release{TagName: "v" + version, Assets: []Asset{{Name: ManifestName(m.Target), URL: url, Size: int64(len(raw)), Digest: "sha256:" + hex.EncodeToString(sum[:])}, {Name: TarName(version, m.Target), URL: m.URL, Size: m.Size, Digest: "sha256:" + m.SHA256}}})
	}
	c := NewClient(&http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		raw := bodies[r.URL.String()]
		if r.URL.Host == "api.github.com" {
			raw, _ = json.Marshal(releases)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: ioCloser(raw)}, nil
	})}, pub)
	offer, err := c.latestAPI(context.Background(), "0.1.7", "windows-amd64")
	if err != nil || offer == nil || offer.Manifest.Version != "0.1.9" {
		t.Fatalf("newest version missed: %+v %v", offer, err)
	}
}
