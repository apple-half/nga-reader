package nga

import "testing"

func TestImageURLsUseCurrentOfficialCDN(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"./mon_202611/10/-7Q31-fcoxK23T3cSk0-zk.jpg", "https://img.nga.cn/attachments/mon_202611/10/-7Q31-fcoxK23T3cSk0-zk.jpg"},
		{"https://img.nga.178.com/attachments/mon_202611/test.jpg", "https://img.nga.cn/attachments/mon_202611/test.jpg"},
		{"http://img4.nga.178.com/attachments/test.jpg", "https://img4.nga.cn/attachments/test.jpg"},
		{"//img8.nga.cn/attachments/test.jpg", "https://img8.nga.cn/attachments/test.jpg"},
	} {
		got, err := ImageURL(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("image %q resolves to %q, want %q: %v", tc.input, got, tc.want, err)
		}
	}
}

func TestImageURLRejectsNonOfficialOrigins(t *testing.T) {
	for _, address := range []string{"https://img4.nga.cn.evil.test/a.jpg", "https://img.nga.cn:8443/a.jpg", "https://user:pass@img.nga.cn/a.jpg", "file:///a.jpg", "https://127.0.0.1/a.jpg"} {
		if _, err := ImageURL(address); err == nil {
			t.Errorf("accepted image origin %q", address)
		}
	}
}
