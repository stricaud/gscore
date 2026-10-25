// Package version carries the program's version number.
package version

// Version is the canonical gscore version.
//
// The series restarts at 0.1.0 with the Go rewrite. The last release of the C
// program was 0.0.9 "harder, better, stronger, faster" in April 2005; its
// 0.0.9's VERSION file file is left as it shipped, still reading REVISION=7, having
// never been bumped for 0.0.8 or 0.0.9.
const Version = "0.1.0"
