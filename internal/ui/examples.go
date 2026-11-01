package ui

import "github.com/nomad-software/reghead/internal/settings"

// The names of the engines the examples are for, as the engine package has them.
const (
	goEngineName   = "Go"
	pcreEngineName = "PCRE2"
	jsEngineName   = "JavaScript"
)

// example is what the Examples menu puts in the window.
type example struct {
	// engine is the name of the engine it is for, under which the menu lists it, and
	// which choosing it switches to, as syntax differs between them.
	engine                     string
	name, pattern, flags, text string
	// replacement makes it an example of substitution; "" is one of matching.
	replacement string
	// expected is how many matches it should find, which a test checks.
	expected int
}

var examples = []example{
	{
		engine:   goEngineName,
		name:     "Email Addresses",
		pattern:  `[\w.+-]+@[\w-]+(?:\.[\w-]+)+`,
		flags:    "g",
		text:     "Write to alice@example.com or bob.smith+news@mail.example.org.\nThe address nobody@ is not complete.\n",
		expected: 2,
	},
	{
		engine:   goEngineName,
		name:     "Log Lines (Multi-line)",
		pattern:  `^(?P<time>\S+) \[(?P<level>ERROR|WARN)\] (?P<message>.*)$`,
		flags:    "gm",
		text:     "09:14:02 [INFO] Server started\n09:14:07 [WARN] Disk nearly full\n09:15:31 [INFO] Request served\n09:16:44 [ERROR] Connection lost\n",
		expected: 2,
	},
	{
		engine:   goEngineName,
		name:     "URL Parts (Optional Groups)",
		pattern:  `(https?)://([^/\s:]+)(?::(\d+))?(/\S*)?`,
		flags:    "gi",
		text:     "See https://example.com/docs/intro and HTTP://localhost:8080 for more.\n",
		expected: 2,
	},
	{
		engine:   goEngineName,
		name:     "Greedy and Lazy Repeats",
		pattern:  `<(\w+)>.*?</\w+>`,
		flags:    "g",
		text:     "<b>bold</b> and <i>italic</i>\n\nTurn off the ? in the pattern, or tick Ungreedy, to see the difference.\n",
		expected: 2,
	},
	{
		engine:   goEngineName,
		name:     "Across Lines (Dot Matches Newline)",
		pattern:  `begin.*?end`,
		flags:    "gis",
		text:     "BEGIN\n  the first block\nEND\n\nbegin the second block end\n",
		expected: 2,
	},
	{
		engine:   goEngineName,
		name:     "Unicode Scripts",
		pattern:  `\p{Greek}+|\p{Han}+`,
		flags:    "g",
		text:     "English, Ελληνικά, 中文 😀 and more Ελληνικά.\n",
		expected: 3,
	},
	{
		engine:      goEngineName,
		name:        "Reformat Dates (Substitution)",
		pattern:     `(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`,
		flags:       "g",
		text:        "The project began on 2026-11-01.\nThe first release is due on 2027-01-15.\n",
		replacement: `${day}/${month}/${year}`,
		expected:    2,
	},
	{
		engine:      goEngineName,
		name:        "Swap Names (Substitution)",
		pattern:     `^(\w+), (\w+)$`,
		flags:       "gm",
		text:        "Lovelace, Ada\nTuring, Alan\nHopper, Grace\n",
		replacement: `$2 $1`,
		expected:    3,
	},
	{
		engine:      goEngineName,
		name:        "Mask Card Numbers (Substitution)",
		pattern:     `\b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?(\d{4})\b`,
		flags:       "g",
		text:        "Visa test card: 4111 1111 1111 1111\nMastercard test card: 5555-5555-5555-4444\n",
		replacement: `**** **** **** $1`,
		expected:    2,
	},
	{
		engine:   pcreEngineName,
		name:     "Password Rules (Lookahead)",
		pattern:  `^(?=.*\d)(?=.*[a-z])(?=.*[A-Z]).{8,}$`,
		flags:    "gm",
		text:     "correct horse\nCorrectHorse9\nabc1\nTr0ub4dor&3\n",
		expected: 2,
	},
	{
		engine:   pcreEngineName,
		name:     "Repeated Words (Backreference)",
		pattern:  `\b(\w++)\s+\1\b`,
		flags:    "gi",
		text:     "This is is a test of the the backreference.\n",
		expected: 2,
	},
	{
		engine:   pcreEngineName,
		name:     "Nested Brackets (Recursion)",
		pattern:  `\((?:[^()]++|(?R))*\)`,
		flags:    "g",
		text:     "call(a, (b + c), (d * (e - f))) and (g), but not (h\n",
		expected: 2,
	},
	{
		engine:   pcreEngineName,
		name:     "Quoted Strings (Possessive)",
		pattern:  `"[^"]*+"`,
		flags:    "g",
		text:     "He said \"hello\" and then \"goodbye\", then \"stopped\n",
		expected: 2,
	},
	{
		engine:   pcreEngineName,
		name:     "Keep Only the Number (\\K)",
		pattern:  `price:\s*\K\d+`,
		flags:    "gi",
		text:     "Price: 42\nprice:7\ncost: 9\n",
		expected: 2,
	},
	{
		engine:   pcreEngineName,
		name:     "Spaced Out Pattern (Extended)",
		pattern:  `(\d{3}) [-.\s]? (\d{4})`,
		flags:    "gx",
		text:     "Call 555-0199 or 555 0123, not 55-01.\n",
		expected: 2,
	},
	{
		engine:   pcreEngineName,
		name:     "Accented Words (Unicode)",
		pattern:  `\b\w+\b`,
		flags:    "gu",
		text:     "naïve café 中文\n\nTurn off Unicode to see the words break at the accents.\n",
		expected: 14,
	},
	{
		engine:      pcreEngineName,
		name:        "Reformat Dates (Substitution)",
		pattern:     `(\d{4})-(\d{2})-(\d{2})`,
		flags:       "g",
		text:        "The project began on 2026-11-01.\nThe first release is due on 2027-01-15.\n",
		replacement: `\3/\2/$1`,
		expected:    2,
	},
	{
		engine:      pcreEngineName,
		name:        "Key and Value (Named Groups, Substitution)",
		pattern:     `(?P<key>\w+)\s*=\s*(?P<value>\S+)`,
		flags:       "g",
		text:        "host = example.com\nport=8080\n",
		replacement: `${2}: $1`,
		expected:    2,
	},
	{
		engine:   jsEngineName,
		name:     "Quoted Strings (Backreference)",
		pattern:  `(['"]).*?\1`,
		flags:    "g",
		text:     "He said \"it's fine\" and 'see you', then \"stopped\n",
		expected: 2,
	},
	{
		engine:   jsEngineName,
		name:     "Tokens in a Row (Sticky)",
		pattern:  `\d+,?`,
		flags:    "gy",
		text:     "1,22,333 4,5\n\nTurn off Sticky to see the numbers after the space match as well.\n",
		expected: 3,
	},
	{
		engine:   jsEngineName,
		name:     "Greek Words (Unicode Property)",
		pattern:  `\p{Script=Greek}+`,
		flags:    "gu",
		text:     "English, Ελληνικά, and more Ελληνικά.\n\nTurn off Unicode, and \\p no longer means a property.\n",
		expected: 2,
	},
	{
		engine:   jsEngineName,
		name:     "Line Starts and Ends (Multi-line)",
		pattern:  `^\w+$`,
		flags:    "gm",
		text:     "one\ntwo words\nthree\n",
		expected: 2,
	},
	{
		engine:   jsEngineName,
		name:     "Any Character ([^])",
		pattern:  `/\*[^]*?\*/`,
		flags:    "g",
		text:     "code(); /* a comment\nacross lines */ more(); /* another */\n",
		expected: 2,
	},
	{
		engine:      jsEngineName,
		name:        "Wrap Prices (Lookbehind, $&)",
		pattern:     `(?<=\$)\d+(?:\.\d{2})?`,
		flags:       "g",
		text:        "Tea costs $3.50 and cake costs $4, or 7 euros.\n",
		replacement: `[$&]`,
		expected:    2,
	},
	{
		engine:      jsEngineName,
		name:        "Swap Names ($<name>)",
		pattern:     `(?<first>\w+) (?<last>\w+)`,
		flags:       "g",
		text:        "Ada Lovelace\nGrace Hopper\n",
		replacement: `$<last>, $<first>`,
		expected:    2,
	},
	{
		engine:      jsEngineName,
		name:        "Camel Case to Snake Case (Substitution)",
		pattern:     `([a-z])([A-Z])`,
		flags:       "g",
		text:        "parseHttpHeader and readFileToEnd\n",
		replacement: `$1_$2`,
		expected:    5,
	},
	{
		engine:      jsEngineName,
		name:        "Show the Context ($` and $')",
		pattern:     `middle`,
		flags:       "g",
		text:        "before middle after\n",
		replacement: "<$`|$&|$'>",
		expected:    1,
	},
}

// mode returns the mode that shows the example.
func (e example) mode() string {
	if e.replacement != "" {
		return settings.ModeSubstitute
	}
	return settings.ModeMatch
}
