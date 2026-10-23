/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * position.h
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

#ifndef _POSITION_H_
#define _POSITION_H_

#include "key_cursor.h"

gint get_position(gint key, gint staff, gint y);

gint get_pitch_from_y(gint staff, gint y);

gdouble get_y_from_position(gint key, gdouble staff_begin_y, gint position);

gdouble get_last_x_from_x(gint staff, gint x);

gdouble set_position(gint staff, gint x);

gdouble get_spacing_for_object(gint type);

gdouble set_x_pos(gint staff, gint x, gint type);

gdouble get_x_pos(gint staff, gint x);

gdouble get_y_from_position_no_key(guint16 space_btwn_lines, gint zero, gdouble staff_begin_y, gdouble height, gint position);

gboolean position_set_adjustment(GtkWidget *widget, KeyCursor_t *cursor);

gboolean position_set_adjustment_x(GtkWidget *widget, gdouble x);

#endif
