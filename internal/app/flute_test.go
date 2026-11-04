/*
 * flute_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Looking through the flute's fingerings.
 *
 * Twenty-nine thousand of them is not a list anybody can read, so what
 * the endpoint is for is the ordering and the filtering: nearest the
 * standard chart first, and a way of saying "only the ones in tune" or
 * "only the ones that take the gizmo".
 */

package app

import (
	"encoding/json"
	"testing"
)

func fluteAsk(t *testing.T, h *harness, query string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/flute"+query)), &out); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return out
}

// With nothing asked, the instrument: the window draws itself from this
// rather than carrying a second copy of what a flute is.
func TestTheEndpointSaysWhatAFluteIs(t *testing.T) {
	h := newHarness(t)
	d := fluteAsk(t, h, "")
	keys, _ := d["keys"].([]any)
	notes, _ := d["notes"].([]any)
	if len(keys) < 20 {
		t.Errorf("it names %d keys", len(keys))
	}
	if len(notes) < 36 {
		t.Errorf("it gives %d notes", len(notes))
	}
	if left, _ := d["left"].(float64); left < 1 || int(left) >= len(keys) {
		t.Errorf("the left hand has %v of the keys", d["left"])
	}
}

func TestANotesFingeringsComeBackNearestTheChartFirst(t *testing.T) {
	h := newHarness(t)
	d := fluteAsk(t, h, "?note=D5")
	if total, _ := d["total"].(float64); total < 100 {
		t.Fatalf("D5 has %v alternatives", d["total"])
	}
	shown, _ := d["shown"].([]any)
	if len(shown) == 0 {
		t.Fatal("nothing was shown")
	}
	last := -1
	for _, row := range shown {
		m := row.(map[string]any)
		away := int(m["away"].(float64))
		if away < last {
			t.Fatalf("the list goes %d keys from the chart after %d", away, last)
		}
		last = away
	}
	// And the standard one is given on its own, since it is the answer
	// nine times in ten.
	std, _ := d["standard"].(map[string]any)
	if s, _ := std["spell"].(string); s != "Th 2 3 | 1 2 3" {
		t.Errorf("the D5 chart came back as %q", s)
	}
}

func TestTheFiltersNarrowIt(t *testing.T) {
	h := newHarness(t)
	all := fluteAsk(t, h, "?note=E6")["total"].(float64)

	for _, q := range []string{"&away=1", "&within=5", "&using=gizmo",
		"&without=gizmo"} {
		got := fluteAsk(t, h, "?note=E6"+q)["total"].(float64)
		if got > all {
			t.Errorf("%s gave %v of %v, which is more than all of them",
				q, got, all)
		}
	}
	// Holding and not holding the same key is all of them between the two.
	with := fluteAsk(t, h, "?note=E6&using=gizmo")["total"].(float64)
	without := fluteAsk(t, h, "?note=E6&without=gizmo")["total"].(float64)
	if with+without != all {
		t.Errorf("%v with the gizmo and %v without, of %v", with, without, all)
	}
}

func TestALongAnswerIsPaged(t *testing.T) {
	h := newHarness(t)
	first := fluteAsk(t, h, "?note=D5&limit=10")
	second := fluteAsk(t, h, "?note=D5&limit=10&offset=10")
	a, _ := first["shown"].([]any)
	b, _ := second["shown"].([]any)
	if len(a) != 10 || len(b) != 10 {
		t.Fatalf("pages of %d and %d", len(a), len(b))
	}
	if a[0].(map[string]any)["keys"] == b[0].(map[string]any)["keys"] {
		t.Error("the second page begins where the first did")
	}
}

// A trill is asked for as a pair of notes, because that is what a trill
// is. The answer holds one fingering and names the key that shakes.
func TestATrillIsAskedForAsTwoNotes(t *testing.T) {
	h := newHarness(t)
	d := fluteAsk(t, h, "?from=C5&to=D5")
	if total, _ := d["total"].(float64); total < 1 {
		t.Fatal("no way of trilling C5 to D5")
	}
	rows, _ := d["trills"].([]any)
	first := rows[0].(map[string]any)
	if key, _ := first["key"].(string); key != "tr1" {
		t.Errorf("the first way shakes %q, want the first trill key", key)
	}
	hold := first["hold"].(map[string]any)
	if s, _ := hold["spell"].(string); s != "1 | D#" {
		t.Errorf("it holds %q", s)
	}
	if down, _ := first["down"].(bool); !down {
		t.Error("it says the trill key comes up rather than going down")
	}
}

func TestANoteThatIsNotThereSaysSo(t *testing.T) {
	h := newHarness(t)
	for _, q := range []string{"?note=Z9", "?from=Z9&to=D5", "?from=D5&to=Z9"} {
		if why := h.getFails("/api/flute" + q); why == "" {
			t.Errorf("%s was refused without saying why", q)
		}
	}
}
