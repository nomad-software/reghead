package engine

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// jsTimeout is how long a pattern can search before it is stopped, as a
// backtracking engine can take for ever over the wrong pattern.
const jsTimeout = 2 * time.Second

// jsEngine simulates JavaScript's RegExp on regexp2, a pure Go backtracking engine
// of the same family, by translating patterns to mean the same there. It works on
// code points throughout, as JavaScript does only with the u flag.
type jsEngine struct{ flagsApart }

func (jsEngine) Name() string { return "JavaScript" }

var jsFlags = []Flag{
	{"i", "Case Insensitive"},
	{"m", "Multi-line"},
	{"s", "Dot Matches Newline"},
	{"u", "Unicode"},
	{"y", "Sticky"},
}

func (jsEngine) Flags() []Flag { return jsFlags }

func (jsEngine) FullPattern(pattern, flags string) string {
	return delimited(pattern, append([]Flag{{Key: FlagGlobal}}, jsFlags...), flags)
}

func (jsEngine) ReplacementHint() string {
	return "$1 or $<name> for a group, $& for the match, $$ for a dollar"
}

// ParseReplacement follows String.prototype.replace. A number is a reference only
// so far as there are such groups, and $<name> only if any group has a name. With
// no groups known, the pattern having an error, any number is taken as one.
func (jsEngine) ParseReplacement(template string, names []string) []ReplacementPart {
	var parts []ReplacementPart
	from := 0
	flush := func(to int) {
		if to > from {
			parts = append(parts, ReplacementPart{Source: template[from:to], Literal: template[from:to], Number: -1})
		}
	}
	named := false
	for _, name := range names {
		named = named || name != ""
	}
	for i := 0; i+1 < len(template); i++ {
		if template[i] != '$' {
			continue
		}
		part, length := jsReference(template[i:], names, named)
		if length == 0 {
			continue
		}
		flush(i)
		part.Source = template[i : i+length]
		parts = append(parts, part)
		i += length - 1
		from = i + 1
	}
	flush(len(template))
	return parts
}

// jsReference reads the $ reference that text starts with, returning its length,
// or 0 if the $ is just itself.
func jsReference(text string, names []string, named bool) (ReplacementPart, int) {
	switch text[1] {
	case '$':
		return ReplacementPart{Literal: "$", Number: -1, Escaped: true}, 2
	case '&':
		return ReplacementPart{IsRef: true, Number: 0}, 2
	case '`':
		return ReplacementPart{Number: -1, Before: true}, 2
	case '\'':
		return ReplacementPart{Number: -1, After: true}, 2
	case '<':
		if end := strings.IndexByte(text, '>'); named && end > 0 {
			return ReplacementPart{IsRef: true, Name: text[2:end], Number: -1}, end + 1
		}
		return ReplacementPart{}, 0
	}
	known := func(n int) bool { return n >= 1 && (names == nil || n < len(names)) }
	for length := 2; length >= 1; length-- {
		if len(text) < 1+length {
			continue
		}
		if n, err := strconv.Atoi(text[1 : 1+length]); err == nil && text[1] >= '0' && known(n) {
			return ReplacementPart{IsRef: true, Number: n}, 1 + length
		}
	}
	return ReplacementPart{}, 0
}

func (jsEngine) Compile(pattern, flags string) (re Regex, err error) {
	// regexp2 is given text it was not written for, and a slip in it is an error in
	// the pattern to the person typing, not the end of the program.
	defer func() {
		if r := recover(); r != nil {
			re, err = nil, fmt.Errorf("the pattern could not be compiled: %v", r)
		}
	}()
	translated, names, err := translateJS(pattern, flags)
	if err != nil {
		return nil, err
	}
	options := regexp2.None
	if strings.Contains(flags, "i") {
		options |= regexp2.IgnoreCase
	}
	compiled, err := regexp2.Compile(translated, options)
	if err != nil {
		message, _, _ := strings.Cut(strings.TrimPrefix(err.Error(), "error parsing regexp: "), " in `")
		return nil, errors.New(message)
	}
	compiled.MatchTimeout = jsTimeout
	return jsRegex{compiled, names}, nil
}

func (jsEngine) Explain(pattern, flags string) []Node {
	return explain(&jsDialect, pattern, flags)
}

type jsRegex struct {
	re    *regexp2.Regexp
	names []string
}

func (j jsRegex) GroupNames() []string { return j.names }

// FindAll returns what was found before any timeout along with the error.
func (j jsRegex) FindAll(text string, n int) ([]Match, error) {
	// regexp2 counts in characters.
	bytesAt := make([]int, 0, len(text)+1)
	for i := range text {
		bytesAt = append(bytesAt, i)
	}
	bytesAt = append(bytesAt, len(text))

	deadline := time.Now().Add(jsTimeout)
	var matches []Match
	m, err := j.re.FindStringMatch(text)
	for m != nil && err == nil {
		spans := make([]Span, len(j.names))
		for g := range spans {
			spans[g] = Span{-1, -1}
			if group := m.GroupByNumber(g); group != nil && len(group.Captures) > 0 {
				spans[g] = Span{bytesAt[group.Index], bytesAt[group.Index+group.Length]}
			}
		}
		matches = append(matches, Match{spans})
		if len(matches) == n {
			return matches, nil
		}
		if time.Now().After(deadline) {
			return matches, fmt.Errorf("stopped after %s, so there may be more matches", jsTimeout)
		}
		m, err = j.re.FindNextMatch(m)
	}
	if err != nil {
		return matches, fmt.Errorf("stopped after %s: the pattern backtracks too much", jsTimeout)
	}
	return matches, nil
}

// runeRange is an inclusive range of characters.
type runeRange struct{ lo, hi rune }

// The sets JavaScript's shorthands stand for. regexp2's follow .NET, and cover far
// more of Unicode, so they are spelled out.
var (
	digitSet   = []runeRange{{'0', '9'}}
	wordSet    = []runeRange{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}
	spaceSet   = []runeRange{{9, 13}, {' ', ' '}, {0xA0, 0xA0}, {0x1680, 0x1680}, {0x2000, 0x200A}, {0x2028, 0x2029}, {0x202F, 0x202F}, {0x205F, 0x205F}, {0x3000, 0x3000}, {0xFEFF, 0xFEFF}}
	newlineSet = []runeRange{{10, 10}, {13, 13}, {0x2028, 0x2029}}
	// notLineBreak is a bracket expression of what is not a line break.
	notLineBreak = "[" + setBody(complementOf(newlineSet)) + "]"
)

// complementOf returns the characters a sorted set leaves out.
func complementOf(set []runeRange) []runeRange {
	var out []runeRange
	next := rune(0)
	for _, r := range set {
		if r.lo > next {
			out = append(out, runeRange{next, r.lo - 1})
		}
		next = r.hi + 1
	}
	return append(out, runeRange{next, utf8.MaxRune})
}

// escapedRune writes a character, as a code if it might not survive as itself, and
// with a backslash if it is one of those that are special where it goes.
func escapedRune(r rune, special string) string {
	switch {
	case r < 0x20 || (r >= 0x7F && r <= 0xFFFF):
		return fmt.Sprintf(`\u%04X`, r)
	case strings.ContainsRune(special, r):
		return `\` + string(r)
	}
	return string(r)
}

// classRune writes a character as it may appear in a bracket expression.
func classRune(r rune) string { return escapedRune(r, `\]^-[`) }

// setBody writes a set for inside a bracket expression.
func setBody(set []runeRange) string {
	var b strings.Builder
	for _, r := range set {
		b.WriteString(classRune(r.lo))
		if r.hi > r.lo {
			b.WriteString("-" + classRune(r.hi))
		}
	}
	return b.String()
}

// literalRune writes a character as it may appear outside a bracket expression.
func literalRune(r rune) string { return escapedRune(r, `\.+*?()|[]{}^$#`) }

// jsScope is the flags in force at a point of a pattern.
type jsScope struct{ multi, dotAll bool }

// jsTranslator rewrites a JavaScript pattern for regexp2. Groups all become
// numbered, as regexp2 numbers named groups after the others, where JavaScript
// numbers them in order.
type jsTranslator struct {
	s       string
	pos     int
	out     strings.Builder
	scope   jsScope
	stack   []jsScope
	unicode bool
	names   []string
	numbers map[string]int
}

// translateJS returns a pattern as regexp2 takes it, and the name of each group,
// "" if it has none, where index 0 is the whole match.
func translateJS(pattern, flags string) (string, []string, error) {
	names, err := jsGroups(pattern)
	if err != nil {
		return "", nil, err
	}
	t := jsTranslator{
		s:       pattern,
		scope:   jsScope{strings.Contains(flags, "m"), strings.Contains(flags, "s")},
		unicode: strings.Contains(flags, "u"),
		names:   names,
		numbers: map[string]int{},
	}
	for g, name := range names {
		if name != "" {
			t.numbers[name] = g
		}
	}
	for t.pos < len(t.s) {
		if err := t.step(); err != nil {
			return "", nil, err
		}
	}
	translated := t.out.String()
	// Sticky has each match start where the last one ended.
	if strings.Contains(flags, "y") {
		translated = `\G(?:` + translated + ")"
	}
	return translated, names, nil
}

var jsGroupName = regexp.MustCompile(`^[\p{L}\p{Nl}$_][\p{L}\p{Nl}\p{Mn}\p{Mc}\p{Nd}\p{Pc}$]*$`)

// jsGroups finds the capture groups, as references can look forward.
func jsGroups(s string) ([]string, error) {
	names := []string{""}
	inClass := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\':
			i++
		case inClass:
			inClass = c != ']'
		case c == '[':
			inClass = true
		case c != '(':
		case !strings.HasPrefix(s[i:], "(?"):
			names = append(names, "")
		case strings.HasPrefix(s[i:], "(?<") && !strings.HasPrefix(s[i:], "(?<=") && !strings.HasPrefix(s[i:], "(?<!"):
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				return nil, errors.New("group name is not closed")
			}
			name := s[i+3 : i+end]
			if !jsGroupName.MatchString(name) {
				return nil, fmt.Errorf("invalid group name %q", name)
			}
			if slices.Contains(names, name) {
				return nil, fmt.Errorf("two groups are named %q", name)
			}
			names = append(names, name)
		}
	}
	return names, nil
}

func (t *jsTranslator) rest() string { return t.s[t.pos:] }

func (t *jsTranslator) step() error {
	switch c := t.s[t.pos]; c {
	case '\\':
		return t.escape()
	case '[':
		return t.class()
	case '(':
		return t.open()
	case ')':
		if len(t.stack) == 0 {
			return errors.New("unmatched )")
		}
		t.scope = t.stack[len(t.stack)-1]
		t.stack = t.stack[:len(t.stack)-1]
	case '.':
		t.pos++
		if t.scope.dotAll {
			t.out.WriteString(`[\s\S]`)
		} else {
			t.out.WriteString(notLineBreak)
		}
		return nil
	case '^', '$':
		t.pos++
		t.out.WriteString(t.anchor(c))
		return nil
	}
	// The rest, as repeats, mean the same to regexp2. They are copied whole.
	_, size := utf8.DecodeRuneInString(t.rest())
	t.out.WriteString(t.rest()[:size])
	t.pos += size
	return nil
}

// anchor writes ^ or $. JavaScript's $ allows for no final newline, and its lines
// end in more ways than regexp2's.
func (t *jsTranslator) anchor(c byte) string {
	switch {
	case c == '^' && t.scope.multi:
		return "(?<!" + notLineBreak + ")"
	case c == '^':
		return `\A`
	case t.scope.multi:
		return "(?!" + notLineBreak + ")"
	}
	return `\z`
}

// open handles what starts with (.
func (t *jsTranslator) open() error {
	rest := t.rest()
	t.stack = append(t.stack, t.scope)
	if !strings.HasPrefix(rest, "(?") {
		t.pos++
		t.out.WriteString("(")
		return nil
	}
	for _, prefix := range []string{"(?:", "(?=", "(?!", "(?<=", "(?<!"} {
		if strings.HasPrefix(rest, prefix) {
			t.pos += len(prefix)
			t.out.WriteString(prefix)
			return nil
		}
	}
	if strings.HasPrefix(rest, "(?<") {
		t.pos += strings.IndexByte(rest, '>') + 1
		t.out.WriteString("(")
		return nil
	}
	return t.modifiers()
}

var jsModifiers = regexp.MustCompile(`^\(\?([ims]*)(?:-([ims]+))?:`)

// modifiers handles (?ims-ims:. Multi-line and dot are spelled out by this
// translation, leaving only case to regexp2.
func (t *jsTranslator) modifiers() error {
	m := jsModifiers.FindStringSubmatch(t.rest())
	if m == nil || m[1]+m[2] == "" {
		return errors.New("invalid group")
	}
	on, off := m[1], m[2]
	t.scope.multi = (t.scope.multi || strings.Contains(on, "m")) && !strings.Contains(off, "m")
	t.scope.dotAll = (t.scope.dotAll || strings.Contains(on, "s")) && !strings.Contains(off, "s")
	flags := ""
	if strings.Contains(on, "i") {
		flags = "i"
	}
	if strings.Contains(off, "i") {
		flags += "-i"
	}
	t.out.WriteString("(?" + flags + ":")
	t.pos += len(m[0])
	return nil
}

// reference writes a reference to a group, which in JavaScript matches nothing,
// successfully, if the group has taken no part. In regexp2 it would fail.
func (t *jsTranslator) reference(n int) {
	fmt.Fprintf(&t.out, `(?(%d)\k<%d>)`, n, n)
}

// escape handles what starts with a backslash, outside a bracket expression.
func (t *jsTranslator) escape() error {
	if t.pos+1 >= len(t.s) {
		return errors.New("pattern ends with a backslash")
	}
	c := t.s[t.pos+1]
	switch {
	case c == 'b' || c == 'B':
		t.pos += 2
		word := "[" + setBody(wordSet) + "]"
		at := "(?<=" + word + ")(?!" + word + ")|(?<!" + word + ")(?=" + word + ")"
		if c == 'B' {
			t.out.WriteString("(?!" + at + ")")
		} else {
			t.out.WriteString("(?:" + at + ")")
		}
		return nil
	case c >= '1' && c <= '9':
		start := t.pos + 1
		end := start
		for end < len(t.s) && t.s[end] >= '0' && t.s[end] <= '9' {
			end++
		}
		if n, err := strconv.Atoi(t.s[start:end]); err == nil && n < len(t.names) {
			t.pos = end
			t.reference(n)
			return nil
		}
		if t.unicode {
			return fmt.Errorf("reference to a group that does not exist: %s", t.s[start:end])
		}
	case c == 'k' && strings.HasPrefix(t.rest(), `\k<`) && (t.unicode || t.hasNames()):
		end := strings.IndexByte(t.rest(), '>')
		if end < 0 {
			return errors.New("group name is not closed")
		}
		name := t.rest()[3:end]
		n, ok := t.numbers[name]
		if !ok {
			return fmt.Errorf("reference to a group that does not exist: %s", name)
		}
		t.pos += end + 1
		t.reference(n)
		return nil
	}
	text, isSet, err := t.classEscape(false)
	if err != nil {
		return err
	}
	if isSet {
		text = "[" + text + "]"
	}
	t.out.WriteString(text)
	return nil
}

func (t *jsTranslator) hasNames() bool { return len(t.numbers) > 0 }

// classEscape writes the escape at the position, which means the same in and out
// of a bracket expression but for \b. If isSet, what is written is the inside of a
// bracket expression.
func (t *jsTranslator) classEscape(inClass bool) (text string, isSet bool, err error) {
	if t.pos+1 >= len(t.s) {
		return "", false, errors.New("pattern ends with a backslash")
	}
	c := t.s[t.pos+1]
	var set []runeRange
	switch c {
	case 'd', 'D':
		set = digitSet
	case 'w', 'W':
		set = wordSet
	case 's', 'S':
		set = spaceSet
	}
	if set != nil {
		t.pos += 2
		if c < 'a' {
			set = complementOf(set)
		}
		return setBody(set), true, nil
	}
	if (c == 'p' || c == 'P') && t.unicode {
		text, err := t.property()
		return text, false, err
	}
	r, err := t.character(inClass)
	if err != nil {
		return "", false, err
	}
	if inClass {
		return classRune(r), false, nil
	}
	return literalRune(r), false, nil
}

// character reads an escape that stands for one character.
func (t *jsTranslator) character(inClass bool) (rune, error) {
	c := t.s[t.pos+1]
	t.pos += 2
	simple := map[byte]rune{'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v'}
	if r, ok := simple[c]; ok {
		return r, nil
	}
	hex := func(length int) (rune, bool) {
		if len(t.rest()) < length {
			return 0, false
		}
		n, err := strconv.ParseUint(t.rest()[:length], 16, 32)
		if err != nil {
			return 0, false
		}
		t.pos += length
		return rune(n), true
	}
	switch {
	case c == 'b' && inClass:
		return '\b', nil
	case c == 'c' && t.pos < len(t.s) && isASCIILetter(t.s[t.pos]):
		t.pos++
		return rune(t.s[t.pos-1] % 32), nil
	case c == 'x':
		if r, ok := hex(2); ok {
			return r, nil
		}
	case c == 'u' && t.unicode && strings.HasPrefix(t.rest(), "{"):
		end := strings.IndexByte(t.rest(), '}')
		if end > 1 {
			if n, err := strconv.ParseUint(t.rest()[1:end], 16, 32); err == nil && n <= utf8.MaxRune {
				t.pos += end + 1
				return rune(n), nil
			}
		}
		return 0, errors.New(`invalid \u{...} escape`)
	case c == 'u':
		if r, ok := hex(4); ok {
			return r, nil
		}
	case c >= '0' && c <= '7':
		// A code in octal, which with the u flag can only be \0.
		value := int(c - '0')
		for i := 0; i < 2 && !t.unicode && t.pos < len(t.s) && t.s[t.pos] >= '0' && t.s[t.pos] <= '7' && value*8 < 256; i++ {
			value = value*8 + int(t.s[t.pos]-'0')
			t.pos++
		}
		return rune(value), nil
	}
	// Anything else is the character itself, which the u flag allows only for what
	// has a special meaning.
	t.pos--
	r, size := utf8.DecodeRuneInString(t.rest())
	t.pos += size
	if t.unicode && !strings.ContainsRune(`^$\.*+?()[]{}|/-`, r) {
		return 0, fmt.Errorf(`invalid escape \%c`, r)
	}
	return r, nil
}

func isASCIILetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// jsPropertyNames are the general categories by their long names.
var jsPropertyNames = map[string]string{
	"Letter": "L", "Lowercase_Letter": "Ll", "Uppercase_Letter": "Lu", "Titlecase_Letter": "Lt",
	"Modifier_Letter": "Lm", "Other_Letter": "Lo", "Mark": "M", "Number": "N",
	"Decimal_Number": "Nd", "Punctuation": "P", "Symbol": "S", "Separator": "Z", "Other": "C",
}

// property writes \p{...} in the names regexp2 knows.
func (t *jsTranslator) property() (string, error) {
	kind := t.s[t.pos+1]
	t.pos += 2
	if !strings.HasPrefix(t.rest(), "{") {
		return "", fmt.Errorf(`\%c needs a name in braces`, kind)
	}
	end := strings.IndexByte(t.rest(), '}')
	if end < 0 {
		return "", fmt.Errorf(`incomplete \%c`, kind)
	}
	name := t.rest()[1:end]
	t.pos += end + 1
	// As \p{Script=Greek} and \p{General_Category=Letter}.
	if _, value, ok := strings.Cut(name, "="); ok {
		name = value
	}
	if short, ok := jsPropertyNames[name]; ok {
		name = short
	}
	return `\` + string(kind) + "{" + name + "}", nil
}

// class writes a bracket expression. In JavaScript [] matches nothing and [^]
// anything, where elsewhere a ] first is a literal.
func (t *jsTranslator) class() error {
	switch {
	case strings.HasPrefix(t.rest(), "[]"):
		t.pos += 2
		t.out.WriteString("(?!)")
		return nil
	case strings.HasPrefix(t.rest(), "[^]"):
		t.pos += 3
		t.out.WriteString(`[\s\S]`)
		return nil
	}
	t.pos++
	t.out.WriteString("[")
	if strings.HasPrefix(t.rest(), "^") {
		t.pos++
		t.out.WriteString("^")
	}
	for t.pos < len(t.s) {
		switch t.s[t.pos] {
		case ']':
			t.pos++
			t.out.WriteString("]")
			return nil
		case '\\':
			text, _, err := t.classEscape(true)
			if err != nil {
				return err
			}
			t.out.WriteString(text)
		case '-':
			t.pos++
			t.out.WriteString("-")
		default:
			r, size := utf8.DecodeRuneInString(t.rest())
			t.pos += size
			t.out.WriteString(classRune(r))
		}
	}
	return errors.New("missing closing ]")
}
