package appupdate

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

const Repository = "apple-half/nga-reader"
const MaxArchiveBytes int64 = 100 << 20
const MaxManifestBytes int64 = 1 << 20

var Targets = []string{"windows-amd64", "darwin-arm64", "darwin-amd64"}

type Manifest struct {
	Version           string `json:"version"`
	Notes             string `json:"notes,omitempty"`
	Date              string `json:"date"`
	Target            string `json:"target"`
	URL               string `json:"url"`
	Size              int64  `json:"size"`
	SHA256            string `json:"sha256"`
	Signature         string `json:"signature"`
	MetadataSignature string `json:"metadataSignature,omitempty"`
}

func ManifestName(target string) string { return "update-" + target + ".json" }
func TarName(version, target string) string {
	return "nga-reader-" + strings.TrimPrefix(version, "v") + "-" + target + ".tar.gz"
}
func PortableName(version, target string) string {
	labels := map[string]string{"windows-amd64": "Windows-x64", "darwin-arm64": "macOS-AppleSilicon", "darwin-amd64": "macOS-Intel"}
	return "NGAReader-" + strings.TrimPrefix(version, "v") + "-" + labels[target] + "-portable.zip"
}
func AssetURL(version, name string) string {
	return "https://github.com/" + Repository + "/releases/download/v" + strings.TrimPrefix(version, "v") + "/" + url.PathEscape(name)
}
func SafeAssetURL(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	s, ok := strings.CutPrefix(u.Path, "/"+Repository+"/releases/download/")
	if !ok {
		return false
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[1] == "" || strings.ContainsAny(parts[1], `\:`) {
		return false
	}
	_, err = parseVersion(parts[0])
	return err == nil && strings.HasPrefix(parts[0], "v")
}
func PublicKey(text string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("更新公钥无效")
	}
	return ed25519.PublicKey(b), nil
}
func PrivateKey(text string) (ed25519.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("更新私钥无效")
	}
	derived := ed25519.NewKeyFromSeed(b[:32])
	if !bytes.Equal(derived, b) {
		return nil, errors.New("更新私钥不匹配")
	}
	return ed25519.PrivateKey(b), nil
}
func signDigest(key ed25519.PrivateKey, digest []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, digest))
}
func metadataBytes(m Manifest) ([]byte, error) { m.MetadataSignature = ""; return json.Marshal(m) }
func SignManifest(m *Manifest, key ed25519.PrivateKey) error {
	if len(key) != ed25519.PrivateKeySize {
		return errors.New("更新私钥无效")
	}
	raw, err := metadataBytes(*m)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	m.MetadataSignature = signDigest(key, sum[:])
	return nil
}
func DecodeManifest(raw []byte, pub ed25519.PublicKey, target string) (Manifest, error) {
	fail := func() (Manifest, error) { return Manifest{}, errors.New("更新说明或签名校验失败") }
	if int64(len(raw)) > MaxManifestBytes || len(pub) != ed25519.PublicKeySize {
		return fail()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var m Manifest
	if d.Decode(&m) != nil {
		return fail()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fail()
	}
	if _, err := parseVersion(m.Version); err != nil || strings.HasPrefix(m.Version, "v") {
		return fail()
	}
	validTarget := false
	for _, t := range Targets {
		if target == t {
			validTarget = true
		}
	}
	if !validTarget || m.Target != target || m.URL != AssetURL(m.Version, TarName(m.Version, target)) || m.Size <= 0 || m.Size > MaxArchiveBytes {
		return fail()
	}
	if _, err := time.Parse(time.RFC3339, m.Date); err != nil {
		return fail()
	}
	if digest, err := hex.DecodeString(m.SHA256); err != nil || len(digest) != sha256.Size || strings.ToLower(m.SHA256) != m.SHA256 {
		return fail()
	}
	if sig, err := base64.StdEncoding.DecodeString(m.Signature); err != nil || len(sig) != ed25519.SignatureSize {
		return fail()
	}
	sig, err := base64.StdEncoding.DecodeString(m.MetadataSignature)
	if err != nil {
		return fail()
	}
	payload, _ := metadataBytes(m)
	sum := sha256.Sum256(payload)
	if !ed25519.Verify(pub, sum[:], sig) {
		return fail()
	}
	return m, nil
}

func VerifyArchiveBytes(data []byte, m Manifest, pub ed25519.PublicKey) error {
	sum := sha256.Sum256(data)
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || int64(len(data)) != m.Size || hex.EncodeToString(sum[:]) != m.SHA256 || !ed25519.Verify(pub, sum[:], sig) {
		return fmt.Errorf("更新包大小、摘要或签名不一致")
	}
	return nil
}
