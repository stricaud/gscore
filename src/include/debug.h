/*
 * debug.h: function for debugging purpose
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2004 Sebastien Tricaud
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
#ifndef _DEBUG_H_
#define _DEBUG_H_

#include "gscore.h"

void print_objects_staff(Score_t *score, gint staff);

void print_object(Object_t *object);

void print_staves_list(Score_t *score);

void print_note_list(gpointer *data, gpointer *user_data);

void debug_msg(gchar *message);

void show_object_tooltip(Object_t *object);

#endif
