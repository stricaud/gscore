// Package version carries the program's version number.
package version

// Version is the gscore version this binary was built as.
//
// The number itself lives in one place and one place only: the VERSION file
// at the root of the repository. Every build that ships reads it from there
// and writes it in here at link time with -X — the Makefile's targets, the
// Debian package, the Windows installer and the WebAssembly build all do —
// so a package, a disk image and the binary's own --version cannot drift
// apart from each other or from the tag.
//
// What is left here is not a version. It is what a build with no linker
// flags is: a development build, which `go run .` and every test binary are.
// Writing the real number here as well meant setting it twice for every
// release and a test to catch the time somebody forgot, which is two places
// to edit and one of them silent. "dev" cannot go stale.
//
// The series restarts at 0.1.0 with the Go rewrite. The last release of the C
// program was 0.0.9 "harder, better, stronger, faster" in April 2005; its
// VERSION file is left as it shipped, still reading REVISION=7, having never
// been bumped for 0.0.8 or 0.0.9.
var Version = "dev"

// Development is what an un-linked build reports, and what the VERSION file
// must never contain.
const Development = "dev"
