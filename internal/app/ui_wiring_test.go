package app

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
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

	// One codepoint is a mark on its own; two are a note and the mark drawn
	// on it. Either way the comment names them in the same order.
	rows := regexp.MustCompile(
		`"([a-z0-9-]+)":\s*"((?:\\u[0-9a-fA-F]{4})+)",\s*//\s*([A-Za-z0-9 ]+)`).
		FindAllStringSubmatch(table, -1)
	if len(rows) < 60 {
		t.Fatalf("only %d icons read from the table; it has more than that",
			len(rows))
	}
	composites := 0

	html := readUI(t, "index.html")
	for _, row := range rows {
		action := row[1]
		chars := []rune(unescape(row[2]))
		names := strings.Fields(row[3])
		if len(chars) > 1 {
			composites++
		}
		if len(chars) != len(names) {
			t.Errorf("%s draws %d glyphs and its comment names %d of them",
				action, len(chars), len(names))
			continue
		}
		for i, name := range names {
			want, ok := smufl.Glyph(name)
			if !ok {
				t.Errorf("%s: the font has no glyph called %q", action, name)
				continue
			}
			if chars[i] != want {
				t.Errorf("%s draws \\u%04x where it says %s, which is \\u%04x",
					action, chars[i], name, want)
			}
		}
		// And it is an icon for something that is actually in a menu.
		if !strings.Contains(html, `data-action="`+action+`"`) {
			t.Errorf("%s has an icon but is in no menu", action)
		}
	}
	if composites < 5 {
		t.Errorf("only %d entries draw a mark on a note; the articulations "+
			"all should", composites)
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

// TestTheFingeringPanelReadsTheStaffTheWayTheEngraverDrewIt.
//
// The panel turns a staff position back into a pitch, which is the engraver's
// arithmetic run backwards. It keeps its own copy of the clef offsets,
// because it is JavaScript and the engraver is Go — so the two can drift,
// and a fingering panel that disagrees with the page about which note is
// being pointed at is worse than no panel.
func TestTheFingeringPanelReadsTheStaffTheWayTheEngraverDrewIt(t *testing.T) {
	js := readUI(t, "app.js")
	body := functionBody(js, "function midiForStep")
	if body == "" {
		t.Fatal("there is no midiForStep; this guard needs updating")
	}

	want := map[string]model.Clef{
		"treble": model.Treble, "bass": model.Bass, "alto": model.Alto,
		"tenor": model.Tenor, "percussion": model.Percussion,
	}
	rows := regexp.MustCompile(`(\w+):\s*(-?\d+)`).FindAllStringSubmatch(body, -1)
	if len(rows) != len(want) {
		t.Fatalf("%d clefs in the panel's table, want %d", len(rows), len(want))
	}
	for _, row := range rows {
		clef, ok := want[row[1]]
		if !ok {
			t.Errorf("the panel knows a clef called %q that the engraver does not",
				row[1])
			continue
		}
		got, err := strconv.Atoi(row[2])
		if err != nil {
			t.Errorf("%s: %q is not an offset", row[1], row[2])
			continue
		}
		// The engraver's own answer: where it puts middle C on that staff,
		// since a diatonic index of nought is middle C.
		at := engrave.StaffStep(clef, model.Pitch{Step: model.StepC, Octave: 4})
		if got != at {
			t.Errorf("the panel puts %s's middle C at step %d; the engraver "+
				"draws it at %d", row[1], got, at)
		}
	}
}

// TestTheChromeIsTranslated.
//
// Every word that sits on screen waiting to be read — the menus, the palette
// and the labels the shortcuts window is built from — has French against it.
// A missing one is not a crash: it comes back as English, which is the whole
// point of keying the dictionary by the English. But it is a word somebody
// reading the French interface meets in English, and nothing else would say
// so.
//
// What is deliberately left alone is listed here rather than guessed at: the
// name of the program, the dynamics and the Italian a score is marked up in,
// the note names and time signatures, the examples somebody is meant to copy,
// and the symbols on the transport buttons. Translating those would be
// translating the music.
func TestTheChromeIsTranslated(t *testing.T) {
	fr := frenchTable(t)

	english := map[string]string{} // the string, and where it came from
	add := func(where string, ss ...string) {
		for _, s := range ss {
			s = strings.TrimSpace(s)
			if s != "" {
				english[s] = where
			}
		}
	}

	html := readUI(t, "index.html")
	for _, m := range regexp.MustCompile(
		`<button class="menu-title">([^<]+)</button>`).FindAllStringSubmatch(html, -1) {
		add("a menu", m[1])
	}
	for _, m := range regexp.MustCompile(
		`<button class="submenu-title"><span>([^<]+)</span>`).
		FindAllStringSubmatch(html, -1) {
		add("a submenu", m[1])
	}
	for _, m := range regexp.MustCompile(
		`<button data-action="[^"]+"[^>]*><span[^>]*>([^<]+)</span>`).
		FindAllStringSubmatch(html, -1) {
		add("a menu entry", m[1])
	}

	palette := readUI(t, "palette.js")
	for _, m := range regexp.MustCompile(`label:\s*"((?:[^"\\]|\\.)*)"`).
		FindAllStringSubmatch(palette, -1) {
		add("a palette button", unescape(m[1]))
	}
	for _, m := range regexp.MustCompile(`name:\s*"([^"]+)"`).
		FindAllStringSubmatch(palette, -1) {
		add("a palette section", m[1])
	}

	js := readUI(t, "app.js")
	table := js[strings.Index(js, "const KEY_BINDINGS"):]
	for _, m := range regexp.MustCompile(`label:\s*"((?:[^"\\]|\\.)*)"`).
		FindAllStringSubmatch(table, -1) {
		add("a keyboard shortcut", unescape(m[1]))
	}
	for _, m := range regexp.MustCompile(`group:\s*"([^"]+)"`).
		FindAllStringSubmatch(table, -1) {
		add("a shortcut heading", m[1])
	}

	var missing []string
	for s, where := range english {
		if _, ok := fr[s]; ok || leaveInEnglish(s) {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s: %q", where, s))
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d of %d words have no French:", len(missing), len(english))
		for _, m := range missing[:min(len(missing), 20)] {
			t.Errorf("   %s", m)
		}
		if len(missing) > 20 {
			t.Errorf("   … and %d more", len(missing)-20)
		}
	}
}

// leaveInEnglish says whether a string is one of the ones that stays as it
// is in every language.
func leaveInEnglish(s string) bool {
	switch s {
	case "Gscore", "pppp", "ppp", "pp", "p", "mp", "mf", "f", "ff", "fff",
		"ffff", "sf", "PDF…", "SVG…", "MIDI…", "MusicXML…", "ABC…",
		"LilyPond…", "Tab", "Common", "Marcato", "Staccato", "Tenuto",
		"Accent", "Crescendo", "Diminuendo", "Tremolo", "Coda", "Segno":
		return true
	}
	// A time signature, a note name, or anything with no letters in it at
	// all: the music, which is the same in every language.
	if regexp.MustCompile(`^\d+/\d+$`).MatchString(s) {
		return true
	}
	if !regexp.MustCompile(`\p{L}{2}`).MatchString(s) {
		return true
	}
	return false
}

// frenchTable reads the French out of i18n.js, by the English it stands for.
func frenchTable(t *testing.T) map[string]string {
	t.Helper()
	js := readUI(t, "i18n.js")
	start := strings.Index(js, "const MESSAGES = {")
	if start < 0 {
		t.Fatal("i18n.js has no table of messages")
	}
	out := map[string]string{}
	for _, m := range regexp.MustCompile(
		`(?m)^  ("(?:[^"\\]|\\.)*"): ("(?:[^"\\]|\\.)*"),$`).
		FindAllStringSubmatch(js[start:], -1) {
		var k, v string
		if err := json.Unmarshal([]byte(m[1]), &k); err != nil {
			t.Fatalf("%s: %v", m[1], err)
		}
		if err := json.Unmarshal([]byte(m[2]), &v); err != nil {
			t.Fatalf("%s: %v", m[2], err)
		}
		out[k] = v
	}
	if len(out) < 300 {
		t.Fatalf("only %d words of French read; the table is bigger than that",
			len(out))
	}
	return out
}

// TestNoFrenchStandsForNothing is the other half: a translation whose English
// is nowhere in the interface is a word somebody translated and then renamed,
// and it will sit in the table for ever doing nothing.
func TestNoFrenchIsLeftBehind(t *testing.T) {
	fr := frenchTable(t)
	// Run together the way the dictionary holds them: a sentence is written
	// across several lines of markup and the dictionary keys it as one.
	all := regexp.MustCompile(`\s+`).ReplaceAllString(
		readUI(t, "index.html")+readUI(t, "palette.js")+readUI(t, "app.js"), " ")
	// What script puts on the screen itself, which is in neither table.
	spoken := map[string]bool{
		"Change": true, "Taken by": true, "reset": true,
		"Automatic (your browser’s)": true, "Language": true,
		"The language the menus and panels are written in.": true,
	}
	var orphans []string
	for en := range fr {
		if spoken[en] || strings.Contains(all, en) {
			continue
		}
		// A word written with an escape in the source — an em dash or a
		// curly quote — is there under another spelling.
		if strings.ContainsAny(en, "—’“”—") {
			continue
		}
		orphans = append(orphans, en)
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("%d translations stand for nothing on screen:", len(orphans))
		for _, o := range orphans[:min(len(orphans), 15)] {
			t.Errorf("   %q", o)
		}
	}
}

// The Mac menu bar gets the pictures too, from the one table.
//
// The bar in the native application is built from the page's own menus, for
// the reason the file says: one list of what gscore can do, not two that
// disagree. The pictures beside the entries have to travel the same way, or
// they are a second table that is right on the day it is written — and the
// Mac draws them itself rather than taking a picture from the page, because
// a menu is the system's and is set in the system's colours at whatever size
// its own text is.
//
// So the tree the page hands over carries the codepoints, and the Swift
// side reads them. This checks both ends are still there: it is the sort of
// wire that breaks silently, leaving a menu bar with the words and none of
// the pictures and nothing anywhere saying why.
func TestTheMacMenuBarIsGivenTheIcons(t *testing.T) {
	js := readUI(t, "app.js")
	tree := js[strings.Index(js, "window.gscoreMenuTree"):]
	if i := strings.Index(tree, "\n  };"); i > 0 {
		tree = tree[:i]
	}
	if !strings.Contains(tree, "icon: MENU_ICONS[node.dataset.action]") {
		t.Error("the menu tree no longer carries the icon from the table")
	}

	swift, err := os.ReadFile("../../ui/macosx/Sources/Menu.swift")
	if err != nil {
		t.Skipf("the Mac sources are not here: %v", err)
	}
	if !strings.Contains(string(swift), `entry["icon"]`) {
		t.Error("the Mac menu bar no longer reads the icon out of the tree")
	}
	if !strings.Contains(string(swift), "Glyphs.image(icon)") {
		t.Error("the Mac menu bar no longer draws the icon")
	}

	// And that the drawing follows the colours of the menu it is in rather
	// than baking them. A note is dark on a light menu and light on a dark
	// one, which is what labelColor means; the mark is red on both.
	glyphs, err := os.ReadFile("../../ui/macosx/Sources/Glyphs.swift")
	if err != nil {
		t.Skipf("the Mac sources are not here: %v", err)
	}
	for _, want := range []string{"NSColor.labelColor", "NSColor.systemRed"} {
		if !strings.Contains(string(glyphs), want) {
			t.Errorf("the menu pictures are no longer drawn with %s", want)
		}
	}
}
