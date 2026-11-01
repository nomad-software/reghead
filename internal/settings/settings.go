// Package settings stores what RegHead remembers between runs, in a JSON file
// in the system's configuration directory (~/.config/reghead on Linux).
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// The modes of the program.
const (
	ModeMatch      = "match"
	ModeSubstitute = "substitute"
)

// The themes that can be chosen, besides following the system, which is "".
const (
	ThemeLight = "light"
	ThemeDark  = "dark"
)

const (
	defaultWidth  = 1200
	defaultHeight = 760
	minSize       = 200
)

// Settings is the state saved to disk. Pane sizes of zero mean the default
// layout.
type Settings struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`

	// SideWidth is the width of the right hand panes, ExplainHeight the height
	// of the explanation pane above the matches, and TextHeight the height of
	// the test text above the substitution pane.
	SideWidth     int `json:"sideWidth"`
	ExplainHeight int `json:"explainHeight"`
	TextHeight    int `json:"textHeight"`
	// MatchColumns are the widths of the columns of the match list, except the
	// last, which fills what is left.
	MatchColumns []int `json:"matchColumns,omitempty"`

	Theme  string `json:"theme"`
	Mode   string `json:"mode"`
	Engine string `json:"engine"`
	// Flags holds the key of each flag that is on.
	Flags string `json:"flags"`

	Pattern     string `json:"pattern"`
	Text        string `json:"text"`
	Replacement string `json:"replacement"`
}

// Default returns the settings of a first run, with an example to explore.
func Default() Settings {
	return Settings{
		Width:       defaultWidth,
		Height:      defaultHeight,
		Mode:        ModeMatch,
		Flags:       "g",
		Pattern:     `(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`,
		Text:        "RegHead was started on 2026-11-01.\nThe next release is due on 2027-01-15.\n",
		Replacement: "${day}/${month}/${year}",
	}
}

// Path returns where the settings file is kept.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reghead", "settings.json"), nil
}

// Prepare makes the directory of the settings file at path.
func Prepare(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o700)
}

// Load reads the settings at path. A file that is missing or unreadable gives
// the defaults, as does any value that is missing or unusable.
func Load(path string) Settings {
	s := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if json.Unmarshal(data, &s) != nil {
		return Default()
	}
	if s.Width < minSize || s.Height < minSize {
		s.Width, s.Height = defaultWidth, defaultHeight
	}
	s.SideWidth = max(s.SideWidth, 0)
	s.ExplainHeight = max(s.ExplainHeight, 0)
	s.TextHeight = max(s.TextHeight, 0)
	if s.Theme != ThemeLight && s.Theme != ThemeDark {
		s.Theme = ""
	}
	if s.Mode != ModeSubstitute {
		s.Mode = ModeMatch
	}
	return s
}

// Save replaces the file at path atomically, readable only by the user as it
// holds whatever text was being tested.
func (s Settings) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
