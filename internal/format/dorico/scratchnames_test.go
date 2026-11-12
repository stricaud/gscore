package dorico

import (
	"fmt"
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/model"
)

func TestScratchDumpNames(t *testing.T) {
	if os.Getenv("DUMP") == "" {
		t.Skip("set DUMP=1")
	}
	f, err := os.Open(os.Getenv("HOME") + "/Downloads/Beethoven_Symph_5.dorico")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scores, _ := Import(f)
	for si, s := range scores {
		fmt.Printf("== movement %d: %s\n", si+1, s.Info.Title)
		for pi, p := range s.Parts {
			notes := 0
			for _, m := range p.Measures {
				for _, v := range m.Voices {
					for _, el := range v.Elements {
						if _, ok := el.(*model.Rest); !ok {
							notes++
						}
					}
				}
			}
			fmt.Printf("  part %2d  name=%-40q instrument=%-28q sound=%-28q notes=%d\n",
				pi, p.Name, p.Instrument, p.Sound, notes)
		}
		if si >= 1 {
			break
		}
	}
}
