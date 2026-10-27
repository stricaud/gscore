/*
 * quit.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/stricaud/gscore/internal/format"
	gscorefmt "github.com/stricaud/gscore/internal/format/gscore"
)

/*
 * Closing the window with work in hand.
 *
 * A page can refuse to be left — that is what beforeunload is for, and the
 * browser asks on the page's behalf. The native window cannot: it is a
 * WKWebView, a WebView2 or a WebKitGTK view with no browser chrome around
 * it, the close button belongs to the operating system, and the binding
 * gscore uses offers no hook into it and no panel for beforeunload to raise.
 * So nothing was asked, and the edits went with the window.
 *
 * What is left is the moment after. The window's event loop returns when the
 * window closes and the program is still running, so that is where the
 * question gets asked — by the system's own dialog, since there is no longer
 * a page to ask in. The window has gone by then, which is not how a save
 * prompt usually feels; it is a great deal better than the alternative.
 */

// OfferToSaveOnExit asks about unsaved work, and saves it if asked to.
//
// It never returns an error for the asking: a machine with no dialog to show,
// or somebody dismissing it, is not a failure of the program. Only a save
// that was asked for and did not happen is worth reporting.
func (a *App) OfferToSaveOnExit() error {
	a.mu.RLock()
	unsaved, name := a.unsaved, a.filename
	a.mu.RUnlock()
	if !unsaved {
		return nil
	}

	what := "this score"
	if name != "" {
		what = filepath.Base(name)
	}
	save, asked := confirmSave(what)
	if !asked {
		// Nowhere to ask. Saying so beats the silence that lost the work,
		// and the score is still in the file it came from.
		fmt.Fprintf(os.Stderr,
			"gscore: %s has unsaved changes and there is no way to ask about them here\n", what)
		return nil
	}
	if !save {
		return nil
	}

	path := name
	if path == "" || !strings.EqualFold(filepath.Ext(path), ".gscore") {
		// Nothing to write back to, or it came from a format that cannot
		// hold everything gscore knows. Ask where it should go.
		suggested := filepath.Join(ScoresDir(), defaultSaveName(name))
		chosen, _, err := saveDialog(suggested)
		if err != nil || chosen == "" {
			return err
		}
		path = chosen
	}
	return a.writeScore(path)
}

// defaultSaveName is what to call a score that has not been saved before.
func defaultSaveName(from string) string {
	if from == "" {
		return "Untitled.gscore"
	}
	base := filepath.Base(from)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ".gscore"
}

// writeScore puts the score on disk, the same way the save handler does and
// without going through HTTP to do it.
func (a *App) writeScore(path string) error {
	a.mu.RLock()
	score, symbols := a.score, a.symbols
	a.mu.RUnlock()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if strings.EqualFold(filepath.Ext(path), ".gscore") {
		var preview []byte
		if SaveScorePDF() {
			var buf bytes.Buffer
			// No request behind this one: the defaults are what a save from
			// the menu uses when nothing is asked for.
			if err := a.renderPDF(&buf, &http.Request{URL: &url.URL{}}); err == nil {
				preview = buf.Bytes()
			}
		}
		if err := gscorefmt.WriteWithPDF(f, score, symbols, preview); err != nil {
			return err
		}
	} else {
		exp, err := format.ExporterFor(path)
		if err != nil {
			return err
		}
		if err := exp.Export(f, score); err != nil {
			return err
		}
	}

	a.lock()
	a.filename = path
	a.unsaved = false
	a.mu.Unlock()
	a.remember(path)
	fmt.Printf("gscore: saved %s\n", path)
	return nil
}

// confirmSave asks whether to save, and says whether it managed to ask.
func confirmSave(what string) (save, asked bool) {
	prompt := fmt.Sprintf("Save changes to %s before closing?", what)
	switch runtime.GOOS {
	case "darwin":
		script := `tell application "System Events" to display dialog "` +
			strings.ReplaceAll(prompt, `"`, "") +
			`" buttons {"Don't Save", "Save"} default button "Save" with icon caution`
		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			// Cancelled, or no way to show it. Either way nothing was said.
			return false, false
		}
		return strings.Contains(string(out), "button returned:Save"), true

	case "linux":
		for _, tool := range []string{"zenity", "kdialog"} {
			p, err := exec.LookPath(tool)
			if err != nil {
				continue
			}
			var cmd *exec.Cmd
			if tool == "zenity" {
				cmd = exec.Command(p, "--question", "--text="+prompt,
					"--ok-label=Save", "--cancel-label=Don't Save")
			} else {
				cmd = exec.Command(p, "--yesno", prompt,
					"--yes-label", "Save", "--no-label", "Don't Save")
			}
			// Both answer yes with status 0 and no with status 1.
			return cmd.Run() == nil, true
		}
		return false, false

	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms;` +
			`[System.Windows.Forms.MessageBox]::Show('` + powershellQuote(prompt) +
			`','gscore','YesNo','Warning')`
		out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		if err != nil {
			return false, false
		}
		return strings.TrimSpace(string(out)) == "Yes", true
	}
	return false, false
}
