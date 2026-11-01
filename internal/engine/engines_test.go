package engine

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	pcreName = "PCRE2"
	jsName   = "JavaScript"
)

// matched returns the text of each match, with its groups after a colon.
func matched(t *testing.T, engineName, pattern, flags, text string) []string {
	t.Helper()
	r := Evaluate(Request{Engine: ByName(engineName), Pattern: pattern, Flags: flags, Text: text})
	if r.Error != "" {
		t.Errorf("%s %q: %s", engineName, pattern, r.Error)
		return nil
	}
	found := []string{}
	for _, m := range r.Matches {
		parts := []string{}
		for _, g := range m.Groups {
			if g.Start < 0 {
				parts = append(parts, "<none>")
			} else {
				parts = append(parts, text[g.Start:g.End])
			}
		}
		found = append(found, strings.Join(parts, ":"))
	}
	return found
}

type matchCase struct {
	pattern, flags, text string
	want                 []string
}

func runCases(t *testing.T, engineName string, cases []matchCase) {
	t.Helper()
	for _, c := range cases {
		if got := matched(t, engineName, c.pattern, c.flags, c.text); !slices.Equal(got, c.want) {
			t.Errorf("%s /%s/%s on %q: got %q, want %q", engineName, c.pattern, c.flags, c.text, got, c.want)
		}
	}
}

func TestPCREMatching(t *testing.T) {
	runCases(t, pcreName, []matchCase{
		{`a++b`, "g", "aaab", []string{"aaab"}},
		{`a++a`, "g", "aaa", []string{}},
		{`(?>a+)b`, "g", "aab", []string{"aab"}},
		{`(?<=x)y`, "g", "xy y", []string{"y"}},
		{`(a)\1`, "g", "aa", []string{"aa:a"}},
		{`(?<n>a)(b)`, "g", "ab", []string{"ab:a:b"}}, // Numbered in order.
		{`(?P<n>a)(?P=n)`, "g", "aa", []string{"aa:a"}},
		{`\((?:[^()]|(?R))*\)`, "g", "(a(b)c)", []string{"(a(b)c)"}},
		{`\w+`, "g", "é a", []string{"a"}},
		{`\w+`, "gu", "é a", []string{"é", "a"}},
		{`a b # note`, "gx", "ab", []string{"ab"}},
		{`a.*`, "gU", "aaa", []string{"a", "a", "a"}},
		{`^b`, "gm", "a\nb", []string{"b"}},
		{`a.b`, "gs", "a\nb", []string{"a\nb"}},
		{`A`, "gi", "a", []string{"a"}},
		{`a`, "", "aa", []string{"a"}},
		{`😀(.)`, "g", "x😀é", []string{"😀é:é"}},
		{`(a)|(b)`, "g", "b", []string{"b:<none>:b"}},
	})
	bad := Evaluate(Request{Engine: ByName(pcreName), Pattern: `(`})
	if !strings.Contains(bad.Error, "missing closing parenthesis at position 1") {
		t.Errorf("error %q", bad.Error)
	}
}

func TestJavaScriptMatching(t *testing.T) {
	runCases(t, jsName, []matchCase{
		{`(?<n>a)(b)`, "g", "ab", []string{"ab:a:b"}}, // Numbered in order.
		{`(a)\k<n>(?<n>b)`, "g", "ab", []string{"ab:a:b"}},
		{`(a)|\1b`, "g", "b", []string{"b:<none>"}}, // An unset reference matches nothing.
		{`\d+`, "g", "١2", []string{"2"}},           // ASCII digits only.
		{`\w+`, "g", "é a", []string{"a"}},
		{`\w+`, "gu", "é a", []string{"a"}},
		{`\s`, "g", " x", []string{" "}}, // Unicode spaces.
		{`\bé`, "g", "é", []string{}},    // é is not a word character.
		{`a$`, "g", "a\n", []string{}},   // No allowance for a final newline.
		{`a$`, "gm", "a\nb", []string{"a"}},
		{`^b`, "gm", "a b", []string{"b"}},
		{`.`, "g", " ", []string{}},
		{`.`, "gs", "\n", []string{"\n"}},
		{`[]`, "g", "a", []string{}},
		{`[^]`, "g", "\n", []string{"\n"}},
		{`[]a]`, "g", "]a", []string{}}, // [] never matches, whatever follows.
		{`a{,2}`, "g", "a{,2}", []string{"a{,2}"}},
		{`A`, "g", "A", []string{"A"}},
		{`\u{1F600}`, "gu", "😀", []string{"😀"}},
		{`\p{L}`, "g", "p{L}", []string{"p{L}"}}, // \p is only a p without u.
		{`\p{L}+`, "gu", "aλ1", []string{"aλ"}},
		{`\p{Script=Greek}+`, "gu", "aλ", []string{"λ"}},
		{`\p{Letter}`, "gu", "1a", []string{"a"}},
		{`a`, "gy", "aab", []string{"a", "a"}},
		{`b`, "gy", "ab", []string{}},
		{`(?<=x)y(?!z)`, "g", "xy", []string{"y"}},
		{`(?i:a)b`, "g", "Ab aB", []string{"Ab"}},
		{`😀(.)`, "g", "x😀é", []string{"😀é:é"}},
		{`[\d-]+`, "g", "1-2", []string{"1-2"}},
		{`[\b]`, "g", "\b", []string{"\b"}},
		{`[a-z-[aeiou]]`, "g", "a]", []string{"a]"}}, // A class then a ], not .NET class subtraction.
		{`A`, "gi", "a", []string{"a"}},
		{`a*`, "g", "baa", []string{"", "aa", ""}},
	})
	for _, bad := range []string{`(`, `)`, `(?P<n>a)`, `(?i)a`, `a++`, `*`, `[a`, `(?<n>a)(?<n>b)`, `\k<n>(?<m>a)`, `\`} {
		if r := Evaluate(Request{Engine: ByName(jsName), Pattern: bad}); r.Error == "" {
			t.Errorf("%q should not compile", bad)
		}
	}
	for _, bad := range []string{`\1`, `\a`, `\u{110000}`, `\p`} {
		if r := Evaluate(Request{Engine: ByName(jsName), Pattern: bad, Flags: "u"}); r.Error == "" {
			t.Errorf("%q should not compile with u", bad)
		}
	}
}

// A pattern that backtracks without end stops, and keeps what it found.
func TestJavaScriptTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("takes as long as the timeout")
	}
	r := Evaluate(Request{Engine: ByName(jsName), Pattern: `(a+)+$`, Flags: "g", Text: strings.Repeat("a", 60) + "!"})
	if r.Error != "" || !strings.Contains(r.Warning, "stopped") {
		t.Errorf("error %q, warning %q", r.Error, r.Warning)
	}
}

// Hitting PCRE2's limit is told apart from finding nothing, and keeps what it found.
func TestPCRELimit(t *testing.T) {
	pattern := `^(?:(a|a)*)*$`
	text := strings.Repeat("a", 40) + "!"
	for _, flags := range []string{"", "g"} {
		r := Evaluate(Request{Engine: ByName(pcreName), Pattern: pattern, Flags: flags, Text: text})
		if !strings.Contains(r.Warning, "match limit") {
			t.Errorf("flags %q: warning %q", flags, r.Warning)
		}
	}
	for _, flags := range []string{"", "g"} {
		r := Evaluate(Request{Engine: ByName(pcreName), Pattern: "z", Flags: flags, Text: "a"})
		if r.Warning != "" {
			t.Errorf("flags %q: warning %q with nothing wrong", flags, r.Warning)
		}
		r = Evaluate(Request{Engine: ByName(pcreName), Pattern: "a*", Flags: flags, Text: "baa"})
		if r.Warning != "" {
			t.Errorf("flags %q: warning %q after matches", flags, r.Warning)
		}
	}
}

func replaced(engineName, pattern, text, template string) string {
	r := Evaluate(Request{Engine: ByName(engineName), Pattern: pattern, Flags: "g", Text: text, Substitute: true, Replacement: template})
	return r.Output
}

func TestReplacements(t *testing.T) {
	for _, c := range []struct{ engine, pattern, text, template, want string }{
		{pcreName, `(a)(b)`, "ab", `$2\1`, "ba"},
		{pcreName, `(a)`, "a", `${1}1`, "a1"},
		{pcreName, `(a)`, "a", `$0-\0`, "a-a"},
		{pcreName, `(a)`, "a", `\\1`, `\1`},
		{pcreName, `(a)`, "a", `$9`, ""},
		{pcreName, `(?<n>a)`, "a", `${n}`, "${n}"}, // No references by name.
		{pcreName, `a`, "a", `$`, "$"},
		{pcreName, `a`, "a", `$$`, "$$"},
		{jsName, `(a)(b)`, "ab", `$2$1`, "ba"},
		{jsName, `(a)`, "a", `$0`, "$0"},
		{jsName, `(a)`, "a", `$01`, "a"},
		{jsName, `(a)`, "a", `$12`, "a2"}, // Two digits only if there are that many groups.
		{jsName, `(a)`, "a", `$2`, "$2"},
		{jsName, `b`, "abc", "$`|$'|$&|$$", "aa|c|b|$c"},
		{jsName, `(?<n>a)`, "a", `$<n>!`, "a!"},
		{jsName, `(?<n>a)`, "a", `$<x>!`, "!"},
		{jsName, `(a)`, "a", `$<n>`, "$<n>"}, // No names, so not a reference.
		{jsName, `a`, "a", `$`, "$"},
	} {
		if got := replaced(c.engine, c.pattern, c.text, c.template); got != c.want {
			t.Errorf("%s %q %q: got %q, want %q", c.engine, c.pattern, c.template, got, c.want)
		}
	}
	// What each part refers to is known, for highlighting and hovering.
	r := Evaluate(Request{Engine: ByName(jsName), Pattern: `(?<n>a)`, Flags: "g", Text: "a", Substitute: true, Replacement: "$<n>"})
	if got := r.ReplacementParts[0].GroupOf(r.GroupNames); got != 1 {
		t.Errorf("group %d", got)
	}
}

func TestFullPattern(t *testing.T) {
	for _, c := range []struct{ engine, pattern, flags, want string }{
		{"Go", `a`, "gi", `(?i)a`},
		{"Go", `a/b`, "g", `a/b`},
		{pcreName, `a/b`, "gxi", `/a\/b/ix`}, // No g in the written form.
		{pcreName, `a\/b[/]`, "", `/a\/b[/]/`},
		{jsName, `a/b`, "yig", `/a\/b/giy`},
	} {
		if got := ByName(c.engine).FullPattern(c.pattern, c.flags); got != c.want {
			t.Errorf("%s: got %q, want %q", c.engine, got, c.want)
		}
	}
}

func TestDialectExplanations(t *testing.T) {
	for _, c := range []struct{ engine, pattern, flags, want string }{
		{pcreName, `a++`, "", `Literally "a", one or more times (as many as possible, never giving any back)`},
		{pcreName, `a{,3}`, "", `Literally "a", at most 3 times (as many as possible)`},
		{pcreName, `(?=a)`, "", "Followed by this (lookahead), which is not part of the match"},
		{pcreName, `(?<!a)`, "", "Not preceded by this (negative lookbehind)"},
		{pcreName, `(?>a)`, "", "An atomic group, which never gives back what it matched"},
		{pcreName, `(?#x)`, "", "A comment, which is ignored"},
		{pcreName, `(?R)`, "", "The whole pattern again (recursion)"},
		{pcreName, `(?1)`, "", "The pattern of group 1 again"},
		{pcreName, `$`, "", "The end of the text, or before a newline at its end"},
		{pcreName, `\h`, "", "Horizontal whitespace"},
		{pcreName, `\K`, "", "Here, the match restarts: what came before is left out of it"},
		{pcreName, `\d`, "u", "A Unicode digit"},
		{pcreName, `\k<n>`, "", `The same text as the group named "n" matched`},
		{pcreName, `\1`, "", "The same text as capture group 1 matched"},
		{pcreName, `(?P=n)`, "", `The same text as the group named "n" matched`},
		{pcreName, `(?'n'a)`, "", `Capture group 1, named "n"`},
		{jsName, `.`, "", "Any character except a line break"},
		{jsName, `[]`, "", "Nothing: an empty class never matches"},
		{jsName, `[^]`, "", "Any character at all"},
		{jsName, `(?<=a)`, "", "Preceded by this (lookbehind), which is not part of the match"},
		{jsName, `\k<n>`, "", `The same text as the group named "n" matched`},
		{jsName, `A`, "", `Literally "A"`},
		{jsName, `\p`, "", `Literally "p"`},
		{jsName, `\p{L}`, "u", "A Unicode letter"},
		{jsName, `\A`, "", `Literally "A"`},
		{jsName, `(?<n>a)`, "", `Capture group 1, named "n"`},
	} {
		nodes := ByName(c.engine).Explain(c.pattern, c.flags)
		if len(nodes) != 1 || nodes[0].Text != c.want {
			t.Errorf("%s %q: got %+v, want %q", c.engine, c.pattern, nodes, c.want)
		}
	}
	// Whitespace and comments in extended mode are explained as ignored.
	nodes := ByName(pcreName).Explain("a # note\nb", "x")
	if len(nodes) != 4 || nodes[1].Text != "Ignored in extended mode" {
		t.Errorf("extended: %+v", nodes)
	}
	// Groups inside lookaround still number from the left.
	r := Evaluate(Request{Engine: ByName(pcreName), Pattern: `(?=(a))(b)`, Flags: "g"})
	if !slices.Equal(r.GroupRanges, []Span{{0, 10}, {3, 6}, {7, 10}}) {
		t.Errorf("ranges %+v", r.GroupRanges)
	}
}

func TestDialectTokens(t *testing.T) {
	nodes := ByName(pcreName).Explain(`(?=a)\1(?#c)`, "")
	var kinds []TokenKind
	for _, n := range nodes {
		for _, token := range n.Tokens {
			kinds = append(kinds, token.Kind)
		}
	}
	want := []TokenKind{TokenGroup, TokenLiteral, TokenGroup, TokenBackreference, TokenComment}
	if !slices.Equal(kinds, want) {
		t.Errorf("got %v", kinds)
	}
}

// FuzzEngines has every engine take any pattern without failing, explain it with
// tokens that spell it, and find what it can.
func FuzzEngines(f *testing.F) {
	for _, s := range []string{
		`^(?P<a>\d+)|[^\]a-z[:alpha:]]*?$`, `\Qa.b\E+\x{1F600}\pL\p{^Greek}`, `(?i-s:a(?m)b)c{2,3}?`, `[[:]`, `a|`, `()`, `(|)`, `\Q\E`, `[\d-a]`, `\101`, `(?P<`, `(?<a`,
		`(?<n>a)\k<n>`, `(?P<n>a)(?P=n)`, `(?'n'a)`, `a++`, `a{,2}`, `(?=a)(?!b)(?<=c)(?<!d)`, `(?>a)`, `(?|a)`, `(?#c)`, `(?#`,
		`(*ACCEPT)`, `(?R)`, `(?1)`, `(?-1)`, `(?&n)`, `(?P>n)`, `(?(1)a|b)`, `(?(`, `\g{1}`, `\g1`, `\g<1>`, `\k`, `\k<`, `\k'`, `\k{`,
		`[]`, `[^]`, `[]a]`, `[`, `[[:alpha:]]`, `[[:`, `\`, `\c`, `\cA`, `\u`, `\u{`, `\u{1F600}`, `A`, `\x`, `\xZ`, `\x{`, `\p`, `\p{`, `\p{L}`,
		`\p{Script=Greek}`, `\1`, `\99`, `\0`, `\012`, `(?i:a)`, `(?i`, `(?-i:a)`, `(?x) a # b`, "a # b\nc", `(a|b)*?`, `\Qa\E`, `\Q`, `$^.`,
	} {
		f.Add(s, "g")
		f.Add(s, "gixus")
	}
	f.Fuzz(func(t *testing.T, pattern, flags string) {
		for _, e := range All() {
			var spelled, sources strings.Builder
			for _, n := range e.Explain(pattern, flags) {
				if n.synthetic {
					continue
				}
				sources.WriteString(n.Source)
				for _, token := range n.Tokens {
					spelled.WriteString(token.Text)
				}
			}
			if sources.String() != pattern {
				t.Errorf("%s %q: steps spell %q", e.Name(), pattern, sources.String())
			}
			if spelled.String() != pattern {
				t.Errorf("%s %q: tokens spell %q", e.Name(), pattern, spelled.String())
			}
			checkTokens(t, pattern, e.Explain(pattern, flags))
			// PCRE's own text handling is Qt's, for valid UTF-8 as everything from
			// the window is.
			if !utf8.ValidString(pattern) {
				continue
			}
			r := Evaluate(Request{Engine: e, Pattern: pattern, Flags: flags, Text: "aé😀b\n", Substitute: true, Replacement: "$1"})
			for _, m := range r.Matches {
				for _, g := range m.Groups {
					if g.Start > g.End || g.End > len("aé😀b\n") {
						t.Errorf("%s %q: span %+v", e.Name(), pattern, g)
					}
				}
			}
		}
	})
}
