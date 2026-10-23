/*
 * selection.h
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

void set_selection_accidentals(accidentals_e selection);

accidentals_e get_selection_accidentals(void);

gint set_selection_origin(GtkWidget *widget, GdkEventButton *event);

gint do_selection(gint x_root, gint y_root, gint x, gint y);

gint highlight_selection(Score_t *score, gdouble x_origin, gdouble y_origin, gdouble x, gdouble y);

gint undo_selection(Score_t *score);

gint undo_selection_widget(GtkWidget *widget, gpointer user_data);

gint get_selection_object_type(gint x, gint y, gint staff);

gint get_selection_id(gint x, gint y, gint staff);

gdouble get_selection_x_selection(gint x, gint y, gint staff);

gdouble get_selection_y_selection(gint x, gint y, gint staff);

gboolean get_selection_with_sharp(gint x, gint y, gint staff);

gboolean get_selection_with_flat(gint x, gint y, gint staff);

gboolean get_selection_with_natural(gint x, gint y, gint staff);
