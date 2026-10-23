/*
 * insert.h
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2004 Sebastien Tricaud
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

#ifndef _INSERT_H_
#define _INSERT_H_

void insert_key_callback(GtkWidget * widget);

void insert_text_callback(GtkButton *widget, gpointer user_data);

gint insert_barline(gpointer callback_data, guint callback_action, GtkWidget *widget);

gint insert_barline_choose(gpointer callback_data, guint callback_action, GtkWidget *widget);

void insert_key(gpointer callback_data, guint callback_action,
		GtkWidget * widget);

void flow_direction(gpointer callback_data, guint callback_action,
		GtkWidget * widget);

void key_signature(gpointer callback_data, guint callback_action,
	    GtkWidget * widget);

void time_signature(gpointer callback_data, guint callback_action,
		    GtkWidget * widget);

void insert_tempo(gpointer callback_data, guint callback_action,
		GtkWidget * widget);

void insert_text(gpointer callback_data, guint callback_action,
		 GtkWidget * widget);

void
insert_dyn_mezzopiano(gpointer callback_data, guint callback_action, 
		      GtkWidget *widget);

gint insert_dynamic_alt_cresc(gpointer callback_data, guint callback_action, 
                              GtkWidget *widget);

gint insert_dynamic_alt_decresc(gpointer callback_data, guint callback_action, 
                                GtkWidget *widget);

void insert_barline_single(GtkWidget *drawingarea, GdkEventExpose *ev);

void insert_barline_double(GtkWidget *drawingarea, GdkEventExpose *ev);

void insert_barline_openrepeat(GtkWidget *drawingarea, GdkEventExpose *ev);

void insert_barline_closerepeat(GtkWidget *drawingarea, GdkEventExpose *ev);

void insert_barline_opencloserepeat(GtkWidget *drawingarea, GdkEventExpose *ev);

#endif
