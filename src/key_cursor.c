/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * cursor.c
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * e-mail : toady@gscore.org
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

/* BEWARE: This has nothing to do with the mouse cursor, this file stand for the
 * cursor controlled by the keyboard
 */

#include <gtk/gtk.h>

#include "gscore.h"
#include "constants.h"
#include "position.h"
#include "common.h"
#include "key_cursor.h"
#include "score.h"

extern 
void draw_key_cursor(GtkWidget *area, gint position, gdouble current_x, gdouble extremity_begin_y)
{

	  GdkGC * gc;
	  GdkColor color;
	  KeyCursor_t *cursor = score_get_cursor_from_widget(area);

	  gdouble y = 0;

	  gc = gdk_gc_new(area->window);

	  color.red   = 20000;
	  color.green = 20000;
	  color.blue  = 20000;

	  gdk_colormap_alloc_color(gtk_widget_get_colormap(area), &color, TRUE, TRUE);
	  gdk_gc_set_foreground(gc, &color);


	  y = get_y_from_position_no_key(8, 4, extremity_begin_y, 8, position) + 1;

	  gdk_draw_rectangle(area->window, gc,
	                     TRUE, current_x, y, 10, 6);

	  cursor->x_returned = current_x;
	  cursor->y_returned = y - 1; /* - 1 because of the fact that it returns a x value to draw the object */

}

extern 
gint get_key_cursor_position(KeyCursor_t *cursor)
{
	return cursor->position;
}

extern 
void get_key_cursor_action(gint staff)
{
	/*      action_type = APPEND_NOTE; */
}
