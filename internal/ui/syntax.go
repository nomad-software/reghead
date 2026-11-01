package ui

import (
	"fmt"

	"github.com/nomad-software/reghead/internal/engine"
)

// tokenColour is how a token of a pattern is drawn.
type tokenColour struct{ text, background string }

// syntax is the colours of matches and patterns. The program's own light and dark
// palettes are in theme.go.
type syntax struct {
	// fills holds, for the whole match and then for each group, the background
	// of odd and even matches.
	fills [7][2]string
	// edges holds the strong colour of each, which outlines a dot in
	// the list of matches, where a pale fill alone would hardly show.
	edges [7]string
	// groups holds the brackets of groups by how deeply they nest.
	groups                          [5]tokenColour
	meta, escaped, class, classMeta tokenColour
	modifier                        tokenColour
	backreference, comment          tokenColour
	errorText                       string
	// hoverText is the text over a group's strong colour when it is hovered.
	hoverText string
}

var (
	lightSyntax = syntax{
		fills: [7][2]string{
			{"#e2eef9", "#c4d9ec"}, {"#e6f0e3", "#cbdcc6"}, {"#f4ebde", "#e4d3bd"}, {"#ebebf9", "#d3d3ed"},
			{"#f2e8f5", "#e0cfe5"}, {"#f9e8e5", "#eccec9"}, {"#e1f0ee", "#c2dcd8"},
		},
		edges: [7]string{"#398ed1", "#5e9a48", "#b47a07", "#817ad2", "#a86cba", "#ca6658", "#169c91"},
		groups: [5]tokenColour{
			{"#22643d", "#e4f2e7"}, {"#205c37", "#d8eadd"}, {"#1c5432", "#cbe1d0"}, {"#194c2c", "#bed8c3"}, {"#154426", "#b1cfb6"},
		},
		meta:          tokenColour{"#185aa6", "#e3edfb"},
		escaped:       tokenColour{"#465363", "#e9edf2"},
		class:         tokenColour{"#854a15", "#fbecd8"},
		classMeta:     tokenColour{"#854a15", "#f4dfbf"},
		modifier:      tokenColour{"#703f8d", "#eee4f6"},
		backreference: tokenColour{"#155f78", "#dff0f6"},
		comment:       tokenColour{text: "#626873"},
		errorText:     "#c62828",
		hoverText:     "#ffffff",
	}
	darkSyntax = syntax{
		fills: [7][2]string{
			{"#203243", "#2c4258"}, {"#29341f", "#38452b"}, {"#392e18", "#4c3e22"}, {"#2f2d43", "#3f3c58"},
			{"#3a293b", "#4d374e"}, {"#422826", "#563634"}, {"#1a362d", "#25483c"},
		},
		edges: [7]string{"#529de5", "#7ca84e", "#be9022", "#958ae4", "#c17ac4", "#dd7571", "#1eb18b"},
		groups: [5]tokenColour{
			{"#9bd6ad", "#253c2e"}, {"#a4ddb5", "#2b4534"}, {"#b1e4be", "#324f3b"}, {"#bfebc9", "#3a5943"}, {"#cff1d5", "#42634b"},
		},
		meta:          tokenColour{"#a7caff", "#263c58"},
		escaped:       tokenColour{"#c2cbd8", "#343b46"},
		class:         tokenColour{"#eac28d", "#4a3825"},
		classMeta:     tokenColour{"#eac28d", "#59422b"},
		modifier:      tokenColour{"#d1b0eb", "#42314f"},
		backreference: tokenColour{"#99d1e8", "#263f4c"},
		comment:       tokenColour{text: "#abb2bf"},
		errorText:     "#ff7b7b",
		hoverText:     "#0b0b0b",
	}
)

// syntaxFor returns the colours for a dark or a light background.
func syntaxFor(dark bool) syntax {
	if dark {
		return darkSyntax
	}
	return lightSyntax
}

// fill returns the background of a group of a match, where group 0 is the whole
// match. Further groups than there are colours go round them again, leaving
// out that of the whole match. Matches alternate so that those that touch can
// be told apart.
func (s syntax) fill(group, match int) string {
	return s.fills[colourIndex(group, len(s.fills))][match%2]
}

// edge returns the outline of the dot of a group.
func (s syntax) edge(group int) string {
	return s.edges[colourIndex(group, len(s.edges))]
}

// colourIndex returns which of the colours a group has, the first being kept for
// the whole match and the rest going round.
func colourIndex(group, colours int) int {
	if group > 0 {
		return (group-1)%(colours-1) + 1
	}
	return 0
}

// token returns how a token is drawn. A literal has no colours of its own.
func (s syntax) token(t engine.Token) tokenColour {
	switch t.Kind {
	case engine.TokenMeta:
		return s.meta
	case engine.TokenEscaped:
		return s.escaped
	case engine.TokenClass:
		return s.class
	case engine.TokenClassMeta:
		return s.classMeta
	case engine.TokenModifier:
		return s.modifier
	case engine.TokenBackreference:
		return s.backreference
	case engine.TokenComment:
		return s.comment
	case engine.TokenGroup:
		return s.groups[t.Depth%len(s.groups)]
	}
	return tokenColour{}
}

// tokenStyle returns the style sheet of a token.
func tokenStyle(c tokenColour) string {
	if c.text == "" {
		return ""
	}
	if c.background == "" {
		return "color:" + c.text
	}
	return fmt.Sprintf("color:%s;background-color:%s", c.text, c.background)
}
