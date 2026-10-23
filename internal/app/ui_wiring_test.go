package app

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The interface is HTML, CSS and JavaScript, and nothing compiles it.
//
// A menu entry whose action has no case does nothing at all, silently. A
// palette button whose kind no branch handles does nothing at all, silently.
// A handler written into the wrong file does nothing at all, silently. Every
// one of those has happened here, and every one of them reached the person
// using the program rather than being caught on the way.
//
// These are the checks a compiler would do if there were one.

func readUI(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("ui/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestEveryMenuActionIsHandled is the one that matters most: a menu item that
// does nothing is the worst kind of bug, because it looks like the program
// working.
func TestEveryMenuActionIsHandled(t *testing.T) {
	html := readUI(t, "index.html")
	js := readUI(t, "app.js")

	// What the markup asks for.
	wanted := map[string]bool{}
	for _, m := range regexp.MustCompile(
		`data-action="([a-z0-9-]+)"([^>]*)`).FindAllStringSubmatch(html, -1) {
		if strings.Contains(m[2], "disabled") {
			continue // deliberately not implemented yet, and it looks it
		}
		wanted[m[1]] = true
	}
	if len(wanted) < 20 {
		t.Fatalf("only found %d actions in the markup; the pattern is wrong", len(wanted))
	}

	// What the script answers.
	handled := map[string]bool{}
	for _, m := range regexp.MustCompile(
		`case "([a-z0-9-]+)":`).FindAllStringSubmatch(js, -1) {
		handled[m[1]] = true
	}

	var missing []string
	for a := range wanted {
		if !handled[a] {
			missing = append(missing, a)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("menu entries with no handler: %s", strings.Join(missing, ", "))
	}
}

// TestEveryPaletteKindIsHandled checks the other half of the same problem.
// The palette holds its items as data; paletteAction decides what each kind
// does. A kind nobody reads is a button that does nothing.
func TestEveryPaletteKindIsHandled(t *testing.T) {
	palette := readUI(t, "palette.js")
	js := readUI(t, "app.js")

	// The keys an item can carry, other than how it is drawn.
	kinds := map[string]bool{}
	for _, m := range regexp.MustCompile(
		`(?m)\b(tool|apply|measure|staff|ending|sign|jump|custom|needsRange)\s*:`).
		FindAllStringSubmatch(palette, -1) {
		kinds[m[1]] = true
	}
	// Only what paletteAction itself does counts. A mention somewhere else in
	// the file is not a branch, and a branch in palette.js is not reached at
	// all — which is exactly how the crescendo button came to do nothing.
	body := functionBody(js, "async function paletteAction")
	if body == "" {
		t.Fatal("could not find paletteAction in app.js")
	}
	for kind := range kinds {
		if kind == "custom" {
			continue // a section flag, not an action
		}
		// A branch, not a mention: "if (item.x", "switch (item.x)",
		// "item.x ===" and so on all read the value to decide something.
		branch := regexp.MustCompile(
			`(if\s*\(\s*|switch\s*\(\s*|&&\s*|\|\|\s*|!)item\.` + kind + `\b`)
		if !branch.MatchString(body) {
			t.Errorf("palette items carry %q but paletteAction never branches on item.%s",
				kind, kind)
		}
	}
}

// functionBody returns the text of a function, from its opening brace to the
// matching close. Crude, but it only has to handle this file.
func functionBody(src, decl string) string {
	i := strings.Index(src, decl)
	if i < 0 {
		return ""
	}
	open := strings.Index(src[i:], "{")
	if open < 0 {
		return ""
	}
	open += i
	depth := 0
	for j := open; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open : j+1]
			}
		}
	}
	return ""
}

// TestPaletteActionLivesInAppJS guards the file the branches have to be in.
func TestPaletteActionLivesInAppJS(t *testing.T) {
	if !strings.Contains(readUI(t, "app.js"), "function paletteAction") {
		t.Error("paletteAction is not in app.js, so its branches will not run")
	}
}

// TestMenusDoNotForceSubmenusOpen is the rule that took three attempts.
//
// A descendant selector for an open menu's panel also matches every submenu
// panel inside it, at a higher specificity than the rule that hides them. The
// result is every submenu open at once and no way to script around it.
func TestMenusDoNotForceSubmenusOpen(t *testing.T) {
	css := readUI(t, "style.css")
	for _, bad := range []string{
		".menu.open .menu-items",
		".menu:hover .menu-items",
	} {
		if strings.Contains(css, bad+" ") || strings.Contains(css, bad+"{") {
			t.Errorf("%q is a descendant selector: it opens every submenu too. "+
				"Use a child combinator.", bad)
		}
	}
	if !strings.Contains(css, ".menu.open > .menu-items") {
		t.Error("nothing opens a menu's own panel")
	}
	if !strings.Contains(css, ".submenu.open > .menu-items") {
		t.Error("nothing opens a submenu's panel")
	}
}

// TestPartCardRowsAreScoped guards the same class of mistake in the inspector,
// where a rule meant for a row also matched the card holding the rows and laid
// them out sideways, off the edge of the panel.
func TestPartCardRowsAreScoped(t *testing.T) {
	css := readUI(t, "style.css")
	if strings.Contains(css, "#part-info div {") {
		t.Error("#part-info div also matches the card itself; scope it to the rows")
	}
}

// TestEveryDialogExists catches a dialog opened by id that is not in the
// markup — showModal on null throws and the menu entry appears dead.
func TestEveryDialogExists(t *testing.T) {
	html := readUI(t, "index.html")
	js := readUI(t, "app.js")

	have := map[string]bool{}
	for _, m := range regexp.MustCompile(
		`<dialog id="([a-z0-9-]+)"`).FindAllStringSubmatch(html, -1) {
		have[m[1]] = true
	}
	for _, m := range regexp.MustCompile(
		`\$\("#([a-z0-9-]+)"\)\.showModal\(\)`).FindAllStringSubmatch(js, -1) {
		if !have[m[1]] {
			t.Errorf("app.js opens a dialog #%s that the markup does not have", m[1])
		}
	}
}

// The pointer shape over the score is decided in one place.
//
// Three generations of these rules had piled up — one for a mode that no
// longer exists, one setting a hand on every notehead at a specificity that
// beat the rest — and between them the score showed a hand while you were
// writing in it. The same shape of mistake as the submenus: more than one
// rule deciding one thing.
func TestOnePlaceDecidesThePointer(t *testing.T) {
	css := readUI(t, "style.css")

	// Rules that set a cursor on the score itself, ignoring the pan overlay
	// and the drag-in-progress state, which are gestures rather than modes.
	rule := regexp.MustCompile(`(?m)^([^{\n]*#canvas[^{\n]*)\{[^}]*cursor:`)
	var selectors []string
	for _, m := range rule.FindAllStringSubmatch(css, -1) {
		sel := strings.TrimSpace(m[1])
		if strings.Contains(sel, "panning") || strings.Contains(sel, "dragging") {
			continue
		}
		selectors = append(selectors, sel)
	}
	if len(selectors) == 0 {
		t.Fatal("no pointer rules found at all; the pattern is wrong")
	}

	// They must be consecutive: one block, not scattered through the file.
	first := strings.Index(css, selectors[0])
	last := strings.LastIndex(css, selectors[len(selectors)-1])
	if last-first > 700 {
		t.Errorf("the pointer rules are spread over %d characters of the file, "+
			"so more than one place decides it:\n  %s",
			last-first, strings.Join(selectors, "\n  "))
	}

	// And the class of a mode that no longer exists must not linger.
	if strings.Contains(css, "body.note-input") {
		t.Error("body.note-input is left over from the writing/selecting mode")
	}
}

// TestDrumPadsAreWired checks the pad grid end to end through the markup.
//
// The pads are the only key a drum part has: a drum staff's lines name
// instruments and nothing on the page says which, so a grid that silently
// fails to appear leaves somebody guessing which line is the snare. Every
// element the renderer writes into has to be in the page.
func TestDrumPadsAreWired(t *testing.T) {
	html := readUI(t, "index.html")
	js := readUI(t, "app.js")
	css := readUI(t, "style.css")

	for _, id := range []string{"drumpad", "drumpad-grid", "drumpad-kit", "drumpad-note"} {
		if !strings.Contains(html, `id="`+id+`"`) {
			t.Errorf("app.js writes into #%s, which is not in the page", id)
		}
		if !strings.Contains(js, `"#`+id+`"`) {
			t.Errorf("#%s is in the page and nothing in app.js uses it", id)
		}
	}

	// The pad's own classes have to be styled, or the grid renders as a
	// column of unreadable buttons — which is how it looked before the rules
	// below existed.
	for _, cls := range []string{".pad", ".pad.on", ".pad-head", ".pad-name", ".pad-key", ".pad-midi"} {
		if !strings.Contains(css, cls+" ") && !strings.Contains(css, cls+",") &&
			!strings.Contains(css, cls+"{") && !strings.Contains(css, cls+" {") {
			t.Errorf("the pads use %s and the stylesheet does not mention it", cls)
		}
	}

	if !strings.Contains(js, "changeDrumKit") {
		t.Error("the kit picker has no handler: the sizes could not be chosen between")
	}
}

// TestDrumLettersAreTestedBeforeTheEditingLetters guards the one thing that
// makes two sets of single-letter keys possible at once.
//
// The pad letters and the editing commands deliberately overlap — both want
// the keys under the left hand — and they are told apart by where the cursor
// is. That only works if the drum test runs first: put it after the switch
// that handles "d" for stem-down and the pads under the left hand never fire,
// which looks like the shortcuts on the pads being decoration.
func TestDrumLettersAreTestedBeforeTheEditingLetters(t *testing.T) {
	js := readUI(t, "app.js")

	drum := strings.Index(js, "drumPadForKey(e.key)")
	if drum < 0 {
		t.Fatal("nothing routes a key press to a drum pad")
	}
	stem := strings.Index(js, `case "d": action("stem-down")`)
	if stem < 0 {
		t.Fatal(`the "d" shortcut has moved; this guard needs updating`)
	}
	if drum > stem {
		t.Error("the editing letters are handled first, so a pad's letter never reaches the kit")
	}

	// And the test has to be scoped, or the pad letters would swallow the
	// editing commands everywhere in the score.
	window := js[drum-400 : drum]
	if !strings.Contains(window, "isDrumStaff") {
		t.Error("the pad letters are not scoped to a drum staff: they would take over the whole score")
	}
}

// TestTheCursorIsPlacedAgainstItsOwnTune guards the arithmetic that decides
// where the view scrolls to during playback.
//
// A book is engraved one tune at a time and the drawings are stacked, so a
// position out of the engraver is measured from the top of its own tune and
// not from the top of the page. Scrolling to it without adding the tune's own
// offset took the view to the top of the book whichever tune was playing,
// which looks like playback having lost its place rather than like a missing
// addition.
func TestTheCursorIsPlacedAgainstItsOwnTune(t *testing.T) {
	js := readUI(t, "app.js")

	body := functionBody(js, "function followCursor")
	if body == "" {
		t.Fatal("could not find followCursor in app.js")
	}
	if !strings.Contains(body, "tuneOffsetY") {
		t.Error("followCursor scrolls to a position without adding the tune's own offset")
	}
	if !strings.Contains(js, "function tuneOffsetY") {
		t.Error("tuneOffsetY is used and not defined")
	}
}

// TestPlayFollowsTheTuneOnScreen guards the other half of the same bug.
//
// With a whole book on screen the keyboard has no way of knowing which tune
// somebody means, so it has to mean the one they are looking at. Without
// this, scrolling to a tune and pressing space played the first one.
func TestPlayFollowsTheTuneOnScreen(t *testing.T) {
	js := readUI(t, "app.js")

	body := functionBody(js, "async function play(opts)")
	if body == "" {
		t.Fatal("could not find play in app.js")
	}
	if !strings.Contains(body, "visibleTune") {
		t.Error("play does not look at which tune is on screen: in a book it would play the first")
	}
	if !strings.Contains(js, "function visibleTune") {
		t.Error("visibleTune is used and not defined")
	}
	// A selection says which tune more precisely than the scroll position
	// does, so it has to win: clicking a note in one tune and scrolling to
	// another should still play the note that was clicked.
	if !strings.Contains(body, "hasSelection()") {
		t.Error("play does not let a selection override the tune on screen")
	}
}

// TestEveryElementTheScriptAsksForExists is the same class of bug one level
// down from the menus: $("#thing") on an id the markup does not have returns
// null, the listener is never attached and the control does nothing, silently.
// A renamed id or a typo in one of the two files is enough.
func TestEveryElementTheScriptAsksForExists(t *testing.T) {
	html := readUI(t, "index.html")
	js := readUI(t, "app.js")

	// Some elements are built by the script itself and are not in the page
	// until it runs, so its own markup counts as well as the document's.
	declared := map[string]bool{}
	for _, m := range regexp.MustCompile(`id="([A-Za-z0-9_-]+)"`).
		FindAllStringSubmatch(html+js, -1) {
		declared[m[1]] = true
	}

	asked := map[string]bool{}
	for _, m := range regexp.MustCompile(`\$\("#([A-Za-z0-9_-]+)"\)`).
		FindAllStringSubmatch(js, -1) {
		asked[m[1]] = true
	}
	if len(asked) < 20 {
		t.Fatalf("only found %d lookups in the script; the pattern is wrong", len(asked))
	}

	var missing []string
	for id := range asked {
		if !declared[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the script looks for elements nothing creates: %s",
			strings.Join(missing, ", "))
	}
}

// The playback cursor is a rect inside the drawing, and several things
// replace the drawing whole while the music is playing — zooming is the one
// that gets done by accident. When that happens the script is left holding an
// element that is no longer in the document, and the cursor disappears with
// no error anywhere: the clock runs on, the notes sound, and the line is
// simply gone.
//
// There is no way to check that from here other than structurally, which is
// the same bargain the rest of this file makes: nothing compiles the
// interface, so the checks are the ones a compiler would do.
func TestTheCursorSurvivesARedraw(t *testing.T) {
	js := readUI(t, "app.js")

	body := functionBody(js, "function moveCursor(")
	if body == "" {
		t.Fatal("moveCursor is gone; this test needs rewriting against whatever replaced it")
	}
	if !strings.Contains(body, "isConnected") {
		t.Error("moveCursor does not check that the cursor is still in the document, " +
			"so a redraw during playback loses it")
	}
	if !strings.Contains(body, "makeCursor()") {
		t.Error("moveCursor does not put the cursor back when it has gone")
	}

	// And the reason it can go: the drawing is replaced wholesale.
	if !strings.Contains(js, `$("#canvas").innerHTML`) {
		t.Skip("the canvas is no longer replaced wholesale; the hazard may be gone")
	}
}
