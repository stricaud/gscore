/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * text.c
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
#include <pango/pango.h>

#include "gscore.h"
#include "text.h"

void set_text(GtkWidget *area, gchar *text, gint x, gint y)
{

	GdkGC *gc;

	PangoLayout *layout;


	layout = gtk_widget_create_pango_layout (area, text);


	gc = gdk_gc_new(area->window);

	gdk_draw_layout(area->window, gc,
			x, y, layout);

	g_object_unref(layout);

}

void set_dynamic_text(GtkWidget *area, gchar *text, gint x, gint y)
{

	GdkGC *gc;

	PangoLayout *layout;
	PangoFontDescription *desc;

	layout = gtk_widget_create_pango_layout (GTK_WIDGET (area), text);

	desc = pango_font_description_new();
	pango_font_description_set_weight(desc, PANGO_WEIGHT_BOLD);
	pango_font_description_set_style(desc, PANGO_STYLE_ITALIC);

	pango_layout_set_font_description(layout, desc);

	gc = gdk_gc_new(area->window);

	gdk_draw_layout(area->window, gc,
			x, y, layout);

	g_object_unref(layout);

}
