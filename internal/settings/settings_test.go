package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Default()
	s.Width, s.Height, s.SideWidth, s.ExplainHeight, s.TextHeight = 900, 500, 300, 200, 150
	s.MatchColumns = []int{120, 80}
	s.Theme, s.Mode, s.Flags, s.Pattern, s.Text = ThemeDark, ModeSubstitute, "", "", ""
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if got := Load(path); !reflect.DeepEqual(got, s) {
		t.Errorf("got %+v, want %+v", got, s)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions %v", info.Mode().Perm())
	}
}

func TestLoadBadValues(t *testing.T) {
	dir := t.TempDir()
	if got := Load(filepath.Join(dir, "missing")); !reflect.DeepEqual(got, Default()) {
		t.Errorf("missing file: %+v", got)
	}
	corrupt := filepath.Join(dir, "corrupt")
	os.WriteFile(corrupt, []byte("{nope"), 0o600)
	if got := Load(corrupt); !reflect.DeepEqual(got, Default()) {
		t.Errorf("corrupt file: %+v", got)
	}
	bad := filepath.Join(dir, "bad")
	os.WriteFile(bad, []byte(`{"width":1,"height":1,"theme":"pink","mode":"x","sideWidth":-4}`), 0o600)
	got := Load(bad)
	if got.Width != defaultWidth || got.Theme != "" || got.Mode != ModeMatch || got.SideWidth != 0 {
		t.Errorf("bad values: %+v", got)
	}
}
