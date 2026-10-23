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
		`(?m)\b(tool|apply|measure|staff|ending|custom|needsRange)\s*:`).
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
