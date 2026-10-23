/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * note.c
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * e-mail : toady@gscore.og
 * URL    : http://www.gscore.org
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA 02111-1307, USA.
 */

#include <gtk/gtk.h>

#include "gscore.h"
#include "constants.h"

/* Returns TRUE if the type is a note */
extern gboolean 
is_note(gint type)
{
        switch(type) {

        case DOUBLEWHOLE:
        case WHOLE:
        case HALF:
        case QUARTER:
        case EIGHTH:
        case SIXTEENTH:
        case THIRTYSECOND:
        case SIXTYFOURTH:
                return TRUE;
        default:
                return FALSE;
        }

        return FALSE;
}

extern gboolean
is_barline(gint type)
{
        switch(type) {

        case BARLINE:
        case BARLINE_SINGLE:
        case BARLINE_DOUBLE:
        case BARLINE_OPENREPEAT:
        case BARLINE_CLOSEREPEAT:
        case BARLINE_OPENCLOSEREPEAT:
        case BARLINE_OPEN:
        case BARLINE_CLOSE:
                return TRUE;
        default:
                return FALSE;

        }

        return FALSE;
}
