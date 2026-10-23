/*
 * score.h
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

#ifndef _SCORE_H_
#define _SCORE_H_

#include "key_cursor.h"

Score_t *gscore_score_new(void);

gint score_add_tab (gpointer callback_data, guint callback_action,
                    GtkWidget * widget);

void remove_score (gpointer callback_data, guint callback_action,
	    GtkWidget * widget);

void score2tab (gpointer callback_data, guint callback_action,
		GtkWidget * widget);

void properties (gpointer callback_data, guint callback_action,
	    GtkWidget * widget);

void score_midi_play (gpointer callback_data, guint callback_action,
                      GtkWidget * widget);

Score_t *score_get_from_widget(GtkWidget *widget);

gboolean score_set_to_widget(Score_t *score, GtkWidget *widget);

GtkWidget *score_get_area_from_widget(GtkWidget *widget);

Display_t *score_get_display_from_widget(GtkWidget *widget);

Score_selection_t *score_get_selection_from_widget(GtkWidget *widget);

KeyCursor_t *score_get_cursor_from_widget(GtkWidget *widget);

GtkScrolledWindow *score_get_scrolled_window_from_widget(GtkWidget *widget);

GtkWidget *score_get_setkey_treble_rb_from_widget(GtkWidget *widget);

GtkWidget *score_get_setkey_bass_rb_from_widget(GtkWidget *widget);

GtkWidget *score_get_setkey_tenor_rb_from_widget(GtkWidget *widget);

GtkWidget *score_get_setkey_alto_rb_from_widget(GtkWidget *widget);

GtkWidget *score_get_sks_clef_label_from_widget(GtkWidget *widget);

GtkWidget *score_get_sks_image_from_widget(GtkWidget *widget);

GtkWidget *score_get_sks_hbox_from_widget(GtkWidget *widget);

GtkWidget *score_get_tempo_spinbutton_from_widget(GtkWidget *widget);

GtkWidget *score_get_tempo_entry_from_widget(GtkWidget *widget);

GtkWidget *score_get_title_entry_from_widget(GtkWidget *widget);

GtkWidget *score_get_subtitle_entry_from_widget(GtkWidget *widget);

GtkWidget *score_get_composer_entry_from_widget(GtkWidget *widget);

GtkWidget *score_get_insert_text_entry_from_widget(GtkWidget *widget);

GtkWidget *score_get_insert_text_font_from_widget(GtkWidget *widget);

GtkWidget *score_get_insert_text_color_from_widget(GtkWidget *widget);

void score_create_window(Score_t *score);

gboolean score_set_staff_extremity_end_x(Score_t *score, gdouble extremity_end_x);

/*
void keybindings (gpointer callback_data,
		  guint callback_action, GtkWidget * widget);
*/
#endif
