package engine

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func eval(pattern, flags, text string) Result {
	return Evaluate(Request{Engine: ByName("Go"), Pattern: pattern, Flags: flags, Text: text})
}

func TestMatches(t *testing.T) {
	r := eval(`(?P<k>\w+)=(\d+)?`, "g", "a=1 b=")
	if r.Error != "" || len(r.Matches) != 2 {
		t.Fatalf("%+v", r)
	}
	if r.GroupNames[1] != "k" {
		t.Errorf("names %q", r.GroupNames)
	}
	if g := r.Matches[1].Groups[2]; g.Start != -1 {
		t.Errorf("unmatched group %+v", g)
	}
	if n := len(eval("a", "", "aaa").Matches); n != 1 {
		t.Errorf("without global: %d matches", n)
	}
	if n := len(eval("A", "gi", "aaa").Matches); n != 3 {
		t.Errorf("case insensitive: %d matches", n)
	}
	if n := len(eval("", "g", "aaa").Matches); n != 0 {
		t.Errorf("empty pattern: %d matches", n)
	}
}

func TestError(t *testing.T) {
	r := eval(`(a`, "gi", "a")
	if r.Error != "missing closing ): `(a`" {
		t.Errorf("error %q", r.Error)
	}
}

// The output must be what the standard library's own replacement gives,
// including around empty matches.
func TestSubstituteAgreesWithReplaceAll(t *testing.T) {
	for _, c := range []struct{ pattern, text, repl string }{
		{`a*`, "baaac", "[$0]"},
		{`x*`, "héllo", "-"},
		{`(\w+) (\w+)`, "one two, three four", "$2 ${1}!"},
		{`$`, "ab", "!"},
		{`\b`, "hi there", "|"},
		{`z`, "abc", "y"},
	} {
		r := Evaluate(Request{Engine: ByName("Go"), Pattern: c.pattern, Flags: "g", Text: c.text, Substitute: true, Replacement: c.repl})
		want := regexp.MustCompile(c.pattern).ReplaceAllString(c.text, c.repl)
		if r.Output != want {
			t.Errorf("%q on %q: got %q, want %q", c.pattern, c.text, r.Output, want)
		}
	}
}

func TestReplacedSpans(t *testing.T) {
	r := Evaluate(Request{Engine: ByName("Go"), Pattern: "b", Flags: "g", Text: "abc", Substitute: true, Replacement: "XY"})
	if r.Output != "aXYc" || r.Replaced[0] != (Span{1, 3}) {
		t.Errorf("%q %+v", r.Output, r.Replaced)
	}
}

func TestOffsets(t *testing.T) {
	text := "aé😀b" // 1, 2 and 4 bytes; 1, 1 and 2 UTF-16 units.
	o := NewOffsets(text)
	for _, c := range []struct{ b, runes, utf16 int }{{0, 0, 0}, {1, 1, 1}, {3, 2, 2}, {7, 3, 4}, {8, 4, 5}} {
		if o.Rune(c.b) != c.runes || o.UTF16(c.b) != c.utf16 {
			t.Errorf("byte %d: rune %d utf16 %d", c.b, o.Rune(c.b), o.UTF16(c.b))
		}
	}
}

func TestExplain(t *testing.T) {
	for _, c := range []struct{ pattern, flags, source, want string }{
		{`abc`, "", `abc`, `Literally "abc"`},
		{`abc`, "i", `abc`, `Literally "abc" (in any case)`},
		{`a\.b`, "", `a\.b`, `Literally "a.b"`},
		{`\d+`, "", `\d+`, "A digit (0-9), one or more times (as many as possible)"},
		{`\d+?`, "", `\d+?`, "A digit (0-9), one or more times (as few as possible)"},
		{`\d+`, "U", `\d+`, "A digit (0-9), one or more times (as few as possible)"},
		{`\d{4}`, "", `\d{4}`, "A digit (0-9), exactly 4 times"},
		{`x{2,}`, "", `x{2,}`, `Literally "x", at least 2 times (as many as possible)`},
		{`x{2,3}`, "", `x{2,3}`, `Literally "x", between 2 and 3 times (as many as possible)`},
		{`^`, "", `^`, "The start of the text"},
		{`^`, "m", `^`, "The start of a line"},
		{`.`, "", `.`, "Any character except a newline"},
		{`.`, "s", `.`, "Any character, including a newline"},
		{`[a-z_\d]`, "", `[a-z_\d]`, `One character from: "a" to "z", "_", a digit (0-9)`},
		{`[^[:alpha:]]`, "", `[^[:alpha:]]`, "One character that is not: a letter"},
		{`[]a-]`, "", `[]a-]`, `One character from: "]", "a", "-"`},
		{`(a)`, "", `(a)`, "Capture group 1"},
		{`(?P<y>a)`, "", `(?P<y>a)`, `Capture group 1, named "y"`},
		{`(?:a)+`, "", `(?:a)+`, "A group that captures nothing, one or more times (as many as possible)"},
		{`(?i)`, "", `(?i)`, "Turn on case-insensitive matching for the rest of the pattern"},
		{`\x41`, "", `\x41`, `Literally "A"`},
		{`\pL`, "", `\pL`, "A Unicode letter"},
		{`\p{Lu}`, "", `\p{Lu}`, "A Unicode upper case letter"},
		{`\P{Lu}`, "", `\P{Lu}`, "Anything except a Unicode upper case letter"},
		{`\p{Greek}`, "", `\p{Greek}`, "A character of the Greek script"},
		{`a|b`, "", `a|b`, "Either of these 2 alternatives"},
		{`\n`, "", `\n`, "A newline"},
		{`a{`, "", `a{`, `Literally "a{"`},
	} {
		nodes := ByName("Go").Explain(c.pattern, c.flags)
		if c.flags != "" { // The flags come first, and have a test of their own.
			nodes = nodes[1:]
		}
		if len(nodes) != 1 || nodes[0].Source != c.source || nodes[0].Text != c.want {
			t.Errorf("%q: got %+v, want %q", c.pattern, nodes, c.want)
		}
	}
}

func TestExplainStructure(t *testing.T) {
	nodes := ByName("Go").Explain(`(a)(?:(b)|c|)`, "")
	alt := nodes[1].Children[0]
	if len(alt.Children) != 3 || alt.Children[0].Text != "Alternative 1: capture group 2" {
		t.Errorf("%+v", alt)
	}
	if alt.Children[2].Text != "Alternative 3: nothing (matches the empty string)" {
		t.Errorf("%+v", alt.Children[2])
	}
	// A flag set in a group ends with it.
	nodes = ByName("Go").Explain(`((?s).)(.)`, "")
	if got := nodes[1].Children[0].Text; got != "Any character except a newline" {
		t.Errorf("flag leaked out of its group: %q", got)
	}
}

// checkTokens has the tokens of every node spell out its source.
func checkTokens(t *testing.T, pattern string, nodes []Node) {
	t.Helper()
	for _, n := range nodes {
		var got strings.Builder
		for _, token := range n.Tokens {
			got.WriteString(token.Text)
		}
		if got.String() != n.Source {
			t.Errorf("%q: tokens of %q spell %q", pattern, n.Source, got.String())
		}
		checkTokens(t, pattern, n.Children)
	}
}

func TestTokens(t *testing.T) {
	nodes := ByName("Go").Explain(`(a\.[\d-x]+|b)*(?i)^`, "")
	if len(nodes) != 3 {
		t.Fatalf("%+v", nodes)
	}
	want := []Token{
		{"(", TokenGroup, 0}, {"a", TokenLiteral, 0}, {`\.`, TokenEscaped, 0},
		{"[", TokenClass, 0}, {`\d`, TokenClassMeta, 0}, {"-", TokenClass, 0}, {"x", TokenClass, 0}, {"]", TokenClass, 0},
		{"+", TokenMeta, 0}, {"|", TokenMeta, 0}, {"b", TokenLiteral, 0}, {")", TokenGroup, 0}, {"*", TokenMeta, 0},
	}
	if !slices.Equal(nodes[0].Tokens, want) {
		t.Errorf("got %+v", nodes[0].Tokens)
	}
	if k := nodes[1].Tokens[0].Kind; k != TokenModifier {
		t.Errorf("modifier is %v", k)
	}
	if d := ByName("Go").Explain(`((a))`, "")[0].Tokens[1].Depth; d != 1 {
		t.Errorf("depth %d", d)
	}
}

// A pattern is highlighted while it does not compile, and whatever its flags.
func TestResultTokens(t *testing.T) {
	r := eval(`(a\d`, "gi", "a1")
	if r.Error == "" {
		t.Fatal("no error")
	}
	want := []Token{{"(", TokenGroup, 0}, {"a", TokenLiteral, 0}, {`\d`, TokenMeta, 0}}
	if !slices.Equal(r.Tokens, want) {
		t.Errorf("got %+v", r.Tokens)
	}
	if len(eval("", "g", "a").Tokens) != 0 {
		t.Error("tokens without a pattern")
	}
}

func TestGroupRanges(t *testing.T) {
	r := eval(`x(a(b)+)|(?:c)(?P<n>d)`, "g", "")
	want := []Span{{0, 22}, {1, 8}, {3, 7}, {14, 22}}
	if !slices.Equal(r.GroupRanges, want) {
		t.Errorf("got %+v", r.GroupRanges)
	}
	// The only thing in an alternative is still known as its group.
	if r := eval(`(a)|b`, "g", ""); r.GroupRanges[1] != (Span{0, 3}) {
		t.Errorf("got %+v", r.GroupRanges)
	}
}

func TestByteAtUTF16(t *testing.T) {
	o := NewOffsets("aé😀b")
	for _, c := range []struct{ position, b int }{{0, 0}, {1, 1}, {2, 3}, {4, 7}, {5, 8}} {
		if got := o.ByteAtUTF16(c.position); got != c.b {
			t.Errorf("position %d: byte %d, want %d", c.position, got, c.b)
		}
	}
}

func TestReplacedGroups(t *testing.T) {
	r := Evaluate(Request{Engine: ByName("Go"), Pattern: `(?P<a>x)(y)?`, Flags: "g", Text: "xy x", Substitute: true, Replacement: "${a}-$2$$"})
	if r.Output != "x-y$ x-$" {
		t.Fatalf("output %q", r.Output)
	}
	want := []ReplacedGroup{{Span{0, 1}, 0, 1}, {Span{2, 3}, 0, 2}, {Span{5, 6}, 1, 1}}
	if !slices.Equal(r.ReplacedGroups, want) {
		t.Errorf("groups %+v", r.ReplacedGroups)
	}
	if !slices.Equal(r.Replaced, []Span{{0, 4}, {5, 8}}) {
		t.Errorf("replaced %+v", r.Replaced)
	}
}

func TestParseReplacement(t *testing.T) {
	var kinds []string
	for _, p := range ByName("Go").ParseReplacement("a$1b ${n}$$$-$", nil) {
		kinds = append(kinds, p.Source)
	}
	if want := []string{"a", "$1b", " ", "${n}", "$$", "$", "-", "$"}; !slices.Equal(kinds, want) {
		t.Errorf("got %q", kinds)
	}
	// The parts are known even if the pattern is not valid.
	r := Evaluate(Request{Engine: ByName("Go"), Pattern: "(", Substitute: true, Replacement: "$1"})
	if len(r.ReplacementParts) != 1 || !r.ReplacementParts[0].IsRef {
		t.Errorf("parts %+v", r.ReplacementParts)
	}
}

// A replacement must come out just as Go's own Expand has it.
func FuzzReplacement(f *testing.F) {
	for _, s := range []string{"$1", "${a}", "$a$", "$$", "$0", "$01", "${1}x", "$1x", "${", "${}", "$é", "$b", "$999999999", "$a_b", "a$", "${a", "$\xff"} {
		f.Add(s)
	}
	re := regexp.MustCompile(`(?P<a>x)(y)?(?P<b>z)?`)
	text := "xy xz"
	f.Fuzz(func(t *testing.T, template string) {
		var want []byte
		for _, loc := range re.FindAllStringSubmatchIndex(text, -1) {
			want = re.ExpandString(want, template, text, loc)
		}
		r := Evaluate(Request{Engine: ByName("Go"), Pattern: re.String(), Flags: "g", Text: text, Substitute: true, Replacement: template})
		var got strings.Builder
		for _, m := range r.Replaced {
			got.WriteString(r.Output[m.Start:m.End])
		}
		if got.String() != string(want) {
			t.Errorf("%q: got %q, want %q", template, got.String(), want)
		}
		var source strings.Builder
		for _, p := range r.ReplacementParts {
			source.WriteString(p.Source)
		}
		if source.String() != template {
			t.Errorf("%q: parts spell %q", template, source.String())
		}
	})
}

func TestInlineFlags(t *testing.T) {
	e := ByName("Go")
	if got := e.InlineFlags("gUi"); got != "(?iU)" {
		t.Errorf("prefix %q", got)
	}
	if got := e.InlineFlags("g"); got != "" {
		t.Errorf("prefix %q", got)
	}
	for _, c := range []struct{ in, flags, rest string }{
		{"(?i)a", "i", "a"}, {"(?im)(?s)a", "ims", "a"}, {"a(?i)b", "", "a(?i)b"},
		{"(?i:a)", "", "(?i:a)"}, {"(?-i)a", "", "(?-i)a"}, {"(?i-s)a", "", "(?i-s)a"},
	} {
		if flags, rest := e.SplitInlineFlags(c.in); flags != c.flags || rest != c.rest {
			t.Errorf("%q: %q %q", c.in, flags, rest)
		}
	}
	// The explanation starts with the flags that are on.
	nodes := e.Explain("a", "gi")
	if nodes[0].Source != "(?i)" || nodes[0].Text != "Turn on case-insensitive matching for the rest of the pattern" {
		t.Errorf("%+v", nodes[0])
	}
}
