/*
 * printnative_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"os"
	"strings"
	"testing"
)

// Printing from the Mac application.
//
// A web view has no print machinery: window.print() does nothing in there and
// window.open answers with nothing, so the page's own way of printing — write
// the pages into a second window and let it print itself — put nothing on the
// screen at all. The application prints instead, and the pages travel to it as
// a string through a message handler.
//
// Nothing compiles that path: a renamed message, a flag that stops being
// injected or a handler that stops matching the prefix all leave File ▸ Print
// doing nothing, silently, which is exactly what it did before this. These are
// the checks a compiler would do if there were one.

func macSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../ui/macosx/Sources/" + name)
	if err != nil {
		t.Skipf("the Mac sources are not here: %v", err)
	}
	return string(b)
}

func TestTheMacApplicationIsAskedToPrint(t *testing.T) {
	js := readUI(t, "app.js")

	body := js[strings.Index(js, "async function printScore()"):]
	if i := strings.Index(body, "\n}\n"); i > 0 {
		body = body[:i]
	}
	if body == "" {
		t.Fatal("printScore is not where it was")
	}

	// The page asks the application, and only where the application said it
	// can print. An older application than this page does not answer the
	// message at all, and silently printing nothing is the bug this fixes.
	if !strings.Contains(body, "window.gscoreNativePrint") {
		t.Error("printScore no longer asks whether the application can print")
	}
	if !strings.Contains(body, `postMessage("print:/api/pdf?what="`) {
		t.Error("printScore no longer sends the pages to the application")
	}
	// What was chosen in the dialog has to travel with them, or printing is
	// not what was asked for.
	for _, want := range []string{"multirest=1", "concert=1", "choice.what"} {
		if !strings.Contains(body, want) {
			t.Errorf("the print choice %q no longer reaches the application", want)
		}
	}
	// And the older way stays for an older application.
	if !strings.Contains(body, "/api/openbrowser?path=") {
		t.Error("the fallback to the system browser is gone")
	}
}

func TestTheMacApplicationAnswersTheRequestToPrint(t *testing.T) {
	app := macSource(t, "AppDelegate.swift")
	printing := macSource(t, "Printing.swift")

	// Told, before the page draws anything, that printing is handled here.
	if !strings.Contains(app, "window.gscoreNativePrint = true") {
		t.Error("the page is no longer told the application can print")
	}
	// And the message is matched and handed on.
	if !strings.Contains(app, `hasPrefix("print:")`) {
		t.Error("the application no longer answers the request to print")
	}
	if !strings.Contains(app, "Printing.show(") {
		t.Error("the request to print no longer reaches the printer")
	}

	// The pages are the PDF the exporter writes, shown in the system's own
	// panel. Each of these is load-bearing: without the operation there is
	// no panel, and without runModal it never reaches the screen.
	for _, want := range []string{"PDFDocument(data: data)", "printOperation(for: info",
		"showsPrintPanel = true", "op.runModal("} {
		if !strings.Contains(printing, want) {
			t.Errorf("Printing.swift no longer does %q", want)
		}
	}
	// The margins are in the pages the engraver cut. Taking the printer's
	// as well insets the music twice.
	if !strings.Contains(printing, "info.topMargin = 0") {
		t.Error("the printer adds its own margins to the engraved ones")
	}

	// And the framework that draws the pages has to be linked, or none of
	// the above gets as far as building.
	b, err := os.ReadFile("../../ui/macosx/build.sh")
	if err != nil {
		t.Skipf("the Mac build script is not here: %v", err)
	}
	if !strings.Contains(string(b), "-framework PDFKit") {
		t.Error("the Mac build no longer links PDFKit")
	}
}
