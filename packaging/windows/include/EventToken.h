/*
 * EventToken.h - a spelling, nothing more.
 *
 * WebView2.h, which Microsoft ships and webview_go bundles, asks for
 * "EventToken.h". mingw-w64 ships that header as eventtoken.h, all lower
 * case. On macOS the filesystem does not care and the build works; on Linux
 * it does, and the build stops here:
 *
 *   WebView2.h:978:10: fatal error: EventToken.h: No such file or directory
 *
 * So the capitalised name exists on the include path, and hands straight on
 * to the real one. include_next rather than include: it carries on from the
 * directory after this file's own, so on a case-insensitive filesystem this
 * header does not find itself and recurse.
 *
 * (C) Copyright 2026 Sebastien Tricaud. MIT licence; see LICENSE.
 */

#include_next <eventtoken.h>
