//go:build darwin && cgo

/*
 * savepanel_darwin.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The system's save panel, with the choice of locking a .gscore under a
 * password in the panel itself.
 */

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UniformTypeIdentifiers
#include <stdlib.h>

int gscore_save_panel(const char *dir, const char *name, int lockable, int locked,
                      char **path, char **password, int *lock);
*/
import "C"

import "unsafe"

// SavePanelLocks says the save panel can ask about the password itself.
const SavePanelLocks = true

// SavePanel shows the system's save panel, opening in dir with name filled
// in. Where lockable, the panel carries "Lock with a password" and the
// password twice, ticked to begin with when the score is locked already.
//
// It answers the path chosen — empty when the panel was cancelled — and
// what to do about the lock: LockOpen, LockNew with the password typed, or
// LockKeep for a locked score saved with the boxes left empty.
func SavePanel(dir, name string, lockable, locked bool) (path string, lock Lock, password string) {
	cdir, cname := C.CString(dir), C.CString(name)
	defer C.free(unsafe.Pointer(cdir))
	defer C.free(unsafe.Pointer(cname))
	var cpath, cpw *C.char
	var clock C.int
	b := func(v bool) C.int {
		if v {
			return 1
		}
		return 0
	}
	if C.gscore_save_panel(cdir, cname, b(lockable), b(locked), &cpath, &cpw, &clock) == 0 {
		return "", LockOpen, ""
	}
	path = C.GoString(cpath)
	C.free(unsafe.Pointer(cpath))
	if cpw != nil {
		password = C.GoString(cpw)
		C.free(unsafe.Pointer(cpw))
	}
	return path, Lock(clock), password
}
