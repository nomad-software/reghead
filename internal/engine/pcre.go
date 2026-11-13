package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nomad-software/qute/qtcore"
)

// pcreEngine is the real PCRE2, through Qt's QRegularExpression, which needs no
// QApplication and, being reentrant, can run away from the UI's thread. Qt always compiles in UTF
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
	options := qtcore.QRegularExpressionNoPatternOption
	for key, option := range map[string]qtcore.QRegularExpressionPatternOption{
		"i": qtcore.QRegularExpressionCaseInsensitiveOption,
		"m": qtcore.QRegularExpressionMultilineOption,
		"s": qtcore.QRegularExpressionDotMatchesEverythingOption,
		"x": qtcore.QRegularExpressionExtendedPatternSyntaxOption,
		"U": qtcore.QRegularExpressionInvertedGreedinessOption,
		"u": qtcore.QRegularExpressionUseUnicodePropertiesOption,
	} {
		if strings.Contains(flags, key) {
			options |= option
		}
	}
	re := qtcore.NewQRegularExpressionWithPatternFull(pattern, options)
	if !re.IsValid() {
		err := fmt.Errorf("%s at position %d", re.ErrorString(), re.PatternErrorOffset())
		re.Delete()
		return nil, err
	}
	re.Optimize()
	return pcreRegex{re}, nil
}

func (pcreEngine) Explain(pattern, flags string) []Node {
	return explain(&pcreDialect, pattern, flags)
}

type pcreRegex struct{ re *qtcore.QRegularExpression }

func (p pcreRegex) GroupNames() []string { return p.re.NamedCaptureGroups() }

// Release frees the compiled pattern at once. The garbage collector would only
// hand it to the UI's thread to free, which without a QApplication, as in the
// tests, never comes.
func (p pcreRegex) Release() { p.re.Delete() }

func (p pcreRegex) FindAll(text string, n int) ([]Match, error) {
	offsets := NewOffsets(text)
	groups := p.re.CaptureCount() + 1
	var matches []Match
	add := func(m *qtcore.QRegularExpressionMatch) {
		spans := make([]Span, groups)
		for g := range spans {
			spans[g] = Span{-1, -1}
			if start := m.CapturedStartFull(g); start >= 0 {
				spans[g] = Span{offsets.ByteAtUTF16(start), offsets.ByteAtUTF16(m.CapturedEndFull(g))}
			}
		}
		matches = append(matches, Match{spans})
	}
	if n == 1 {
		m := p.re.Match(text)
		defer m.Delete()
		if !m.IsValid() {
			return nil, errPCRELimit
		}
		if m.HasMatch() {
			add(m)
		}
		return matches, nil
	}
	from := 0
	it := p.re.GlobalMatch(text)
	for it.HasNext() {
		m := it.Next()
		add(m)
		from = m.CapturedEnd()
		m.Delete()
	}
	it.Delete()
	// Qt ends a search that hit PCRE2's limits as if nothing more was found. Asking
	// again from there tells the two apart, as the answer is then not valid.
	last := p.re.MatchFull(text, from, qtcore.QRegularExpressionNormalMatch, qtcore.QRegularExpressionNoMatchOption)
	defer last.Delete()
	if !last.IsValid() {
		return matches, errPCRELimit
	}
	return matches, nil
}

var errPCRELimit = errors.New("PCRE2 stopped at its match limit: the pattern backtracks too much, so there may be more matches")
