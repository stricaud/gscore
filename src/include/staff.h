/*
 * staff.h
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
#ifndef _STAFF_H_
#define _STAFF_H_

#include <glib.h>
#include <gtk/gtk.h>
#include "gscore.h"

gboolean create_staff(Score_t *score, guint8 nb_lines,
		      guint8 space_btw_lines,
                      gdouble extremity_begin_x,
		      gdouble extremity_begin_y);

gboolean delete_staff(Score_t *score, gint staff);

gint get_staff_selected(const Score_t *score);

gint staff_get_key_signature(const Score_t * score, guint staff);

gboolean staff_set_key(Score_t *score, gint staff, gint key);

void staff_set_key_callback(GtkButton *button, gpointer user_data);

gint staff_get_key(const Score_t *score, gint staff);

gboolean staff_set_current_x(Score_t *score, gint staff, gint current_x);

gint staff_get_current_x(const Score_t *score, gint staff);

gdouble get_staff_extremity_begin_x(const Score_t *score, gint staff);

gdouble get_staff_extremity_begin_y(const Score_t *score, gint staff);

gboolean staff_set_key_signature(Score_t *score, gint staff, gint key_signature);

gboolean staff_set_time_signature(Score_t *score, gint staff, gint signature_type, gint number_of_beats, gint beat_duration);

gboolean staff_set_midi_instrument(Score_t *score, gint staff, gint midi_instrument);

gint staff_get_y_for_next(const Score_t *score);

gboolean staff_update_statusbar(GladeXML *xml);

gboolean staff_set_start_x(Score_t *score, gint staff, gint start_x);

gint staff_get_start_x(Score_t *score, gint staff);

void staff_key_signature(gpointer callback_data, guint callback_action, GtkWidget *widget);

void staff_time_signature(gpointer callback_data, guint callback_action, GtkWidget *widget);

/* void staff_set_midi_instrument(gpointer callback_data, guint callback_action, GtkWidget *widget); */

void staff_display_measures_numbers(gpointer callback_data, guint callback_action, GtkWidget *widget);

gint get_staff_id(Score_t *score, gdouble y);

void deselect_all_staffs(void);

void set_staff_selected(Score_t *score, gint staff);

void set_staff_unselect(Score_t *score, gint staff);

gint staff_remove_staff(gpointer callback_data, guint callback_action, GtkWidget *widget);

gint get_staff_key(const Score_t *score, gint staff);

void test_remove(void);

void staff_add_staff (gpointer callback_data, guint callback_action, GtkWidget * widget);

gdouble get_staff_extremity_end_y(const Score_t *score, gint staff_id);

Staff_t *staff_selected_get_from_score(Score_t *score);

void update_key_signature(GtkButton *widget, gpointer user_data);

#endif
