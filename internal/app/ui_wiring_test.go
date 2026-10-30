package app

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/smufl"
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

// TestTheKeyCatcherComesBeforeTheDialogGuard guards the one ordering that
// makes a key changeable at all.
//
// Keys are turned away while a dialog is open, because a note written behind
// one is a keystroke landing in two places. But the window that asks "press
// a key" is itself a dialog, so with that guard first nothing could ever be
// caught and every Change button did nothing.
func TestTheKeyCatcherComesBeforeTheDialogGuard(t *testing.T) {
	js := readUI(t, "app.js")
	start := strings.Index(js, "function setupKeys()")
	if start < 0 {
		t.Fatal("app.js has no setupKeys")
	}
	body := js[start:]
	catcher := strings.Index(body, "if (state.catchKey)")
	guard := strings.Index(body, `querySelector("dialog[open]")`)
	if catcher < 0 {
		t.Fatal("nothing catches a key for the shortcuts window")
	}
	if guard < 0 {
		t.Fatal("the open-dialog guard has gone; this test needs updating")
	}
	if catcher > guard {
		t.Error("the open-dialog guard runs first, so the shortcuts window " +
			"can never catch the key it is asking for")
	}
}

// TestDrumLettersAreTestedBeforeTheEditingLetters guards the one thing that
// makes two sets of single-letter keys possible at once.
//
// The pad letters and the editing commands deliberately overlap — both want
// the keys under the left hand — and they are told apart by where the cursor
// is. That only works if the drum test runs first: put it after the lookup
// that finds "d" bound to stem-down and the pads under the left hand never
// fire, which looks like the shortcuts on the pads being decoration.
func TestDrumLettersAreTestedBeforeTheEditingLetters(t *testing.T) {
	js := readUI(t, "app.js")

	drum := strings.Index(js, "drumPadForKey(e.key)")
	if drum < 0 {
		t.Fatal("nothing routes a key press to a drum pad")
	}
	// The call in the listener, not the function it reaches: bindingFor is
	// written further up the file, so looking for its name alone would find
	// the definition and compare the wrong two places.
	bound := strings.Index(js, "const bound = bindingFor(e)")
	if bound < 0 {
		t.Fatal("nothing looks a key press up in the bindings; this guard needs updating")
	}
	if drum > bound {
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

	// playNow, because play itself is now the wrapper that says what it is
	// doing while a symphony is got ready. The choosing is still one place.
	body := functionBody(js, "async function playNow(opts)")
	if body == "" {
		t.Fatal("could not find playNow in app.js")
	}
	if !strings.Contains(body, "visibleTune") {
		t.Error("playNow does not look at which tune is on screen: in a book it would play the first")
	}
	if !strings.Contains(js, "function visibleTune") {
		t.Error("visibleTune is used and not defined")
	}
	// A selection says which tune more precisely than the scroll position
	// does, so it has to win: clicking a note in one tune and scrolling to
	// another should still play the note that was clicked.
	if !strings.Contains(body, "hasSelection()") {
		t.Error("playNow does not let a selection override the tune on screen")
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

// TestEveryMenuIconIsTheGlyphItSaysItIs.
//
// The pictures in the menus are glyphs from the score's own font, written
// into the markup from a table so that a hundred and sixty entries are not
// kept in step by hand. The table gives each one as a codepoint with its
// canonical SMuFL name beside it — which is a promise, and this is the check
// that it is kept. A sharp beside the flat is the kind of mistake nobody
// notices and everybody trips over.
func TestEveryMenuIconIsTheGlyphItSaysItIs(t *testing.T) {
	js := readUI(t, "app.js")
	start := strings.Index(js, "const MENU_ICONS = {")
	if start < 0 {
		t.Fatal("there is no menu icon table")
	}
	end := strings.Index(js[start:], "\n};")
	if end < 0 {
		t.Fatal("the menu icon table does not end")
	}
	table := js[start : start+end]

	rows := regexp.MustCompile(
		`"([a-z0-9-]+)":\s*"\\u([0-9a-fA-F]{4})",\s*//\s*([A-Za-z0-9]+)`).
		FindAllStringSubmatch(table, -1)
	if len(rows) < 40 {
		t.Fatalf("only %d icons read from the table; it has more than that",
			len(rows))
	}

	html := readUI(t, "index.html")
	for _, row := range rows {
		action, cp, name := row[1], row[2], row[3]
		want, ok := smufl.Glyph(name)
		if !ok {
			t.Errorf("%s: the font has no glyph called %q", action, name)
			continue
		}
		got, err := strconv.ParseInt(cp, 16, 32)
		if err != nil {
			t.Errorf("%s: %q is not a codepoint", action, cp)
			continue
		}
		if rune(got) != want {
			t.Errorf("%s is \\u%s but %s is \\u%04x: the comment and the "+
				"character disagree", action, cp, name, want)
		}
		// And it is an icon for something that is actually in a menu.
		if !strings.Contains(html, `data-action="`+action+`"`) {
			t.Errorf("%s has an icon but is in no menu", action)
		}
	}
}

// Every glyph the palette shows is in the font too.
func TestEveryPaletteGlyphIsInTheFont(t *testing.T) {
	palette := readUI(t, "palette.js")
	found := 0
	// Only what is drawn as a glyph. A text item may hold any character —
	// "To\u00A0Coda" is two words that must not be split — and those are
	// set in the interface's own font, not the music one.
	for _, m := range regexp.MustCompile(`g:\s*"((?:[^"\\]|\\u[0-9a-fA-F]{4})+)"`).
		FindAllStringSubmatch(palette, -1) {
		for _, r := range unescape(m[1]) {
			// The private use area, which is where SMuFL lives. Anything
			// else in a glyph string is ordinary text.
			if r < 0xE000 || r > 0xF8FF {
				continue
			}
			found++
			if _, ok := smufl.ByRune(r); !ok {
				t.Errorf("the palette shows \\u%04x, which the font does not draw", r)
			}
		}
	}
	if found < 40 {
		t.Errorf("only %d glyphs found in the palette; this guard needs updating",
			found)
	}
}

// unescape reads a JavaScript string body, which may spell a character out.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+5 < len(s) && s[i+1] == 'u' {
			if cp, err := strconv.ParseInt(s[i+2:i+6], 16, 32); err == nil {
				b.WriteRune(rune(cp))
				i += 5
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
