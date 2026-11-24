//go:build js

package app

import "testing"

// keepLog has nothing to point anywhere in a build with no disk.
func keepLog(t *testing.T) string { return "" }
