/*
 * objects_callbacks.h
 * gscore - a music score editor
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

#ifndef _OBJECTS_CALLBACKS_H_
#define _OBJECTS_CALLBACKS_H_

void object_cursor_callback(GtkWidget *widget, GdkEventExpose *event);

/* Notes */
void object_doublewhole_callback(GtkWidget *widget, GdkEventExpose *event);
void object_whole_callback(GtkWidget *widget, GdkEventExpose *event);
void object_half_callback(GtkWidget *widget, GdkEventExpose *event);
void object_quarter_callback(GtkWidget *widget, GdkEventExpose *event);
void object_eighth_callback(GtkWidget *widget, GdkEventExpose *event);
void object_sixteenth_callback(GtkWidget *widget, GdkEventExpose *event);
void object_thirtysecond_callback(GtkWidget *widget, GdkEventExpose *event);
void object_sixtyfourth_callback(GtkWidget *widget, GdkEventExpose *event);

/* Rests */
void object_doublewholerest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_wholerest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_halfrest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_quarterrest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_eighthrest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_sixteenthrest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_thirtysecondrest_callback(GtkWidget *widget, GdkEventExpose *event);
void object_sixtyfourthrest_callback(GtkWidget *widget, GdkEventExpose *event);

/* Accidentals */
void object_sharp_callback(GtkWidget *widget, GdkEventExpose *event);
void object_doublesharp_callback(GtkWidget *widget, GdkEventExpose *event);
void object_flat_callback(GtkWidget *widget, GdkEventExpose *event);
void object_doubleflat_callback(GtkWidget *widget, GdkEventExpose *event);
void object_natural_callback(GtkWidget *widget, GdkEventExpose *event);
void object_parenthesis_callback(GtkWidget *widget, GdkEventExpose *event);

/* Properties */
void object_beam_callback(GtkWidget *widget, GdkEventExpose *event);
void object_tie_callback(GtkWidget *widget, GdkEventExpose *event);
void object_autoupdown_callback(GtkWidget *widget, GdkEventExpose *event);
void object_up_callback(GtkWidget *widget, GdkEventExpose *event);
void object_down_callback(GtkWidget *widget, GdkEventExpose *event);
void object_duration_callback(GtkWidget *widget, GdkEventExpose *event);
void object_staccato_callback(GtkWidget *widget, GdkEventExpose *event);
void object_staccato_op_callback(GtkWidget *widget, GdkEventExpose *event);

/* Barlines */
void object_barline_single_callback(GtkWidget *widget, GdkEventExpose *event);
void object_barline_double_callback(GtkWidget *widget, GdkEventExpose *event);
void object_barline_openrepeat_callback(GtkWidget *widget, GdkEventExpose *event);
void object_barline_closerepeat_callback(GtkWidget *widget, GdkEventExpose *event);
void object_barline_opencloserepeat_callback(GtkWidget *widget, GdkEventExpose *event);


#endif
