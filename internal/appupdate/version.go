package appupdate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

type version struct {
	numbers [3]uint64
	pre     []string
}

func parseVersion(text string) (version, error) {
	m := versionPattern.FindStringSubmatch(text)
	if m == nil {
		return version{}, fmt.Errorf("无效的版本号")
	}
	var v version
	for i := range v.numbers {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return version{}, fmt.Errorf("版本号超出范围")
		}
		v.numbers[i] = n
	}
	if m[4] != "" {
		v.pre = strings.Split(m[4], ".")
		for _, part := range v.pre {
			if numeric(part) && len(part) > 1 && part[0] == '0' {
				return version{}, fmt.Errorf("无效的预发布版本号")
			}
		}
	}
	return v, nil
}
func numeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
func CompareVersions(a, b string) (int, error) {
	x, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	y, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i, n := range x.numbers {
		if n < y.numbers[i] {
			return -1, nil
		}
		if n > y.numbers[i] {
			return 1, nil
		}
	}
	if len(x.pre) == 0 && len(y.pre) > 0 {
		return 1, nil
	}
	if len(y.pre) == 0 && len(x.pre) > 0 {
		return -1, nil
	}
	for i := 0; i < min(len(x.pre), len(y.pre)); i++ {
		p, q := x.pre[i], y.pre[i]
		if p == q {
			continue
		}
		pn, qn := numeric(p), numeric(q)
		if pn && qn {
			if len(p) < len(q) {
				return -1, nil
			}
			if len(p) > len(q) {
				return 1, nil
			}
		} else if pn {
			return -1, nil
		} else if qn {
			return 1, nil
		}
		return strings.Compare(p, q), nil
	}
	if len(x.pre) < len(y.pre) {
		return -1, nil
	}
	if len(x.pre) > len(y.pre) {
		return 1, nil
	}
	return 0, nil
}
