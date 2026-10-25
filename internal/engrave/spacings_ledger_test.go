/*
 * spacings_ledger_test.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package engrave

import (
	"testing"

	"github.com/stricaud/gscore/internal/smufl"
)

// Notes cannot be set closer together than a ledger line is wide.
//
// A ledger line is drawn from a little before the notehead to a little after
// it. Two notes closer together than that have ledger lines that meet, and a
// run of sixteenths above the staff comes out with one unbroken rule through
// the whole of it — which reads as a staff line that is not there rather than
// as a ledger line under each note.
//
// This is arithmetic, not taste, so it is checked rather than remembered.
func TestNotesAreNeverCloserThanALedgerLineIsWide(t *testing.T) {
	o := DefaultOptions()
	head := smufl.Advance("noteheadBlack")
	ledger := head + 2*o.Defaults.LegerLineExtension

	if o.Spacings.MinNote < ledger {
		t.Errorf("the closest two notes may be set is %.3f spaces, and a "+
			"ledger line is %.3f wide: the ledger lines of neighbouring "+
			"notes would run into one another",
			o.Spacings.MinNote, ledger)
	}
}
