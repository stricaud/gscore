package pdf

import (
	"bytes"
	"os"
	"testing"

	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/format/abc"
)

func TestRenderToPDF(t *testing.T) {
	path := os.Getenv("ABC")
	if path == "" {
		t.Skip("set ABC")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := abc.Import(f)
	if err != nil {
		t.Fatal(err)
	}
	eopt := engrave.DefaultOptions()
	eopt.Mode = engrave.Paginated
	eopt.LineWidth = 72
	eopt.ShowPartNames = true
	eopt.ShowMeasureNumbers = true
	lay := engrave.Engrave(s, eopt)
	t.Logf("layout %.1f x %.1f, %d systems", lay.Width, lay.Height, len(lay.Systems))

	var buf bytes.Buffer
	opt := DefaultOptions()
	opt.Title = s.Identity.Title
	if err := Render(&buf, lay, opt); err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("OUT")
	if out == "" {
		out = "/tmp/gscore-test.pdf"
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s, %d bytes", out, buf.Len())
}
