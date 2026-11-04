/*
 * uiscale_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * How big the interface is drawn.
 *
 * A fact about the screen in front of somebody, not about any score, so it
 * is remembered with the other settings and never written into a file.
 */

package app

import (
	"encoding/json"
	"testing"
)

func TestTheInterfaceStartsAtTheSizeItWasDesignedAt(t *testing.T) {
	h := newHarness(t)
	var out map[string]any
	if err := json.Unmarshal([]byte(h.get("/api/settings")), &out); err != nil {
		t.Fatal(err)
	}
	if got, _ := out["uiScale"].(float64); got != 1 {
		t.Errorf("a fresh gscore draws its interface at %v", out["uiScale"])
	}
}

func TestTheInterfaceSizeIsRemembered(t *testing.T) {
	h := newHarness(t)
	h.post("/api/settings?uiScale=1.5")
	if got := UIScale(); got != 1.5 {
		t.Errorf("it came back as %v, want 1.5", got)
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(h.get("/api/settings")), &out)
	if got, _ := out["uiScale"].(float64); got != 1.5 {
		t.Errorf("the settings say %v", out["uiScale"])
	}
	h.post("/api/settings?uiScale=1")
}

// Below three-quarters the labels stop being legible and past twice the
// size the menus no longer fit a laptop screen, at which point the thing
// to change is the window.
func TestASizeOutsideWhatIsUsableIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, bad := range []string{"0", "0.1", "4", "-1", "enormous"} {
		if why := h.postFails("/api/settings?uiScale=" + bad); why == "" {
			t.Errorf("%q was refused without saying why", bad)
		}
	}
	if got := UIScale(); got != 1 {
		t.Errorf("a refused size was kept anyway: %v", got)
	}
}

// A setting written by a later gscore, or by hand, must not leave the
// interface undrawable.
func TestANonsenseSizeOnDiskReadsAsTheDesignedSize(t *testing.T) {
	stored.Lock()
	was := stored.settings.UIScale
	stored.settings.UIScale = 97
	stored.Unlock()
	t.Cleanup(func() {
		stored.Lock()
		stored.settings.UIScale = was
		stored.Unlock()
	})
	if got := UIScale(); got != 1 {
		t.Errorf("a stored 97 reads as %v", got)
	}
}
