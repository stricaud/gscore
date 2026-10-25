// Package version carries the program's version number.
package version

// Version is the canonical gscore version.
//
// The VERSION file at the root of the repository is where this number is
// set. The build reads it from there and writes it in here at link time
// (`-X`), so a package, a disk image and the binary's own --version can
// never drift apart. The literal below is what a plain `go build` or `go
// test` gets, and the test in this package fails if it stops matching the
// file — which is how editing VERSION alone stays safe.
//
// The series restarts at 0.1.0 with the Go rewrite. The last release of the C
// program was 0.0.9 "harder, better, stronger, faster" in April 2005; its
// 0.0.9's VERSION file file is left as it shipped, still reading REVISION=7, having
// never been bumped for 0.0.8 or 0.0.9.
var Version = "0.1.1"
