package ui

import (
	"strings"

	qt "github.com/mappu/miqt/qt6"

	"github.com/nomad-software/reghead/internal/engine"
)

// fillFlagsMenu lists the flags of the current engine. It must not be called
// from one of the menu's own actions, which it deletes.
func (w *Window) fillFlagsMenu() {
	w.flagsMenu.Clear()
	for _, f := range w.allFlags() {
		a := w.flagsMenu.AddActionWithText(f.Label + " (" + f.Key + ")")
		a.SetCheckable(true)
		a.SetChecked(strings.Contains(w.config.Flags, f.Key))
		a.OnTriggeredWithChecked(func(on bool) {
			w.config.Flags = strings.ReplaceAll(w.config.Flags, f.Key, "")
			if on {
				w.config.Flags += f.Key
			}
			w.flagsChanged()
			w.refresh()
		})
	}
	w.flagsChanged()
}

// allFlags returns the flags on offer: the global flag and the engine's own.
func (w *Window) allFlags() []engine.Flag {
	return append([]engine.Flag{{Key: engine.FlagGlobal, Label: "Global"}}, w.engine.Flags()...)
}

// flagsChanged shows the flags on the dropdown and in the regex field.
func (w *Window) flagsChanged() {
	shown := ""
	for _, f := range w.allFlags() {
		if strings.Contains(w.config.Flags, f.Key) {
			shown += f.Key
		}
	}
	// The spaces keep the text clear of the dropdown's arrow.
	text := "Flags"
	if shown != "" {
		text += " (" + shown + ")"
	}
	w.flagsButton.SetText(text + "   ")
	w.inlineFlags.SetText(w.engine.InlineFlags(w.config.Flags))
	w.placeInlineFlags()
}

// makeInlineFlags makes the label that sits inside the regex field, before the
// text. It is apart from the text so that no way of editing can reach it.
func (w *Window) makeInlineFlags(font *qt.QFont) {
	w.inlineFlags = qt.NewQLabel(w.regex.QWidget)
	w.inlineFlags.SetFont(font)
	w.inlineFlags.SetTextFormat(qt.PlainText)
	w.inlineFlags.SetAttribute(qt.WA_TransparentForMouseEvents)
	w.regex.OnResizeEvent(func(super func(*qt.QResizeEvent), e *qt.QResizeEvent) {
		super(e)
		w.placeInlineFlags()
	})
	// Copying the whole field gives the pattern as it runs, flags and all.
	w.regex.copyAll = w.fullPattern
}

// fullPattern returns the pattern with its flags, as it would be written in code.
func (w *Window) fullPattern() string {
	return w.engine.FullPattern(w.regex.Text(), w.config.Flags)
}

// placeInlineFlags fits the label in the field and starts the text after it.
func (w *Window) placeInlineFlags() {
	if w.inlineFlags.Text() == "" {
		w.inlineFlags.Hide()
		w.regex.SetViewportMargins(0, 0, 0, 0)
		return
	}
	w.inlineFlags.AdjustSize()
	w.inlineFlags.Move(inlineFlagsLeft, (w.regex.Height()-w.inlineFlags.Height())/2)
	w.inlineFlags.Show()
	w.regex.SetViewportMargins(inlineFlagsLeft+w.inlineFlags.Width(), 0, 0, 0)
}

// takeInlineFlags moves flag settings at the start of a pattern into the flags.
func takeInlineFlags(e engine.Engine, flags, pattern string) (string, string) {
	found, rest := e.SplitInlineFlags(pattern)
	for _, r := range found {
		if !strings.ContainsRune(flags, r) {
			flags += string(r)
		}
	}
	return flags, rest
}

// regexChanged moves flags typed or pasted at the start of the pattern to the
// dropdown, then evaluates.
func (w *Window) regexChanged() {
	text := w.regex.Text()
	w.copyButton.SetEnabled(text != "")
	flags, rest := takeInlineFlags(w.engine, w.config.Flags, text)
	if rest != text {
		position := w.regex.CursorPosition() - (len(text) - len(rest))
		w.config.Flags = flags
		w.fillFlagsMenu()
		// This comes back here, and that evaluates.
		w.regex.SetText(rest)
		w.regex.SetCursorPosition(max(position, 0))
		return
	}
	w.refresh()
}
