/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * mode.c
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


void 
on_mode_none_activate(GtkWidget *widget)
{
	g_print("None\n");
	widget = glade_xml_get_widget (gladexml, "sw_mode_hbox");
	gtk_widget_hide(widget);
	widget = glade_xml_get_widget (gladexml, "sw_mode_sw");
	gtk_widget_hide(widget);
}

void 
on_mode_abc_activate(GtkWidget *widget)
{
	g_print("ABC\n");
	widget = glade_xml_get_widget (gladexml, "sw_mode_hbox");
	gtk_widget_show(widget);
	widget = glade_xml_get_widget (gladexml, "sw_mode_sw");
	gtk_widget_show(widget);
}

void 
on_mode_lilypond_activate(GtkWidget *widget)
{
	g_print("Lilypond\n");
	widget = glade_xml_get_widget (gladexml, "sw_mode_hbox");
	gtk_widget_show(widget);
	widget = glade_xml_get_widget (gladexml, "sw_mode_sw");
	gtk_widget_show(widget);
}
