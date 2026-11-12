package local

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Dropping the preference during decoding, or accepting an unknown image ID,
// would make these round trips lose the user's choice or show a missing icon.
func TestAppIconPreferenceMigratesAndSurvivesRestart(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`{"theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":"office","theme":"浅色","fontSize":17}`, "office"},
		{`{"appIcon":"work","theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":"../outside.png","theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":42,"theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":false,"theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":{},"theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":[],"theme":"浅色","fontSize":17}`, "work"},
		{`{"appIcon":null,"theme":"浅色","fontSize":17}`, "work"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tc.input), 0600); err != nil {
			t.Fatal(err)
		}
		s, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := Save(dir, s); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			AppIcon  string `json:"appIcon"`
			Theme    string `json:"theme"`
			FontSize int    `json:"fontSize"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.AppIcon != tc.want || got.Theme != "浅色" || got.FontSize != 17 {
			t.Fatalf("preference lost or unrelated settings changed: %+v, want icon %q", got, tc.want)
		}
	}
}
