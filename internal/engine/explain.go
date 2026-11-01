package engine

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// dialect is what makes the syntax of one engine different from another's.
type dialect struct {
	// escapes are the backslash escapes that stand for something by a letter.
	escapes map[byte]string
	// The meaning of . with and without the dot flag, and of ^ and $ with and
	// without the multi-line flag.
	dot, dotAll, start, startLine, end, endLine string
	// flagNames says what each inline flag does.
	flagNames map[byte]string
	// backtracking has lookaround, references and atomic groups.
	backtracking bool
	// perl has what PCRE has beyond that, as possessive repeats and recursion.
	perl bool
	// js has what is JavaScript's alone, as [] matching nothing.
	js bool
	// quoting has \Q...\E, and posix has [[:alpha:]].
	quoting, posix bool
}

var goDialect = dialect{
	escapes: goEscapes,
	dot:     "Any character except a newline", dotAll: "Any character, including a newline",
	start: "The start of the text", startLine: "The start of a line",
	end: "The end of the text", endLine: "The end of a line",
	flagNames: goFlagNames,
	quoting:   true, posix: true,
}

var pcreDialect = dialect{
	escapes: pcreEscapes,
	dot:     "Any character except a newline", dotAll: "Any character, including a newline",
	start: "The start of the text", startLine: "The start of a line",
	end: "The end of the text, or before a newline at its end", endLine: "The end of a line",
	flagNames:    pcreFlagNames,
	backtracking: true, perl: true, quoting: true, posix: true,
}

var jsDialect = dialect{
	escapes: jsEscapes,
	dot:     "Any character except a line break", dotAll: "Any character, including a line break",
	start: "The start of the text", startLine: "The start of a line",
	end: "The end of the text", endLine: "The end of a line",
	flagNames:    goFlagNames,
	backtracking: true, js: true,
}

// explain walks the text of the pattern, rather than a tree parsed from it, as
// regexp/syntax's is simplified: a|b becomes [ab], \d becomes [0-9], and common
// prefixes of alternatives are moved out. The explanation follows what was typed.
// It need not report errors, only never fail on what it does not expect: patterns
// that do not compile come here too, for their tokens.
func explain(d *dialect, pattern, flags string) []Node {
	// The first pass counts the groups, which references can look forward to.
	first := newExplainer(d, pattern, flags)
	first.walk()
	p := newExplainer(d, pattern, flags)
	p.total = first.groups
	nodes := p.walk()
	slices.SortFunc(p.tokens, func(a, b span) int { return a.start - b.start })
	p.attachTokens(nodes)
	return nodes
}

func newExplainer(d *dialect, pattern, flags string) *explainer {
	return &explainer{d: d, s: pattern, state: explainState{
		fold:     strings.Contains(flags, "i"),
		multi:    strings.Contains(flags, "m"),
		dotAll:   strings.Contains(flags, "s"),
		ungreedy: strings.Contains(flags, "U"),
		extended: strings.Contains(flags, "x"),
		unicode:  strings.Contains(flags, "u"),
	}}
}

// walk explains the whole pattern.
func (p *explainer) walk() []Node {
	var nodes []Node
	for p.pos < len(p.s) {
		nodes = append(nodes, p.alternation()...)
		if p.pos < len(p.s) { // A ")" with nothing to close.
			start := p.pos
			n := p.literalAt()
			n.start, n.end = start, p.pos
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// span is a token before its text is cut out of the pattern.
type span struct {
	start, end, depth int
	kind              TokenKind
}

// emit records a token.
func (p *explainer) emit(start int, kind TokenKind) {
	p.emitDepth(start, kind, 0)
}

func (p *explainer) emitDepth(start int, kind TokenKind, depth int) {
	if p.pos > start {
		p.tokens = append(p.tokens, span{start, p.pos, depth, kind})
	}
}

// attachTokens gives each node the tokens of its part of the pattern. What no
// token covers is literal.
func (p *explainer) attachTokens(nodes []Node) {
	for i := range nodes {
		n := &nodes[i]
		at := n.start
		for _, t := range p.tokens {
			if t.end <= n.start || t.start >= n.end {
				continue
			}
			if t.start > at {
				n.Tokens = append(n.Tokens, Token{Text: p.s[at:t.start]})
			}
			n.Tokens = append(n.Tokens, Token{p.s[t.start:t.end], t.kind, t.depth})
			at = t.end
		}
		if at < n.end {
			n.Tokens = append(n.Tokens, Token{Text: p.s[at:n.end]})
		}
		p.attachTokens(n.Children)
	}
}

// explainState is the flags in force at a point of the pattern.
type explainState struct{ fold, multi, dotAll, ungreedy, extended, unicode bool }

type explainer struct {
	d      *dialect
	total  int
	s      string
	pos    int
	groups int
	depth  int
	state  explainState
	tokens []span
	// branchReset says the next alternation is the inside of a branch reset group.
	branchReset bool
	// number hands specialGroup's group number back to group.
	number int
}

func (p *explainer) more() bool { return p.pos < len(p.s) }

func (p *explainer) rest() string { return p.s[p.pos:] }

// alternation explains up to a ")" or the end.
func (p *explainer) alternation() []Node {
	start := p.pos
	var alts []Node
	var first []Node
	// In a branch reset, each alternative numbers its groups from the same place, and
	// what follows numbers on from the one that had the most.
	reset := p.branchReset
	p.branchReset = false
	base, most := p.groups, p.groups
	defer func() {
		if reset {
			p.groups = most
		}
	}()
	for i := 1; ; i++ {
		altStart := p.pos
		if reset {
			p.groups = base
		}
		seq := p.sequence()
		most = max(most, p.groups)
		if i == 1 {
			first = seq
		}
		alt := alternative(i, p.s[altStart:p.pos], seq)
		alt.start, alt.end = altStart, p.pos
		alts = append(alts, alt)
		if !p.more() || p.s[p.pos] != '|' {
			break
		}
		p.pos++
		p.emit(p.pos-1, TokenMeta)
	}
	if len(alts) == 1 {
		return first
	}
	return []Node{{
		Source:   p.s[start:p.pos],
		Text:     fmt.Sprintf("Either of these %d alternatives", len(alts)),
		Children: alts,
		start:    start,
		end:      p.pos,
	}}
}

func alternative(i int, source string, seq []Node) Node {
	n := Node{Source: source, Text: fmt.Sprintf("Alternative %d", i), Children: seq}
	switch len(seq) {
	case 0:
		n.Text += ": nothing (matches the empty string)"
	case 1:
		n.Text += ": " + lowerFirst(seq[0].Text)
		n.Children = seq[0].Children
		n.group = seq[0].group
	}
	return n
}

// lowerFirst lowers the capital a sentence starts with, but not a name or a
// quotation.
func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// sequence explains items in a row, up to a "|", a ")" that closes a group, or
// the end.
func (p *explainer) sequence() []Node {
	var nodes []Node
	for p.more() {
		c := p.s[p.pos]
		if c == '|' || c == ')' {
			break
		}
		start := p.pos
		if ignored, ok := p.ignored(); ok {
			nodes = append(nodes, ignored)
			continue
		}
		atoms := p.atom()
		p.quantify(&atoms[len(atoms)-1])
		if len(atoms) == 1 {
			atoms[0].start = start
		}
		atoms[len(atoms)-1].end = p.pos
		nodes = append(nodes, atoms...)
	}
	return joinLiterals(nodes)
}

// ignored explains what extended mode skips: whitespace and # comments.
func (p *explainer) ignored() (Node, bool) {
	if !p.state.extended {
		return Node{}, false
	}
	start := p.pos
	switch {
	case p.s[p.pos] == '#':
		end := strings.IndexByte(p.rest(), '\n')
		if end < 0 {
			end = len(p.rest()) - 1
		}
		p.pos += end + 1
		p.emit(start, TokenComment)
	case strings.IndexByte(" \t\n\r\f\v", p.s[p.pos]) >= 0:
		for p.more() && strings.IndexByte(" \t\n\r\f\v", p.s[p.pos]) >= 0 {
			p.pos++
		}
	default:
		return Node{}, false
	}
	return Node{Source: p.s[start:p.pos], Text: "Ignored in extended mode", start: start, end: p.pos}, true
}

// joinLiterals merges neighbouring literals, so that abc is one step.
func joinLiterals(nodes []Node) []Node {
	var out []Node
	for _, n := range nodes {
		if n.literal != "" && len(out) > 0 && out[len(out)-1].literal != "" {
			last := &out[len(out)-1]
			fold := strings.HasSuffix(last.Text, anyCase)
			last.Source += n.Source
			last.literal += n.literal
			last.Text = literalText(last.literal, fold)
			last.end = n.end
			continue
		}
		out = append(out, n)
	}
	return out
}

const anyCase = " (in any case)"

func literalText(s string, fold bool) string {
	text := "Literally " + quote(s)
	if fold && strings.ToLower(s) != strings.ToUpper(s) {
		text += anyCase
	}
	return text
}

// quote puts text in quotation marks, spelling out what cannot be seen.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if strconv.IsPrint(r) {
			b.WriteRune(r)
		} else {
			q := strconv.QuoteRune(r)
			b.WriteString(q[1 : len(q)-1])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (p *explainer) literal(source string, r rune) Node {
	return Node{
		Source:  source,
		Text:    literalText(string(r), p.state.fold),
		literal: string(r),
	}
}

// literalAt takes one character as itself.
func (p *explainer) literalAt() Node {
	r, size := utf8.DecodeRuneInString(p.rest())
	source := p.rest()[:size]
	p.pos += size
	return p.literal(source, r)
}

// atom explains one item, or several for a \Q...\E quotation.
func (p *explainer) atom() []Node {
	pick := func(on bool, yes, no string) string {
		if on {
			return yes
		}
		return no
	}
	switch p.s[p.pos] {
	case '(':
		return []Node{p.group()}
	case '[':
		return []Node{p.class()}
	case '.':
		p.pos++
		p.emit(p.pos-1, TokenMeta)
		return []Node{{Source: ".", Text: pick(p.state.dotAll, p.d.dotAll, p.d.dot)}}
	case '^':
		p.pos++
		p.emit(p.pos-1, TokenMeta)
		return []Node{{Source: "^", Text: pick(p.state.multi, p.d.startLine, p.d.start)}}
	case '$':
		p.pos++
		p.emit(p.pos-1, TokenMeta)
		return []Node{{Source: "$", Text: pick(p.state.multi, p.d.endLine, p.d.end)}}
	case '\\':
		if p.d.quoting && strings.HasPrefix(p.rest(), `\Q`) {
			return p.quotation()
		}
		start := p.pos
		if text, ok := p.reference(); ok {
			p.emit(start, TokenBackreference)
			return []Node{{Source: p.s[start:p.pos], Text: text}}
		}
		text, r, isLiteral := p.escape()
		source := p.s[start:p.pos]
		if isLiteral {
			p.emit(start, TokenEscaped)
			return []Node{p.literal(source, r)}
		}
		p.emit(start, TokenMeta)
		return []Node{{Source: source, Text: upperFirst(text)}}
	}
	return []Node{p.literalAt()}
}

var (
	refNumber  = regexp.MustCompile(`^\\([1-9][0-9]*)`)
	refNamed   = regexp.MustCompile(`^\\k(?:<([^>]*)>|'([^']*)'|\{([^}]*)\})`)
	refG       = regexp.MustCompile(`^\\g(?:\{([^}]*)\}|(-?[0-9]+))`)
	refGSub    = regexp.MustCompile(`^\\g(?:<([^>]*)>|'([^']*)')`)
	jsRefNamed = regexp.MustCompile(`^\\k<([^>]*)>`)
)

// reference explains a backslash reference to a group, if that is what is here.
func (p *explainer) reference() (string, bool) {
	if !p.d.backtracking {
		return "", false
	}
	rest := p.rest()
	if m := refNumber.FindStringSubmatch(rest); m != nil {
		n, err := strconv.Atoi(m[1])
		// More than one digit is an octal code unless there are that many groups, and
		// so in JavaScript is one digit.
		if err == nil && (n <= p.total || (len(m[1]) == 1 && !p.d.js)) {
			p.pos += len(m[0])
			return "The same text as capture group " + m[1] + " matched", true
		}
		return "", false
	}
	if p.d.js {
		if m := jsRefNamed.FindStringSubmatch(rest); m != nil {
			p.pos += len(m[0])
			return "The same text as the group named " + quote(m[1]) + " matched", true
		}
		return "", false
	}
	if m := refNamed.FindStringSubmatch(rest); m != nil {
		p.pos += len(m[0])
		return "The same text as the group named " + quote(m[1]+m[2]+m[3]) + " matched", true
	}
	if m := refG.FindStringSubmatch(rest); m != nil {
		p.pos += len(m[0])
		return "The same text as group " + m[1] + m[2] + " matched", true
	}
	if m := refGSub.FindStringSubmatch(rest); m != nil {
		p.pos += len(m[0])
		return "The pattern of group " + m[1] + m[2] + " again", true
	}
	return "", false
}

// quotation explains \Q...\E, in which each character stands for itself. A
// repeat after it applies to the last character alone, as Go has it.
func (p *explainer) quotation() []Node {
	begin := p.pos
	p.pos += 2
	p.emit(begin, TokenEscaped)
	body, _, closed := strings.Cut(p.rest(), `\E`)
	nodes := []Node{}
	for _, r := range body {
		_, size := utf8.DecodeRuneInString(p.rest())
		n := p.literal(p.rest()[:size], r)
		n.start = p.pos
		p.pos += size
		n.end = p.pos
		nodes = append(nodes, n)
	}
	suffix := ""
	if closed {
		p.pos += 2
		p.emit(p.pos-2, TokenEscaped)
		suffix = `\E`
	}
	if len(nodes) == 0 {
		return []Node{{Source: `\Q` + suffix, Text: "An empty quotation, which matches nothing extra"}}
	}
	nodes[0].Source = `\Q` + nodes[0].Source
	nodes[0].start = begin
	nodes[len(nodes)-1].Source += suffix
	return nodes
}

var (
	goRepeat   = regexp.MustCompile(`^\{(0|[1-9][0-9]*)(,(0|[1-9][0-9]*)?)?\}`)
	upToRepeat = regexp.MustCompile(`^\{,([0-9]+)\}`)
)

// quantify adds any repetition that follows to the node.
func (p *explainer) quantify(n *Node) {
	for p.more() {
		start := p.pos
		var text string
		exact := false
		switch c := p.s[p.pos]; {
		case c == '*':
			text = "zero or more times"
			p.pos++
		case c == '+':
			text = "one or more times"
			p.pos++
		case c == '?':
			text = "optional"
			p.pos++
		case c == '{' && p.d.perl && upToRepeat.MatchString(p.rest()):
			m := upToRepeat.FindStringSubmatch(p.rest())
			p.pos += len(m[0])
			text = "at most " + times(m[1])
		case c == '{':
			m := goRepeat.FindStringSubmatch(p.rest())
			if m == nil {
				return
			}
			p.pos += len(m[0])
			switch {
			case m[2] == "":
				text, exact = "exactly "+times(m[1]), true
			case m[3] == "":
				text = "at least " + times(m[1])
			default:
				text = "between " + m[1] + " and " + times(m[3])
			}
		default:
			return
		}
		lazy, possessive := false, false
		if p.more() && p.s[p.pos] == '?' {
			lazy = true
			p.pos++
		} else if p.more() && p.s[p.pos] == '+' && p.d.perl {
			possessive = true
			p.pos++
		}
		if possessive {
			text += " (as many as possible, never giving any back)"
		} else if !exact {
			if lazy != p.state.ungreedy {
				text += " (as few as possible)"
			} else {
				text += " (as many as possible)"
			}
		}
		p.emit(start, TokenMeta)
		n.literal = ""
		n.Source += p.s[start:p.pos]
		n.Text += ", " + text
	}
}

func times(n string) string {
	if n == "1" {
		return "1 time"
	}
	return n + " times"
}

var pcreFlagNames = map[byte]string{
	'i': "case-insensitive matching",
	'm': "multi-line mode (^ and $ match at each line)",
	's': "dot matching newlines",
	'x': "extended mode (whitespace and # comments are ignored)",
	'U': "ungreedy mode (greedy and lazy swap)",
}

var goFlagNames = map[byte]string{
	'i': "case-insensitive matching",
	'm': "multi-line mode (^ and $ match at each line)",
	's': "dot matching newlines",
	'U': "ungreedy mode (greedy and lazy swap)",
}

// group explains anything that starts with "(".
func (p *explainer) group() Node {
	start := p.pos
	saved := p.state
	p.pos++
	var text string
	number := 0
	p.number = 0

	if special, done := p.specialGroup(start, &text); done {
		return special
	}
	switch rest := p.rest(); {
	case text != "":
	case !strings.HasPrefix(rest, "?"):
		p.groups++
		number = p.groups
		text = fmt.Sprintf("Capture group %d", p.groups)
	case strings.HasPrefix(rest, "?P<") && !p.d.js, strings.HasPrefix(rest, "?<"):
		open := strings.IndexByte(rest, '<')
		end := strings.IndexByte(rest, '>')
		name := ""
		if end > open {
			name = rest[open+1 : end]
			p.pos += end + 1
		} else { // Not closed, so it does not compile.
			p.pos = len(p.s)
		}
		p.groups++
		number = p.groups
		text = fmt.Sprintf("Capture group %d, named %s", p.groups, quote(name))
	default:
		i := 1
		for i < len(rest) && rest[i] != ':' && rest[i] != ')' {
			i++
		}
		settings := p.applyFlags(rest[1:i])
		if i < len(rest) && rest[i] == ')' {
			p.pos += i + 1
			p.emit(start, TokenModifier)
			scope := "the rest of the pattern"
			if p.depth > 0 {
				scope = "the rest of this group"
			}
			return Node{Source: p.s[start:p.pos], Text: upperFirst(settings) + " for " + scope}
		}
		p.pos += min(i+1, len(rest))
		text = "A group that captures nothing"
		if settings != "" {
			text += ", with " + settings
		}
	}

	if p.number > 0 {
		number = p.number
		p.number = 0
	}
	p.emitDepth(start, TokenGroup, p.depth)
	p.depth++
	children := p.alternation()
	p.depth--
	if p.more() && p.s[p.pos] == ')' {
		p.pos++
		p.emitDepth(p.pos-1, TokenGroup, p.depth)
	}
	p.state = saved
	if len(children) == 0 {
		text += " (empty)"
	}
	return Node{Source: p.s[start:p.pos], Text: text, Children: children, group: number}
}

// groupForms are the groups a backtracking dialect has that do something to what is
// inside them, by what they start with after the (.
var groupForms = []struct{ prefix, text string }{
	{"?=", "Followed by this (lookahead), which is not part of the match"},
	{"?!", "Not followed by this (negative lookahead)"},
	{"?<=", "Preceded by this (lookbehind), which is not part of the match"},
	{"?<!", "Not preceded by this (negative lookbehind)"},
}

// perlGroupForms are those only PCRE has.
var perlGroupForms = []struct{ prefix, text string }{
	{"?>", "An atomic group, which never gives back what it matched"},
	{"?|", "A branch reset group, whose alternatives each number their groups from the same place"},
}

var perlRecursion = regexp.MustCompile(`^\?(?:R|[+-]?[0-9]+|&[^)]*|P>[^)]*)\)`)

// specialGroup deals with the groups a backtracking dialect adds. It either returns
// a whole node, with done, or sets the text of a group that is then read as usual,
// having moved past what starts it.
func (p *explainer) specialGroup(start int, text *string) (Node, bool) {
	if !p.d.backtracking {
		return Node{}, false
	}
	whole := func(text string, kind TokenKind) (Node, bool) {
		p.emit(start, kind)
		return Node{Source: p.s[start:p.pos], Text: text}, true
	}
	rest := p.rest()
	forms := groupForms
	if p.d.perl {
		forms = append(slices.Clone(forms), perlGroupForms...)
	}
	for _, form := range forms {
		if strings.HasPrefix(rest, form.prefix) {
			p.pos += len(form.prefix)
			*text = form.text
			p.branchReset = form.prefix == "?|"
			return Node{}, false
		}
	}
	if !p.d.perl {
		return Node{}, false
	}
	closeAt := strings.IndexByte(rest, ')')
	switch {
	case strings.HasPrefix(rest, "?#"), strings.HasPrefix(rest, "*"):
		if closeAt < 0 {
			closeAt = len(rest) - 1
		}
		p.pos += closeAt + 1
		if rest[0] == '*' {
			return whole("A backtracking control verb", TokenModifier)
		}
		return whole("A comment, which is ignored", TokenComment)
	case strings.HasPrefix(rest, "?P=") && closeAt > 0:
		p.pos += closeAt + 1
		return whole("The same text as the group named "+quote(rest[3:closeAt])+" matched", TokenBackreference)
	case perlRecursion.MatchString(rest):
		p.pos += closeAt + 1
		if rest[1] == 'R' {
			return whole("The whole pattern again (recursion)", TokenBackreference)
		}
		return whole("The pattern of group "+strings.TrimLeft(rest[1:closeAt], "&P>")+" again", TokenBackreference)
	case strings.HasPrefix(rest, "?'") && strings.IndexByte(rest[2:], '\'') >= 0:
		end := 2 + strings.IndexByte(rest[2:], '\'')
		p.groups++
		*text = fmt.Sprintf("Capture group %d, named %s", p.groups, quote(rest[2:end]))
		p.number = p.groups
		p.pos += end + 1
	case strings.HasPrefix(rest, "?(") && closeAt > 0 && !strings.HasPrefix(rest, "?(?"):
		*text = "A conditional on " + quote(rest[2:closeAt]) + ", with what to match if so or else"
		p.pos += closeAt + 1
	}
	return Node{}, false
}

// applyFlags changes the state by flags such as "i-s", and says what it did.
func (p *explainer) applyFlags(flags string) string {
	on := true
	var onNames, offNames []string
	for i := 0; i < len(flags); i++ {
		c := flags[i]
		if c == '-' {
			on = false
			continue
		}
		switch c {
		case 'i':
			p.state.fold = on
		case 'm':
			p.state.multi = on
		case 's':
			p.state.dotAll = on
		case 'U':
			p.state.ungreedy = on
		case 'x':
			p.state.extended = on
		}
		if name, ok := p.d.flagNames[c]; ok {
			if on {
				onNames = append(onNames, name)
			} else {
				offNames = append(offNames, name)
			}
		}
	}
	var parts []string
	if len(onNames) > 0 {
		parts = append(parts, "turn on "+strings.Join(onNames, " and "))
	}
	if len(offNames) > 0 {
		parts = append(parts, "turn off "+strings.Join(offNames, " and "))
	}
	return strings.Join(parts, ", ")
}

var goPosix = map[string]string{
	"alnum": "a letter or digit", "alpha": "a letter", "ascii": "an ASCII character",
	"blank": "a space or tab", "cntrl": "a control character", "digit": "a digit",
	"graph": "a visible character", "lower": "a lower case letter",
	"print": "a printable character", "punct": "a punctuation mark",
	"space": "whitespace", "upper": "an upper case letter",
	"word": "a word character", "xdigit": "a hexadecimal digit",
}

// class explains a bracket expression.
func (p *explainer) class() Node {
	start := p.pos
	if p.d.js {
		for prefix, text := range map[string]string{"[]": "Nothing: an empty class never matches", "[^]": "Any character at all"} {
			if strings.HasPrefix(p.rest(), prefix) {
				p.pos += len(prefix)
				p.emit(start, TokenClass)
				return Node{Source: prefix, Text: text}
			}
		}
	}
	p.pos++
	intro := "One character from: "
	if p.more() && p.s[p.pos] == '^' {
		intro = "One character that is not: "
		p.pos++
	}
	p.emit(start, TokenClass)
	var items []string
	for first := true; p.more() && (first || p.s[p.pos] != ']'); first = false {
		if p.d.posix && strings.HasPrefix(p.rest(), "[:") {
			name, _, closed := strings.Cut(p.rest()[2:], ":]")
			negated, isNegated := strings.CutPrefix(name, "^")
			// What is not a known name is plain characters, as in [[:].
			if text, known := goPosix[negated]; closed && known {
				p.pos += len(name) + 4
				p.emit(p.pos-len(name)-4, TokenClassMeta)
				if isNegated {
					text = "anything except " + text
				}
				items = append(items, text)
				continue
			}
		}
		lo := p.classItem()
		// A "-" before the closing bracket stands for itself.
		if p.pos+1 < len(p.s) && p.s[p.pos] == '-' && p.s[p.pos+1] != ']' {
			p.pos++
			p.emit(p.pos-1, TokenClass)
			lo += " to " + p.classItem()
		}
		items = append(items, lo)
	}
	if p.more() {
		p.pos++
		p.emit(p.pos-1, TokenClass)
	}
	return Node{Source: p.s[start:p.pos], Text: intro + strings.Join(items, ", ")}
}

// classItem explains one character or escape within a bracket expression.
func (p *explainer) classItem() string {
	start := p.pos
	if strings.HasPrefix(p.rest(), `\b`) && p.d.backtracking {
		p.pos += 2
		p.emit(start, TokenClassMeta)
		return "a backspace"
	}
	if p.s[p.pos] == '\\' {
		text, r, isLiteral := p.escape()
		p.emit(start, TokenClassMeta)
		if isLiteral {
			return quote(string(r))
		}
		return text
	}
	r, size := utf8.DecodeRuneInString(p.rest())
	p.pos += size
	p.emit(start, TokenClass)
	return quote(string(r))
}

var goEscapes = map[byte]string{
	'd': "a digit (0-9)",
	'D': "anything except a digit",
	's': "whitespace (space, tab, newline, form feed or carriage return)",
	'S': "anything except whitespace",
	'w': "a word character (a-z, A-Z, 0-9 or _)",
	'W': "anything except a word character",
	'b': "a word boundary",
	'B': "anywhere but a word boundary",
	'A': "the start of the text",
	'z': "the end of the text",
	'a': "a bell character",
	'f': "a form feed",
	't': "a tab",
	'n': "a newline",
	'r': "a carriage return",
	'v': "a vertical tab",
}

// pcreEscapes and jsEscapes differ from goEscapes where those dialects do.
var pcreEscapes = extend(goEscapes, map[byte]string{
	's': "whitespace (space, tab, newline, vertical tab, form feed or carriage return)",
	'h': "horizontal whitespace",
	'H': "anything except horizontal whitespace",
	'v': "vertical whitespace",
	'V': "anything except vertical whitespace",
	'R': "any line break",
	'N': "anything except a newline",
	'K': "here, the match restarts: what came before is left out of it",
	'G': "where the last match ended",
	'Z': "the end of the text, or before a newline at its end",
	'e': "an escape character",
})

var jsEscapes = extend(goEscapes, map[byte]string{
	's': "whitespace (including Unicode's spaces and line breaks)",
	'A': "", 'z': "", 'a': "",
})

// pcreUnicodeEscapes are what the shorthands mean with the Unicode flag.
var pcreUnicodeEscapes = map[byte]string{
	'd': "a Unicode digit", 'D': "anything except a Unicode digit",
	'w': "a Unicode letter, digit or _", 'W': "anything except a Unicode letter, digit or _",
	's': "Unicode whitespace", 'S': "anything except Unicode whitespace",
}

// extend returns a map with more entries, those with no text being taken out.
func extend(base, more map[byte]string) map[byte]string {
	out := maps.Clone(base)
	for key, text := range more {
		out[key] = text
		if text == "" {
			delete(out, key)
		}
	}
	return out
}

// escape explains the escape at the position. It gives either a description or
// the literal character it stands for.
func (p *explainer) escape() (text string, r rune, isLiteral bool) {
	p.pos++
	if !p.more() {
		return "", '\\', true
	}
	c := p.s[p.pos]
	if p.d.perl && p.state.unicode {
		if text, ok := pcreUnicodeEscapes[c]; ok {
			p.pos++
			return text, 0, false
		}
	}
	if text, ok := p.d.escapes[c]; ok {
		p.pos++
		return text, 0, false
	}
	switch {
	case (c == 'p' || c == 'P') && (!p.d.js || p.state.unicode):
		p.pos++
		return p.unicodeClass(c == 'P'), 0, false
	case c == 'c' && p.d.backtracking && p.pos+1 < len(p.s):
		letter := p.s[p.pos+1]
		p.pos += 2
		return "the control character " + quote("^"+string(unicode.ToUpper(rune(letter)))), 0, false
	case c == 'x' && (!p.d.js || hexDigits(p.rest()[1:], 2)):
		p.pos++
		return "", p.hex(), true
	case c == 'u' && p.d.js && hexDigits(p.rest()[1:], 4):
		n, _ := strconv.ParseInt(p.s[p.pos+1:p.pos+5], 16, 32)
		p.pos += 5
		return "", rune(n), true
	case c == 'u' && p.d.js && p.state.unicode && strings.HasPrefix(p.rest()[1:], "{"):
		p.pos++
		return "", p.hex(), true
	case c >= '0' && c <= '7':
		n := 0
		for i := 0; i < 3 && p.more() && p.s[p.pos] >= '0' && p.s[p.pos] <= '7'; i++ {
			n = n*8 + int(p.s[p.pos]-'0')
			p.pos++
		}
		return "", rune(n), true
	}
	r, size := utf8.DecodeRuneInString(p.rest())
	p.pos += size
	return "", r, true
}

// hexDigits says if text starts with so many hexadecimal digits.
func hexDigits(text string, count int) bool {
	if len(text) < count {
		return false
	}
	_, err := strconv.ParseUint(text[:count], 16, 32)
	return err == nil
}

// hex reads the digits of \xHH or \x{H...}.
func (p *explainer) hex() rune {
	digits := ""
	if p.more() && p.s[p.pos] == '{' {
		end := strings.IndexByte(p.rest(), '}')
		if end < 0 {
			p.pos = len(p.s)
			return 0
		}
		digits = p.rest()[1:end]
		p.pos += end + 1
	} else {
		end := min(p.pos+2, len(p.s))
		digits = p.s[p.pos:end]
		p.pos = end
	}
	n, _ := strconv.ParseInt(digits, 16, 32)
	return rune(n)
}

var goCategories = map[string]string{
	"L": "a letter", "Lu": "an upper case letter", "Ll": "a lower case letter",
	"Lt": "a title case letter", "Lm": "a modifier letter", "Lo": "a letter with no case",
	"M": "a combining mark", "N": "a number", "Nd": "a decimal digit",
	"P": "a punctuation mark", "S": "a symbol", "Sc": "a currency symbol",
	"Z": "a separator", "Zs": "a space separator", "C": "a control or other character",
}

// unicodeClass explains what follows \p or \P.
func (p *explainer) unicodeClass(negated bool) string {
	name := ""
	if p.more() && p.s[p.pos] == '{' {
		end := strings.IndexByte(p.rest(), '}')
		if end < 0 {
			p.pos = len(p.s)
			return "a Unicode character"
		}
		name = p.rest()[1:end]
		p.pos += end + 1
	} else if p.more() {
		_, size := utf8.DecodeRuneInString(p.rest())
		name = p.rest()[:size]
		p.pos += size
	}
	if rest, ok := strings.CutPrefix(name, "^"); ok {
		name, negated = rest, !negated
	}
	text, ok := goCategories[name]
	switch {
	case ok:
		text = strings.Replace(text, "a", "a Unicode", 1)
		if strings.HasPrefix(text, "an ") {
			text = "a Unicode" + text[2:]
		}
	case len(name) <= 2:
		text = "a character in Unicode category " + name
	default:
		text = "a character of the " + name + " script"
	}
	if negated {
		return "anything except " + text
	}
	return text
}
