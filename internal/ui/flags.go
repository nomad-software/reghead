package ui

import (
	"strings"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"github.com/nomad-software/reghead/internal/engine"
)

// fillFlagsMenu lists the flags of the current engine. It must not be called
// from one of the menu's own actions, which it deletes.
func (w *Window) fillFlagsMenu() {
	w.flagsMenu.Clear()
	for _, f := range w.allFlags() {
		a := w.flagsMenu.AddActionText(f.Label + " (" + f.Key + ")")
		a.SetCheckable(true)
		a.SetChecked(strings.Contains(w.config.Flags, f.Key))
		a.OnTriggered(func(on bool) {
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

// offeredFlags returns those of the flags that the engine has, in its own order.
func (w *Window) offeredFlags(flags string) string {
	kept := ""
	for _, f := range w.allFlags() {
		if strings.Contains(flags, f.Key) {
			kept += f.Key
		}
	}
	return kept
}

// flagsChanged shows the flags on the dropdown and in the regex field.
func (w *Window) flagsChanged() {
	shown := w.offeredFlags(w.config.Flags)
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
func (w *Window) makeInlineFlags(font *qtgui.QFont) {
	w.inlineFlags = qtwidgets.NewQLabel(w.regex)
	w.inlineFlags.SetFont(font)
	w.inlineFlags.SetTextFormat(qtcore.PlainText)
	w.inlineFlags.SetAttribute(qtcore.WA_TransparentForMouseEvents)
	w.regex.OverrideResizeEvent(func(super func(*qtgui.QResizeEvent), e *qtgui.QResizeEvent) {
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
		w.regex.setLeftMargin(0)
		return
	}
	w.inlineFlags.AdjustSize()
	w.inlineFlags.Move(inlineFlagsLeft, (w.regex.Height()-w.inlineFlags.Height())/2)
	w.inlineFlags.Show()
	w.regex.setLeftMargin(inlineFlagsLeft + w.inlineFlags.Width())
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
