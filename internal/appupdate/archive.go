package appupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type zipEntry struct {
	file *zip.File
	name string
	mode int64
}

func archiveName(name, target string) (string, bool, error) {
	if strings.ContainsAny(name, `\:`) || !fs.ValidPath(name) {
		return "", false, fmt.Errorf("更新包路径无效")
	}
	for _, part := range strings.Split(name, "/") {
		switch strings.ToLower(part) {
		case "data", "runtime", "cache", "logs", ".git", "drafts.json", "settings.json", "session.enc", ".nga-reader.lock":
			return "", false, fmt.Errorf("更新包含有私人数据")
		}
	}
	if target == "windows-amd64" {
		s, ok := strings.CutPrefix(name, "NGAReader/")
		if !ok {
			return "", false, fmt.Errorf("Windows包目录无效")
		}
		if s != "NGAReader.exe" && s != "LICENSE" && s != "README.zh-CN.txt" && !strings.HasPrefix(s, "LICENSES/") {
			return "", false, fmt.Errorf("Windows包包含未知程序文件")
		}
		return s, false, nil
	}
	if target != "darwin-arm64" && target != "darwin-amd64" {
		return "", false, fmt.Errorf("更新平台无效")
	}
	if name == "README.zh-CN.txt" {
		return "", true, nil
	}
	if !strings.HasPrefix(name, "NGAReader.app/Contents/") {
		return "", false, fmt.Errorf("Mac包目录无效")
	}
	return name, false, nil
}
func readZIPEntries(reader *zip.ReadCloser, target string) ([]zipEntry, error) {
	if len(reader.File) > 256 {
		return nil, fmt.Errorf("更新包文件过多")
	}
	seen := map[string]bool{}
	var entries []zipEntry
	var total uint64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			if _, _, err := archiveName(strings.TrimSuffix(file.Name, "/"), target); err != nil {
				// Container directories are optional in the ZIP and never extracted.
				if file.Name != "NGAReader/" && file.Name != "NGAReader.app/" && file.Name != "NGAReader.app/Contents/" {
					return nil, err
				}
			}
			continue
		}
		if !file.Mode().IsRegular() {
			return nil, fmt.Errorf("更新包包含链接或特殊文件")
		}
		name, skip, err := archiveName(file.Name, target)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			return nil, fmt.Errorf("更新包路径重复")
		}
		seen[key] = true
		if file.UncompressedSize64 > uint64(MaxArchiveBytes) || total > uint64(2*MaxArchiveBytes)-file.UncompressedSize64 {
			return nil, fmt.Errorf("更新包解压内容过大")
		}
		total += file.UncompressedSize64
		mode := int64(0644)
		if strings.HasSuffix(name, "/Contents/MacOS/NGAReader") {
			if file.Mode().Perm()&0111 == 0 {
				return nil, fmt.Errorf("Mac程序缺少执行权限")
			}
			mode = 0755
		}
		entries = append(entries, zipEntry{file: file, name: name, mode: mode})
	}
	required := []string{"NGAReader.exe", "LICENSE"}
	if target != "windows-amd64" {
		required = []string{"NGAReader.app/Contents/MacOS/NGAReader", "NGAReader.app/Contents/Info.plist", "NGAReader.app/Contents/Resources/LICENSE", "NGAReader.app/Contents/_CodeSignature/CodeResources"}
	}
	for _, name := range required {
		if !seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("更新包缺少%s", name)
		}
	}
	for _, entry := range entries {
		if entry.name == required[0] {
			r, err := entry.file.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(io.LimitReader(r, 4096))
			r.Close()
			if err != nil {
				return nil, err
			}
			if err = checkBinary(data, target); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries, nil
}
func checkBinary(data []byte, target string) error {
	if target == "windows-amd64" {
		if len(data) < 64 || string(data[:2]) != "MZ" {
			return fmt.Errorf("Windows程序格式异常")
		}
		offset := uint64(binary.LittleEndian.Uint32(data[60:64]))
		if offset+6 > uint64(len(data)) || string(data[offset:offset+4]) != "PE\x00\x00" || binary.LittleEndian.Uint16(data[offset+4:offset+6]) != 0x8664 {
			return fmt.Errorf("Windows程序架构异常")
		}
		return nil
	}
	cpu := uint32(0x0100000c)
	if target == "darwin-amd64" {
		cpu = 0x01000007
	}
	if len(data) < 32 || !bytes.Equal(data[:4], []byte{0xcf, 0xfa, 0xed, 0xfe}) || binary.LittleEndian.Uint32(data[4:8]) != cpu {
		return fmt.Errorf("Mac程序架构异常")
	}
	return nil
}
func ArchiveFromZIP(zipPath, tarPath, target string) (err error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	entries, err := readZIPEntries(reader, target)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(tarPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(tarPath)
		}
	}()
	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		if err = tw.WriteHeader(&tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(entry.file.UncompressedSize64), Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
			return err
		}
		r, e := entry.file.Open()
		if e != nil {
			return e
		}
		n, e := io.Copy(tw, r)
		r.Close()
		if e != nil {
			return e
		}
		if n != int64(entry.file.UncompressedSize64) {
			return fmt.Errorf("ZIP内容长度异常")
		}
	}
	if err = tw.Close(); err != nil {
		return err
	}
	if err = gz.Close(); err != nil {
		return err
	}
	return f.Close()
}
func SignDirectory(dir, version, notes, date string, key ed25519.PrivateKey) error {
	if _, err := parseVersion(version); err != nil {
		return err
	}
	if len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("签名私钥无效")
	}
	for _, target := range Targets {
		archive := filepath.Join(dir, TarName(version, target))
		if err := ArchiveFromZIP(filepath.Join(dir, PortableName(version, target)), archive, target); err != nil {
			return err
		}
		data, err := os.ReadFile(archive)
		if err != nil {
			return err
		}
		if int64(len(data)) > MaxArchiveBytes {
			return fmt.Errorf("签名归档过大")
		}
		sum := sha256.Sum256(data)
		m := Manifest{Version: version, Notes: notes, Date: date, Target: target, URL: AssetURL(version, TarName(version, target)), Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Signature: signDigest(key, sum[:])}
		if err := SignManifest(&m, key); err != nil {
			return err
		}
		raw, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		if _, err := DecodeManifest(raw, key.Public().(ed25519.PublicKey), target); err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(dir, ManifestName(target)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		_, err = f.Write(append(raw, '\n'))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return VerifyDirectory(dir, version, key.Public().(ed25519.PublicKey))
}
func VerifyDirectory(dir, version string, pub ed25519.PublicKey) error {
	for _, target := range Targets {
		raw, err := os.ReadFile(filepath.Join(dir, ManifestName(target)))
		if err != nil {
			return err
		}
		m, err := DecodeManifest(raw, pub, target)
		if err != nil {
			return err
		}
		if m.Version != version {
			return fmt.Errorf("更新版本与标签不一致")
		}
		info, err := os.Stat(filepath.Join(dir, TarName(version, target)))
		if err != nil {
			return err
		}
		if info.Size() != m.Size {
			return fmt.Errorf("更新包大小异常")
		}
		data, err := os.ReadFile(filepath.Join(dir, TarName(version, target)))
		if err != nil {
			return err
		}
		if err := VerifyArchiveBytes(data, m, pub); err != nil {
			return err
		}
		z, err := zip.OpenReader(filepath.Join(dir, PortableName(version, target)))
		if err != nil {
			return err
		}
		entries, err := readZIPEntries(z, target)
		if err != nil {
			z.Close()
			return err
		}
		err = compareTarZIP(data, entries)
		z.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func compareTarZIP(data []byte, entries []zipEntry) error {
	expected := map[string]zipEntry{}
	for _, e := range entries {
		expected[e.name] = e
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entry, ok := expected[header.Name]
		if !ok || header.Typeflag != tar.TypeReg || header.Size != int64(entry.file.UncompressedSize64) || header.Mode != entry.mode || !fs.ValidPath(header.Name) || path.Clean(header.Name) != header.Name {
			return fmt.Errorf("签名归档与便携包结构不一致")
		}
		got := sha256.New()
		if _, err := io.Copy(got, tr); err != nil {
			return err
		}
		r, err := entry.file.Open()
		if err != nil {
			return err
		}
		want := sha256.New()
		_, err = io.Copy(want, r)
		r.Close()
		if err != nil {
			return err
		}
		if !bytes.Equal(got.Sum(nil), want.Sum(nil)) {
			return fmt.Errorf("签名归档与便携包内容不一致")
		}
		delete(expected, header.Name)
	}
	if len(expected) != 0 {
		return fmt.Errorf("签名归档缺少便携包文件")
	}
	return nil
}
