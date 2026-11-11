package nga

import (
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type cookieKey struct{ domain, path, name string }
type cookieStore struct {
	jar    http.CookieJar
	mu     sync.Mutex
	values map[cookieKey]http.Cookie
}

func newCookieStore() *cookieStore {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return &cookieStore{jar: jar, values: map[cookieKey]http.Cookie{}}
}
func (s *cookieStore) Cookies(u *url.URL) []*http.Cookie { return s.jar.Cookies(u) }
func (s *cookieStore) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.jar.SetCookies(u, cookies)
	if u.Scheme != "https" || u.Hostname() != "bbs.nga.cn" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, original := range cookies {
		if original == nil || original.Valid() != nil {
			continue
		}
		c := *original
		domain := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if domain == "" {
			domain = u.Hostname()
		}
		if domain != "bbs.nga.cn" && domain != "nga.cn" {
			continue
		}
		if c.Path == "" || !strings.HasPrefix(c.Path, "/") {
			c.Path = "/"
			if n := strings.LastIndex(u.Path, "/"); n > 0 {
				c.Path = u.Path[:n]
			}
		}
		key := cookieKey{domain, c.Path, c.Name}
		if c.MaxAge < 0 || (!c.Expires.IsZero() && !c.Expires.After(now)) {
			delete(s.values, key)
			continue
		}
		if c.MaxAge > 0 {
			c.Expires = now.Add(time.Duration(c.MaxAge) * time.Second)
			c.MaxAge = 0
		}
		s.values[key] = c
	}
}
func (s *cookieStore) snapshot() []*http.Cookie {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	out := make([]*http.Cookie, 0, len(s.values))
	for key, value := range s.values {
		if !value.Expires.IsZero() && !value.Expires.After(now) {
			delete(s.values, key)
			continue
		}
		copy := value
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Name < out[j].Name
	})
	return out
}
