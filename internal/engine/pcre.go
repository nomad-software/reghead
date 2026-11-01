package engine

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	qt "github.com/mappu/miqt/qt6"
)

// pcreEngine is the real PCRE2, through Qt's QRegularExpression, which needs no
// QApplication and can run away from the UI's thread. Qt always compiles in UTF
// mode, so that positions go by character.
type pcreEngine struct{ flagsApart }

func (pcreEngine) Name() string { return "PCRE2" }

var pcreFlags = []Flag{
	{"i", "Case Insensitive"},
	{"m", "Multi-line"},
	{"s", "Dot Matches Newline"},
	{"x", "Extended (Ignore Whitespace)"},
	{"U", "Ungreedy"},
	{"u", "Unicode"},
}

func (pcreEngine) Flags() []Flag { return pcreFlags }

func (pcreEngine) FullPattern(pattern, flags string) string {
	return delimited(pattern, pcreFlags, flags)
}

func (pcreEngine) ReplacementHint() string {
	return `$1, ${1} or \1 for a group, $0 for the match`
}

// ParseReplacement takes the usual PCRE2 forms: $n, ${n} and \n, of up to two
// digits, are groups, and \\ is a backslash. There are no references by name.
func (pcreEngine) ParseReplacement(template string, _ []string) []ReplacementPart {
	var parts []ReplacementPart
	literal := func(from, to int) {
		if to > from {
			parts = append(parts, ReplacementPart{Source: template[from:to], Literal: template[from:to], Number: -1})
		}
	}
	from := 0
	for i := 0; i < len(template); {
		if strings.HasPrefix(template[i:], `\\`) {
			literal(from, i)
			parts = append(parts, ReplacementPart{Source: `\\`, Literal: `\`, Number: -1, Escaped: true})
			i += 2
			from = i
			continue
		}
		length := pcreReference(template[i:])
		if length == 0 {
			i++
			continue
		}
		literal(from, i)
		source := template[i : i+length]
		number, _ := strconv.Atoi(strings.Trim(source, `$\{}`))
		parts = append(parts, ReplacementPart{Source: source, IsRef: true, Number: number})
		i += length
		from = i
	}
	literal(from, len(template))
	return parts
}

// pcreReference returns the length of the reference that text starts with, or 0.
func pcreReference(text string) int {
	digits := func(s string) int {
		n := 0
		for n < len(s) && n < 2 && s[n] >= '0' && s[n] <= '9' {
			n++
		}
		return n
	}
	switch {
	case strings.HasPrefix(text, "${"):
		if n := digits(text[2:]); n > 0 && strings.HasPrefix(text[2+n:], "}") {
			return n + 3
		}
	case strings.HasPrefix(text, "$"), strings.HasPrefix(text, `\`):
		if n := digits(text[1:]); n > 0 {
			return n + 1
		}
	}
	return 0
}

func (pcreEngine) Compile(pattern, flags string) (Regex, error) {
	options := qt.QRegularExpression__NoPatternOption
	for key, option := range map[string]qt.QRegularExpression__PatternOption{
		"i": qt.QRegularExpression__CaseInsensitiveOption,
		"m": qt.QRegularExpression__MultilineOption,
		"s": qt.QRegularExpression__DotMatchesEverythingOption,
		"x": qt.QRegularExpression__ExtendedPatternSyntaxOption,
		"U": qt.QRegularExpression__InvertedGreedinessOption,
		"u": qt.QRegularExpression__UseUnicodePropertiesOption,
	} {
		if strings.Contains(flags, key) {
			options |= option
		}
	}
	re := qt.NewQRegularExpression4(pattern, options)
	re.GoGC()
	if !re.IsValid() {
		return nil, fmt.Errorf("%s at position %d", re.ErrorString(), re.PatternErrorOffset())
	}
	re.Optimize()
	return pcreRegex{re}, nil
}

func (pcreEngine) Explain(pattern, flags string) []Node {
	return explain(&pcreDialect, pattern, flags)
}

type pcreRegex struct{ re *qt.QRegularExpression }

func (p pcreRegex) GroupNames() []string { return p.re.NamedCaptureGroups() }

func (p pcreRegex) FindAll(text string, n int) ([]Match, error) {
	defer runtime.KeepAlive(p.re)
	offsets := NewOffsets(text)
	groups := p.re.CaptureCount() + 1
	var matches []Match
	add := func(m *qt.QRegularExpressionMatch) {
		spans := make([]Span, groups)
		for g := range spans {
			spans[g] = Span{-1, -1}
			if start := m.CapturedStartWithNth(g); start >= 0 {
				spans[g] = Span{offsets.ByteAtUTF16(int(start)), offsets.ByteAtUTF16(int(m.CapturedEndWithNth(g)))}
			}
		}
		matches = append(matches, Match{spans})
	}
	if n == 1 {
		m := p.re.Match(text)
		if !m.IsValid() {
			return nil, errPCRELimit
		}
		if m.HasMatch() {
			add(m)
		}
		return matches, nil
	}
	from := int64(0)
	for it := p.re.GlobalMatch(text); it.HasNext(); {
		m := it.Next()
		add(m)
		from = m.CapturedEnd()
	}
	// Qt ends a search that hit PCRE2's limits as if nothing more was found. Asking
	// again from there tells the two apart, as the answer is then not valid.
	if !p.re.Match2(text, from).IsValid() {
		return matches, errPCRELimit
	}
	return matches, nil
}

var errPCRELimit = errors.New("PCRE2 stopped at its match limit: the pattern backtracks too much, so there may be more matches")
