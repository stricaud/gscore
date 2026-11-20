//go:build darwin && cgo

/*
 * pasteboard_darwin.m
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

char *gscore_pasteboard(const char *kind, int *n, long *change)
{
	@autoreleasepool {
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		*change = (long)[pb changeCount];
		*n = 0;
		NSData *data = [pb dataForType:[NSString stringWithUTF8String:kind]];
		if (data == nil || [data length] == 0) {
			return NULL;
		}
		char *out = malloc([data length]);
		if (out == NULL) {
			return NULL;
		}
		memcpy(out, [data bytes], [data length]);
		*n = (int)[data length];
		return out;
	}
}
