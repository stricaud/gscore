/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * objects.h
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

#ifndef _OBJECTS_H_
#define _OBJECTS_H_

#include <gscore.h>

gint object_get_spacing(gint type);

gboolean add_object(Score_t *score, gint staff, gint type,
		    accidentals_e accidentals, object_e nature,
		    gulong group_id, gint x, gint y, gint x2, gint y2,
		    gint x3, gint y3, gint pitch, gint tab_number,
		    gboolean is_selected);

gboolean object_insert(Score_t *score, Object_t *rightobj, gint staff, gint type, accidentals_e accidentals, object_e nature, gulong group_id, gint pitch, gint tab_number, gboolean is_selected);

gboolean remove_object(Score_t *score, gulong id);

gboolean remove_object_selected(Score_t *score);

Object_t *object_get_next(Score_t *score, Object_t *object);

Object_t *object_get_previous(Score_t *score, Object_t *object);

Object_t *object_get_left(Staff_t *staff, gdouble x);

gdouble object_get_x(Staff_t *staff, Object_t *object);
/* gdouble object_get_left_x(Staff_t *staff, gdouble x); */

Object_t *object_get_right(Staff_t *staff, gdouble x);

/* void get_object_properties(GtkWidget *event_box, GdkEventButton *event, gpointer data); */

void set_object(const gchar *image_filename, GCallback callback_handler, gboolean is_selected, gint x, gint y);

gint add_note (guint staff, gint type, gint x, gint offset);

gint add_barline (guint staff, gint x);

gint update_staffs_size (gint value);

void note_append(gint note_type);

gboolean object_selected_change_type(Score_t *score, gint newtype);

gboolean object_selected_pitch_up(Score_t *score);

gboolean object_selected_pitch_down(Score_t *score);

#endif
