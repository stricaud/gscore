/*
 * clipboard.h
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

struct Undo_t {
	guint8 action_type;
	void *data;
	gulong left_object_id;
} Undo_t;

struct Redo_t {
	guint8 action_type;
	void *data;
	gulong left_object_id;
} Redo_t;

gint undo_push(guint8 action_type, void *data, gulong left_object_id);
/*
 * Move the latest element pushed in the Undo stack to the Redo stack.
 * Returns this element.
 */
Undo_t *undo_pop(void);


gint redo_push(guint8 action_type, void *data, gulong left_object_id);
Redo_t *redo_push(void);


