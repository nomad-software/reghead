# RegHead

A regular expression tester, similar to regex101, in Go with a Qt 6 UI through
miqt (`github.com/mappu/miqt/qt6`). The app is called RegHead; the binary, module
and folders are `reghead`. Keep the README up to date with any change users see.

## Commands

`make build`, `make run`, `make test` (race detector), `make vet`, `make fuzz`,
`make screenshot`. The UI tests run offscreen (`QT_QPA_PLATFORM=offscreen`), so
they need no display. Run `make fuzz` after changing an explainer, a translator or
how replacements parse. Its first run recompiles the Qt bindings with coverage, as
the engine package imports Qt, which takes minutes and is not a hang.

## Layout

- `main.go`: finds the settings file, starts Qt and the window.
- `internal/engine`: no widgets, and no need for a `QApplication`, so it runs off
  the UI thread. `engine.go` holds the `Engine` and `Regex` interfaces, the `engines`
  registry, `Evaluate` and `Offsets`. `goengine.go` is Go's regexp, `pcre.go` is the
  real PCRE2 through Qt's `QRegularExpression`, and `javascript.go` is JavaScript
  simulated on regexp2, with its translator. `explain.go` explains all three, each
  with its own `dialect`.
- `internal/settings`: the JSON settings file.
- `internal/desktop`: writes the Linux desktop entry and icon at every start, only if
  they have changed. `desktop.Name` must stay what `main.go` gives Qt, for Wayland.
- `internal/ui`: `window.go` (the window, evaluation, saving), `menus.go`, `flags.go`
  (the dropdown and inline flags), `panes.go` (splitters and their sizes),
  `matches.go` (the match list), `highlight.go` (all colouring and the explanation's
  HTML), `hover.go`, `linefield.go`, `syntax.go` (highlighting colours),
  `settingsdialog.go`, `theme.go`, `about.go`.
- `internal/ui/examples.go`: the Examples menu's data, listed in a submenu per
  engine. Choosing one switches to its engine. A test runs each and checks `expected`
  matches. A new engine needs at least five, or a test fails.
- `packaging/` and `.github/workflows/`: the AppImage and macOS app. The workflows run
  on a `v*` tag, attaching to a release, or by hand, keeping an artifact. Each builds
  with Qt's plugins checked, and starts the result to prove it runs.
- `internal/appicon`: the embedded icon, `appicon.png`, which is hand-made artwork.
  Do not regenerate or replace it. After it changes, run `make icons` for the macOS one.

## Things to keep true

- **Dependencies:** no language runtimes or dependencies besides Go, by decision.
  Qt is already one, so PCRE2 uses Qt's real engine. Other languages are simulated
  in Go, and must say so in the README with how they differ.
- **Adding an engine:** implement `engine.Engine` and add it to `engines`. The
  Engine menu, flags dropdown, replacement hint and Copy are built from the interface.
  Give it a `dialect` for `explain`. Embed `flagsApart` if its language writes flags
  after the pattern.
- **Engines must not run for ever:** a backtracking one stops itself and returns what
  it found with an error from `FindAll`, which shows as `Result.Warning`. regexp2 has
  `jsTimeout`. PCRE2 has its match limit, which Qt reports as no match, so `FindAll`
  checks for it with one more `Match`.
- **The JavaScript translator** keeps regexp2 from meaning something else: regexp2
  follows .NET, whose shorthands cover Unicode, whose `[a-[b]]` subtracts, and which
  numbers named groups last. So shorthands are spelled out as sets and every group
  becomes numbered, with the names kept in `jsRegex`. What it cannot express is an
  error, never a silent change of meaning.
- **Flags are only changed through the dropdown.** `InlineFlags` gives what an
  engine shows for them in the regex field (Go: `(?im)`; return "" for an engine
  whose flags stand apart). It is a label inside the field, not its text, so
  nothing can edit it. `config.Pattern` and the field never hold it:
  `SplitInlineFlags` moves leading flags typed, pasted or loaded to the dropdown.
  Flags later in a pattern, or scoped or negative ones, are regex and stay.
- **Colours:** `syntax.go` holds regex101's syntax colours (match fills and
  tokens) and nothing else from that site. Never use its UI theme colours; the
  app's palettes are in `theme.go`. The dot of a row in the match list and its
  highlight in the text both come from `syntax.fill`. Theme changes redraw through `render`.
- **Tokens:** explainer tokens are recorded with `emit` as the pattern is walked.
  Every node's tokens must spell its `Source`; `FuzzEngines` checks it. `Evaluate`
  also runs `Explain` on patterns that do not compile, for `Result.Tokens`, which
  highlight the regex field as it is typed, so it must cope with any string.
- **Hover** (`hover.go`) lights up one `hoverTarget` in the pattern, replacement,
  text, result and list, in the group's `syntax.edge` colour. A group's place in the pattern comes from
  `Result.GroupRanges`, which `Evaluate` collects from each `Node.group`. The static
  regex colours are by token type, as on regex101, so only hover links them to
  match colours.
- **The regex and replacement fields** (`linefield.go`) are `QPlainTextEdit`s made
  single-line, as a `QLineEdit` cannot colour parts of its text. Colours are extra selections, which
  change neither the text nor the undo history.
- **Offsets:** engines report byte offsets. Qt positions are UTF-16 units and the
  match list shows characters, so always convert through `engine.Offsets`.
- **Evaluation is off the UI thread:** `Window.refresh` runs one `Evaluate` at a
  time in a goroutine, and returns to the UI with `mainthread.Start`. Changes made
  meanwhile set `pending`, the stale result is dropped and a new run starts. A
  backtracking engine can take seconds. Nothing paints from a result while `running`,
  as its positions may not fit the text. `Evaluate` recovers from panics, as one there
  would end the program. Tests wait with `settle`.
- **Explanations walk the pattern text**, not a parsed tree, which for Go would
  simplify what was typed. The pattern need not compile. `explain` must never panic,
  and its top level `Source` values must join up to the whole pattern; `FuzzEngines`
  checks both for every engine, on invalid patterns too.
- **Substitution** is built by `Result.substitute` from `Engine.ParseReplacement`,
  to know where each replacement and group falls. That must behave just as the
  language's own does; for Go, `FuzzReplacement` checks it against `Expand` and a test
  against `ReplaceAllString`. For PCRE2 and JavaScript, `TestReplacements` holds their
  documented rules, as neither is available to compare with.
- **Settings** are plain JSON and are not encrypted, by decision. `Load` starts
  from `Default()`, so a missing field keeps its default while a saved empty
  pattern or flags stays empty; do not add `omitempty` to those fields. They are
  saved a second after anything entered changes (`autosave`), when the window closes,
  and on SIGINT, SIGTERM or SIGHUP (`main.go`). Hidden panes report a size of 0, so
  `TextHeight` is only read while the substitution pane shows.

## miqt ownership

Crashes (`free(): invalid pointer`) come from getting this wrong:

- Delete only what you made with a `New...` constructor.
- Never delete a value a method returned. `Font()`, `Palette()` and
  `ColorWithCr()` are references Qt owns. `TextCursor()`, `Grab()` and
  `QFontDatabase_SystemFont()` carry a Go finalizer. Check the `gen_*.go` source
  if unsure, and use `runtime.KeepAlive` for a finalized value still in use.
- Virtual methods (`On...Event`) can only be overridden on widgets constructed
  from Go, or miqt panics. Splitter handles are made through `OnCreateHandle` for
  this reason (`newPaneSplitter`), which is what lets them paint only grip dots.
- Touch widgets only on the main thread; tests use `mainthread.Wait`.

miqt is pinned to a commit of its master branch, not the last tag (v0.15.0), for
its recent bug fixes. A new version recompiles all the bindings, which is slow.
