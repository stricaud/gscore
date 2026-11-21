//go:build darwin && cgo

/*
 * savepanel_darwin.m
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdlib.h>
#include <string.h>

// GscoreLockBox turns the password boxes on and off with the tick.
@interface GscoreLockBox : NSObject
@property (strong) NSButton *tick;
@property (strong) NSSecureTextField *pw;
@property (strong) NSSecureTextField *again;
@property (strong) NSTextField *note;
- (void)toggle:(id)sender;
@end

@implementation GscoreLockBox
- (void)toggle:(id)sender {
	BOOL on = self.tick.state == NSControlStateValueOn;
	self.pw.enabled = on;
	self.again.enabled = on;
	self.note.hidden = !on || self.note.stringValue.length == 0;
}
@end

static char *copyString(NSString *s) {
	const char *u = s.UTF8String;
	char *out = malloc(strlen(u) + 1);
	strcpy(out, u);
	return out;
}

static void tell(NSString *what) {
	NSAlert *a = [NSAlert new];
	a.messageText = what;
	[a addButtonWithTitle:@"OK"];
	[a runModal];
}

// gscore_save_panel answers 1 with the path, and for a lockable file what
// to do about the lock — 0 save it open, 1 lock it under *password, 2 keep
// the password it has — or 0 when the panel was cancelled.
int gscore_save_panel(const char *dir, const char *name, int lockable, int locked,
                      char **path, char **password, int *lock) {
	__block int saved = 0;
	void (^show)(void) = ^{
		@autoreleasepool {
			[NSApp activateIgnoringOtherApps:YES];
			NSSavePanel *panel = [NSSavePanel savePanel];
			panel.title = @"Save score";
			panel.canCreateDirectories = YES;
			panel.nameFieldStringValue = [NSString stringWithUTF8String:name];
			if (dir[0]) {
				panel.directoryURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:dir] isDirectory:YES];
			}
			GscoreLockBox *box = nil;
			if (lockable) {
				UTType *t = [UTType typeWithFilenameExtension:@"gscore"];
				if (t) panel.allowedContentTypes = @[t];

				NSView *view = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 420, 104)];
				box = [GscoreLockBox new];
				box.tick = [NSButton checkboxWithTitle:@"Lock with a password"
				                                target:box action:@selector(toggle:)];
				box.tick.frame = NSMakeRect(20, 76, 380, 20);
				box.tick.state = locked ? NSControlStateValueOn : NSControlStateValueOff;
				NSTextField *l1 = [NSTextField labelWithString:@"Password:"];
				l1.frame = NSMakeRect(20, 48, 80, 20);
				box.pw = [[NSSecureTextField alloc] initWithFrame:NSMakeRect(104, 46, 220, 22)];
				NSTextField *l2 = [NSTextField labelWithString:@"Again:"];
				l2.frame = NSMakeRect(20, 20, 80, 20);
				box.again = [[NSSecureTextField alloc] initWithFrame:NSMakeRect(104, 18, 220, 22)];
				box.note = [NSTextField labelWithString:
					locked ? @"Leave empty to keep its password." : @""];
				box.note.font = [NSFont systemFontOfSize:NSFont.smallSystemFontSize];
				box.note.textColor = NSColor.secondaryLabelColor;
				box.note.frame = NSMakeRect(104, 0, 300, 16);
				for (NSView *v in @[box.tick, l1, box.pw, l2, box.again, box.note]) {
					[view addSubview:v];
				}
				[box toggle:nil];
				panel.accessoryView = view;
			}
			for (;;) {
				if ([panel runModal] != NSModalResponseOK || panel.URL == nil) return;
				*lock = 0;
				*password = NULL;
				if (box && box.tick.state == NSControlStateValueOn) {
					NSString *pw = box.pw.stringValue, *again = box.again.stringValue;
					if (pw.length == 0 && locked) {
						*lock = 2;
					} else if (pw.length == 0) {
						tell(@"Type a password, or untick Lock with a password.");
						continue;
					} else if (![pw isEqualToString:again]) {
						tell(@"The two passwords are not the same.");
						box.again.stringValue = @"";
						continue;
					} else {
						*lock = 1;
						*password = copyString(pw);
					}
				}
				*path = copyString(panel.URL.path);
				saved = 1;
				return;
			}
		}
	};
	if ([NSThread isMainThread]) {
		show();
	} else {
		dispatch_sync(dispatch_get_main_queue(), show);
	}
	return saved;
}
