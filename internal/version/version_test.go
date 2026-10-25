package version

import (
	"os"
	"strings"
	"testing"
)

// The number lives in the VERSION file at the root, and the build injects it
// with -X. Nothing stops the two from drifting except this: a `go build` with
// no linker flags, which is what `go run .` and every test binary is, falls
// back to the literal in version.go, and a stale literal would be written
// into saved scores and reported by --version without anybody noticing.
func TestTheLiteralMatchesTheVERSIONFile(t *testing.T) {
	b, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(b))
	if want == "" {
		t.Fatal("the VERSION file is empty")
	}
	if Version != want {
		t.Errorf("version.go says %q, the VERSION file says %q:\n"+
			"\tthe file is the authority — set the literal in "+
			"internal/version/version.go to %q", Version, want, want)
	}
}
