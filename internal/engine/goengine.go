package engine

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// goEngine is Go's own regexp package.
type goEngine struct{}

func (goEngine) Name() string { return "Go" }

var goFlags = []Flag{
	{"i", "Case Insensitive"},
	{"m", "Multi-line"},
	{"s", "Dot Matches Newline"},
	{"U", "Ungreedy"},
}

func (goEngine) Flags() []Flag { return goFlags }

func (goEngine) InlineFlags(flags string) string { return goFlagPrefix(flags) }

var goLeadingFlags = regexp.MustCompile(`^\(\?([imsU]+)\)`)

func (goEngine) SplitInlineFlags(pattern string) (flags, rest string) {
	for {
		m := goLeadingFlags.FindStringSubmatch(pattern)
		if m == nil {
			return flags, pattern
		}
		flags += m[1]
		pattern = pattern[len(m[0]):]
	}
}

func (goEngine) FullPattern(pattern, flags string) string {
	return goFlagPrefix(flags) + pattern
}

func (goEngine) ReplacementHint() string {
	return "$1 or ${name} for a group, $$ for a dollar"
}

// ParseReplacement follows the rules of regexp.Regexp.Expand: $name or ${name} is a
// group, by number if the name is all digits, $$ is a dollar, and any other $ is
// itself.
func (goEngine) ParseReplacement(template string, _ []string) []ReplacementPart {
	var parts []ReplacementPart
	literal := func(text string) {
		if text != "" {
			parts = append(parts, ReplacementPart{Source: text, Literal: text, Number: -1})
		}
	}
	for {
		before, after, found := strings.Cut(template, "$")
		if !found {
			literal(template)
			return parts
		}
		literal(before)
		if strings.HasPrefix(after, "$") {
			parts = append(parts, ReplacementPart{Source: "$$", Literal: "$", Number: -1, Escaped: true})
			template = after[1:]
			continue
		}
		name, number, length := goReference(after)
		if length == 0 {
			literal("$")
			template = after
			continue
		}
		parts = append(parts, ReplacementPart{Source: "$" + after[:length], IsRef: true, Name: name, Number: number})
		template = after[length:]
	}
}

// goReference reads the name or {name} at the start of text, returning its length
// including any braces, 0 if there is none, and its number, -1 if it is not one.
func goReference(text string) (name string, number, length int) {
	brace := strings.HasPrefix(text, "{")
	rest := strings.TrimPrefix(text, "{")
	i := 0
	for i < len(rest) {
		r, size := utf8.DecodeRuneInString(rest[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		i += size
	}
	if i == 0 || (brace && (i >= len(rest) || rest[i] != '}')) {
		return "", -1, 0
	}
	name = rest[:i]
	length = i
	if brace {
		length += 2
	}
	for j := 0; j < len(name); j++ {
		if name[j] < '0' || '9' < name[j] || number >= 1e8 {
			number = -1
			break
		}
		number = number*10 + int(name[j]-'0')
	}
	if name[0] == '0' && len(name) > 1 {
		number = -1
	}
	return name, number, length
}

func (goEngine) Compile(pattern, flags string) (Regex, error) {
	// The pattern is checked as it was typed, so that an error quotes it
	// without the prefix of flags.
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, cleanError{err}
	}
	if prefix := goFlagPrefix(flags); prefix != "" {
		if re, err = regexp.Compile(prefix + pattern); err != nil {
			return nil, cleanError{err}
		}
	}
	return goRegex{re}, nil
}

func goFlagPrefix(flags string) string {
	var on strings.Builder
	for _, f := range goFlags {
		if strings.Contains(flags, f.Key) {
			on.WriteString(f.Key)
		}
	}
	if on.Len() == 0 {
		return ""
	}
	return "(?" + on.String() + ")"
}

// cleanError drops the prefix Go gives its syntax errors.
type cleanError struct{ error }

func (e cleanError) Error() string {
	return strings.TrimPrefix(e.error.Error(), "error parsing regexp: ")
}

type goRegex struct{ re *regexp.Regexp }

func (g goRegex) GroupNames() []string { return g.re.SubexpNames() }

func (g goRegex) FindAll(text string, n int) ([]Match, error) {
	found := g.re.FindAllStringSubmatchIndex(text, n)
	matches := make([]Match, len(found))
	for i, loc := range found {
		groups := make([]Span, len(loc)/2)
		for j := range groups {
			groups[j] = Span{loc[2*j], loc[2*j+1]}
		}
		matches[i] = Match{groups}
	}
	return matches, nil
}

func (goEngine) Explain(pattern, flags string) []Node {
	nodes := explain(&goDialect, pattern, flags)
	// The flags come first, as the field shows them and the engine runs them.
	if prefix := goFlagPrefix(flags); prefix != "" {
		q := explainer{d: &goDialect}
		first := Node{
			Source:    prefix,
			Text:      upperFirst(q.applyFlags(prefix[2:len(prefix)-1])) + " for the rest of the pattern",
			Tokens:    []Token{{Text: prefix, Kind: TokenModifier}},
			synthetic: true,
		}
		nodes = append([]Node{first}, nodes...)
	}
	return nodes
}
