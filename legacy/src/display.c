/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * display.c
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

#include <gtk/gtk.h>

#include "gscore.h"
#include "display.h"

/*
 * FIXME: Should be backported to libgscore
 * Creates a new display object
 * @Returns the object
 */
Display_t *gscore_display_new(void)
{
        Display_t *display;

        display = g_malloc(sizeof(Display_t));
        
        if ( ! display ) return NULL;

        display->measure_number = TRUE;
        display->instruments = FALSE;
        display->clefs = TRUE;
        display->score_expressions = TRUE;
        display->barlines = TRUE;
        display->ending_bar = TRUE;
        display->key_signature = TRUE;
        display->time_signature = TRUE;
        display->tempo = TRUE;

        return display;

}

void
barnotes (gpointer callback_data, guint callback_action, GtkWidget *widget)
{

}

void display_new_page(gint measure)
{

}

void display_measure_line_return(GtkWidget *area, gint measure)
{
          GdkGC *gc;
          gint j = 0;

          gc = gdk_gc_new(area->window);

          gdk_draw_line(area->window, gc,
                        20,j, 20, j+20);

}
