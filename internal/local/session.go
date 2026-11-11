package local

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var ErrNoSecret = errors.New("no saved session key")

type KeyStore interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}

const keyName = "nga-session-key-v1"

var associatedData = []byte("NGAReader-session-v1")

func SaveSession(dir string, cookies []*http.Cookie, store KeyStore) error {
	key, err := store.Get(keyName)
	created := false
	if errors.Is(err, ErrNoSecret) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return err
		}
		if err = store.Set(keyName, key); err != nil {
			return fmt.Errorf("系统凭据存储不可用，登录仅保留本次运行")
		}
		created = true
	} else if err != nil {
		return fmt.Errorf("系统凭据存储不可用，登录仅保留本次运行")
	}
	if len(key) != 32 {
		return fmt.Errorf("本机凭据密钥无效，请重新登录")
	}
	raw, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	encrypted := append([]byte("NGAR1"), nonce...)
	encrypted = gcm.Seal(encrypted, nonce, raw, associatedData)
	if err = atomicWrite(dir, "session.enc", encrypted); err != nil {
		if created {
			_ = store.Delete(keyName)
		}
		return err
	}
	return nil
}

func LoadSession(dir string, store KeyStore) ([]*http.Cookie, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "session.enc"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("无法读取本机会话")
	}
	if len(raw) > 128<<10 || len(raw) < 5 || string(raw[:5]) != "NGAR1" {
		return nil, fmt.Errorf("本机会话文件无效，请重新登录")
	}
	key, err := store.Get(keyName)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("无法解锁本机会话，请在这台电脑重新登录")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < 5+gcm.NonceSize()+gcm.Overhead() {
		return nil, fmt.Errorf("本机会话文件不完整")
	}
	nonce := raw[5 : 5+gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, raw[5+gcm.NonceSize():], associatedData)
	if err != nil {
		return nil, fmt.Errorf("本机会话校验失败，请重新登录")
	}
	var cookies []*http.Cookie
	if json.Unmarshal(plain, &cookies) != nil {
		return nil, fmt.Errorf("本机会话内容无效")
	}
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		if c != nil && (c.Expires.IsZero() || c.Expires.After(time.Now())) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func ClearSession(dir string, store KeyStore) error {
	err := os.Remove(filepath.Join(dir, "session.enc"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("无法删除本机会话文件：%w", err)
	}
	if err := store.Delete(keyName); err != nil && !errors.Is(err, ErrNoSecret) {
		return fmt.Errorf("会话已清除，但系统凭据密钥未能移除")
	}
	return nil
}
