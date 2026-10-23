/*
 * chords.h
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

#ifndef _CHORDS_H_
#define _CHORDS_H_

gboolean is_chord(Staff_t *staff, Object_t *object);

/* Returns the x position where the note should be */
/* Gives the information if the base note has the stem up or down */
/* And also, if the note must be drawn with the head oriented on the left or right (it happens of the pitch is just below/above) */
guint make_chord(Staff_t *staff, gulong objectid, gulong group_id, gint pitch, gboolean *stemup, gboolean *notehead_left);

#endif
