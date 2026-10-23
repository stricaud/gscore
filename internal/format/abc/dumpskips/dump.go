package main

import (
	"fmt"
	"os"

	"github.com/stricaud/gscore/internal/format/abc"
)

func main() {
	for _, name := range os.Args[1:] {
		f, err := os.Open(name)
		if err != nil {
			continue
		}
		_, err = abc.ImportAll(f)
		f.Close()
		if p, ok := err.(*abc.PartialError); ok {
			for _, s := range p.Skipped {
				fmt.Printf("%s\t%s\n", name, s)
			}
		}
	}
}
