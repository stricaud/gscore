package version

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The number lives in the VERSION file at the root and nowhere else.
//
// This used to check that the literal in version.go matched that file, which
// meant the number was written in two places and the test existed to catch
// the release where somebody updated only one of them — as happened at
// 0.1.2. The literal is a placeholder now, so there is nothing to keep in
// step; what is worth checking is that it has not quietly become a number
// again, and that the file it is supposed to come from says something a
// version can be.
func TestTheLiteralIsNotAVersion(t *testing.T) {
	if Version != Development {
		t.Errorf("version.go says %q; the number belongs in the VERSION file "+
			"alone, and the build puts it here with -X. Set the literal back "+
			"to %q.", Version, Development)
	}
}

func TestTheVERSIONFileSaysAVersion(t *testing.T) {
	b, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(b))
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(got) {
		t.Errorf("the VERSION file says %q, which is not a version number", got)
	}
}
