package nwc

import (
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestScratchDump(t *testing.T) {
	path := os.Getenv("NWC_DUMP")
	if path == "" {
		t.Skip("set NWC_DUMP")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := Import(f)
	if s == nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Log("partial:", err)
	}
	counts := map[string]int{}
	for _, p := range s.Parts {
		for _, m := range p.Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok {
						counts[n.Accidental.String()]++
					}
				}
			}
		}
	}
	t.Log("accidentals:", counts)
	for _, p := range s.Parts {
		t.Logf("part %q key=%d clef=%v measures=%d", p.Name, p.Key.Fifths, p.Clef, len(p.Measures))
	}
	if len(s.Parts) > 1 {
		shown := 0
		for _, m := range s.Parts[1].Measures {
			for _, v := range m.Voices {
				for _, el := range v.Elements {
					if n, ok := el.(*model.Note); ok && shown < 12 {
						t.Logf("  %v acc=%v dur=%v", n.Pitch, n.Accidental, n.Dur)
						shown++
					}
				}
			}
		}
	}
}
