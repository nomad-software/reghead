// Package engine runs regular expressions and explains them, behind an
// interface that further engines can implement. It has no dependency on the
// UI. To add an engine, implement Engine and list it in engines.
package engine

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// FlagGlobal is the flag, common to every engine, to find all matches rather
// than the first.
const FlagGlobal = "g"

// Flag is an option an engine offers.
type Flag struct {
	// Key is what the flag is known by in a string of flags.
	Key   string
	Label string
}

// flagsApart is for the engines of languages that write flags after the pattern:
// nothing is added to the regex field, and a flag setting within the pattern is
// ordinary syntax, or an error, and stays where it is.
type flagsApart struct{}

func (flagsApart) InlineFlags(string) string { return "" }

func (flagsApart) SplitInlineFlags(pattern string) (flags, rest string) { return "", pattern }

// Span is a range of bytes of a text. Start is -1 if a group took no part in
// a match.
type Span struct{ Start, End int }

// Match is a match: the span of the whole match, then that of each group.
type Match struct{ Groups []Span }

// Node is one step of an explanation.
type Node struct {
	// Source is the part of the pattern that is explained.
	Source string
	// Text says what it does in plain language.
	Text string
	// Tokens are Source split up for syntax highlighting.
	Tokens   []Token
	Children []Node
	// synthetic says it explains something that is not in the pattern as typed, as
	// flags are not.
	synthetic bool
	// literal is the text a plain literal matches, so that neighbours can
	// be joined together.
	literal string
	// start and end are where Source lies in the pattern.
	start, end int
	// group is the number of the capture group it is, if it is one.
	group int
}

// CaptureGroup returns the number of the capture group it is, or 0 if it is not one.
func (n Node) CaptureGroup() int { return n.group }

// InPattern says it explains part of the pattern as typed, which flags held apart
// from it are not.
func (n Node) InPattern() bool { return !n.synthetic }

// TokenKind is the sort of thing a token of a pattern is.
type TokenKind int

// The kinds of token.
const (
	// TokenLiteral is plain text.
	TokenLiteral TokenKind = iota
	// TokenMeta is a character with a special meaning: a quantifier, an anchor,
	// an alternator, a dot, or an escape such as \d.
	TokenMeta
	// TokenEscaped is an escaped literal, such as \. or \x41.
	TokenEscaped
	// TokenClass is part of a bracket expression.
	TokenClass
	// TokenClassMeta is an escape or a named class in a bracket expression.
	TokenClassMeta
	// TokenModifier is an inline flag setting, such as (?i).
	TokenModifier
	// TokenGroup is the bracket of a group, whose Depth is how deeply it nests.
	TokenGroup
	// TokenBackreference refers to a group.
	TokenBackreference
	// TokenComment has no effect.
	TokenComment
)

// Token is a part of a pattern.
type Token struct {
	Text  string
	Kind  TokenKind
	Depth int
}

// Regex is a compiled regular expression.
type Regex interface {
	// GroupNames gives the name of each group, "" where it has none. Index 0
	// is the whole match.
	GroupNames() []string
	// FindAll returns up to n matches of text, or all of them if n is negative. An
	// engine that gave up partway returns what it found with an error.
	FindAll(text string, n int) ([]Match, error)
}

// releaser is a Regex that holds memory outside Go, which Evaluate frees when it
// is done with it.
type releaser interface{ Release() }

// ReplacementPart is a part of a replacement template. It is either Literal text
// or, if IsRef, a reference to a group by Name or, where Number is not -1, number.
type ReplacementPart struct {
	// Source is the part as written.
	Source  string
	Literal string
	IsRef   bool
	Name    string
	Number  int
	// Escaped says it is literal text written with an escape, such as $$.
	Escaped bool
	// Before and After stand for the text before and after the match, as JavaScript's
	// $` and $' do.
	Before, After bool
}

// group returns the group the part refers to, given the names of the groups, or
// -1 if there is none. With a match, it is the first group of that name that took
// part in it; without, the first of that name.
func (p ReplacementPart) group(names []string, m *Match) int {
	took := func(g int) bool { return m == nil || m.Groups[g].Start >= 0 }
	if p.Number >= 0 {
		if p.Number < len(names) && took(p.Number) {
			return p.Number
		}
		return -1
	}
	for g, name := range names {
		if name == p.Name && took(g) {
			return g
		}
	}
	return -1
}

// GroupOf returns the group a reference refers to, or -1 if there is none.
func (p ReplacementPart) GroupOf(names []string) int { return p.group(names, nil) }

// Engine is a regular expression implementation.
type Engine interface {
	Name() string
	// Flags lists the engine's flags, besides FlagGlobal.
	Flags() []Flag
	// InlineFlags returns what the flags add to the start of a pattern, which is
	// shown in the regex field, or "" for an engine whose flags stand apart.
	InlineFlags(flags string) string
	// SplitInlineFlags takes flag settings typed or pasted at the start of a
	// pattern, as the flags alone control them, and returns those flags and the
	// rest of the pattern.
	SplitInlineFlags(pattern string) (flags, rest string)
	// ReplacementHint describes the syntax of replacement templates.
	ReplacementHint() string
	// ParseReplacement splits a replacement template into parts, whose Sources
	// spell it. Names are those of the groups, nil if the pattern has an error, as
	// what a template means can depend on them.
	ParseReplacement(template string, names []string) []ReplacementPart
	// FullPattern returns the pattern with its flags as it would be written in code.
	FullPattern(pattern, flags string) string
	// Compile returns an error fit to show to the user.
	Compile(pattern, flags string) (Regex, error)
	// Explain describes a pattern. A pattern that does not compile gets no
	// sensible explanation, but it must not fail, and its nodes' tokens must still
	// spell the pattern, as they highlight it as it is typed.
	Explain(pattern, flags string) []Node
}

var engines = []Engine{goEngine{}, pcreEngine{}, jsEngine{}}

// All returns the engines, in the order they are offered.
func All() []Engine { return engines }

// ByName returns the engine with the name, or the first if there is none, as
// when the settings name one that has gone.
func ByName(name string) Engine {
	for _, e := range engines {
		if e.Name() == name {
			return e
		}
	}
	return engines[0]
}

// Request is what to evaluate.
type Request struct {
	Engine      Engine
	Pattern     string
	Flags       string
	Text        string
	Substitute  bool
	Replacement string
}

// ReplacedGroup is where the text of a group of a match was put in the output.
type ReplacedGroup struct {
	Span
	Match, Group int
}

// Result is the outcome of a Request.
type Result struct {
	// Error says why the pattern does not compile; the rest is then empty.
	Error string
	// Warning says why the engine gave up partway, the rest being what it found.
	Warning     string
	Explanation []Node
	// Tokens are the whole pattern split up for syntax highlighting, which it has
	// even if it has an error.
	Tokens     []Token
	GroupNames []string
	// GroupRanges says where in the pattern each group lies. Index 0 is the whole
	// pattern, which stands for the whole match.
	GroupRanges []Span
	Matches     []Match
	Text        string
	Offsets     *Offsets
	// ReplacementParts is the template split up, which it has even if the pattern
	// has an error.
	ReplacementParts []ReplacementPart
	// Output is the text with its replacements, and Replaced where they are.
	Output        string
	OutputOffsets *Offsets
	Replaced      []Span
	// ReplacedGroups says where in Output the text of groups was put.
	ReplacedGroups []ReplacedGroup
}

// Evaluate runs a request. It takes no part in the UI, so that it can run away
// from the UI's thread.
func Evaluate(q Request) (r Result) {
	// It runs on a goroutine of its own, where a panic would end the program and lose
	// what was entered, over what is at worst one bad pattern.
	defer func() {
		if failure := recover(); failure != nil {
			r = Result{Text: q.Text, Offsets: NewOffsets(q.Text), Error: fmt.Sprintf("the engine failed on this pattern: %v", failure)}
		}
	}()
	r = Result{Text: q.Text, Offsets: NewOffsets(q.Text)}
	if q.Substitute {
		r.ReplacementParts = q.Engine.ParseReplacement(q.Replacement, nil)
	}
	if q.Pattern == "" {
		r.Output, r.OutputOffsets = q.Text, r.Offsets
		return r
	}
	nodes := q.Engine.Explain(q.Pattern, q.Flags)
	for _, n := range nodes {
		if !n.synthetic {
			r.Tokens = append(r.Tokens, n.Tokens...)
		}
	}
	re, err := q.Engine.Compile(q.Pattern, q.Flags)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if rel, ok := re.(releaser); ok {
		defer rel.Release()
	}
	r.Explanation = nodes
	r.GroupNames = re.GroupNames()
	r.GroupRanges = make([]Span, len(r.GroupNames))
	r.GroupRanges[0] = Span{0, len(q.Pattern)}
	collectGroups(nodes, r.GroupRanges)
	n := 1
	if strings.Contains(q.Flags, FlagGlobal) {
		n = -1
	}
	r.Matches, err = re.FindAll(q.Text, n)
	if err != nil {
		r.Warning = err.Error()
	}
	if q.Substitute {
		r.ReplacementParts = q.Engine.ParseReplacement(q.Replacement, r.GroupNames)
		r.substitute()
		r.OutputOffsets = NewOffsets(r.Output)
	}
	return r
}

// collectGroups notes where in the pattern each group lies.
func collectGroups(nodes []Node, ranges []Span) {
	for _, n := range nodes {
		// A branch reset has several groups of one number. The first stands for them.
		if n.group > 0 && n.group < len(ranges) && ranges[n.group] == (Span{}) {
			ranges[n.group] = Span{n.start, n.end}
		}
		collectGroups(n.Children, ranges)
	}
}

// substitute fills in the output: the text with each match replaced, noting where
// the replacements and the groups in them fell.
func (r *Result) substitute() {
	var out strings.Builder
	r.Replaced = make([]Span, 0, len(r.Matches))
	last := 0
	for i := range r.Matches {
		m := &r.Matches[i]
		whole := m.Groups[0]
		out.WriteString(r.Text[last:whole.Start])
		start := out.Len()
		for _, part := range r.ReplacementParts {
			switch {
			case part.Before:
				out.WriteString(r.Text[:whole.Start])
				continue
			case part.After:
				out.WriteString(r.Text[whole.End:])
				continue
			case !part.IsRef:
				out.WriteString(part.Literal)
				continue
			}
			g := part.group(r.GroupNames, m)
			if g < 0 {
				continue
			}
			span := m.Groups[g]
			at := out.Len()
			out.WriteString(r.Text[span.Start:span.End])
			r.ReplacedGroups = append(r.ReplacedGroups, ReplacedGroup{Span{at, out.Len()}, i, g})
		}
		r.Replaced = append(r.Replaced, Span{start, out.Len()})
		last = whole.End
	}
	out.WriteString(r.Text[last:])
	r.Output = out.String()
}

// Offsets converts byte offsets of a text to the positions of its characters,
// which are what people count, and to UTF-16 code units, which are what Qt
// counts.
type Offsets struct{ runes, utf16 []int32 }

// NewOffsets indexes a text.
func NewOffsets(text string) *Offsets {
	o := &Offsets{make([]int32, len(text)+1), make([]int32, len(text)+1)}
	var runes, units int32
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		for j := range size {
			o.runes[i+j], o.utf16[i+j] = runes, units
		}
		runes++
		units++
		if r > 0xFFFF {
			units++
		}
		i += size
	}
	o.runes[len(text)], o.utf16[len(text)] = runes, units
	return o
}

// Rune returns the character position of a byte offset.
func (o *Offsets) Rune(b int) int { return int(o.runes[b]) }

// UTF16 returns the UTF-16 position of a byte offset.
func (o *Offsets) UTF16(b int) int { return int(o.utf16[b]) }

// ByteAtUTF16 returns the byte offset of the character at a UTF-16 position.
func (o *Offsets) ByteAtUTF16(position int) int {
	at, _ := slices.BinarySearch(o.utf16, int32(position))
	return at
}

// delimited writes a pattern as a language writes it, between slashes with its
// flags after them, in the order the engine lists them. The global flag is the
// caller's to pass in the list, as not every language has one.
func delimited(pattern string, list []Flag, flags string) string {
	var b strings.Builder
	b.WriteByte('/')
	inClass := false
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '\\' && i+1 < len(pattern):
			b.WriteByte(c)
			i++
			b.WriteByte(pattern[i])
			continue
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '/' && !inClass:
			b.WriteByte('\\')
		}
		b.WriteByte(pattern[i])
	}
	b.WriteByte('/')
	for _, f := range list {
		if strings.Contains(flags, f.Key) {
			b.WriteString(f.Key)
		}
	}
	return b.String()
}
