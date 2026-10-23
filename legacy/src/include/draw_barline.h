/*
 * draw_barline.h
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

#ifndef _DRAW_BARLINE_H_
#define _DRAW_BARLINE_H_

gint draw_barline_single(GtkWidget *area, Staff_t *staff, gint x);

gint draw_barline_double(GtkWidget *area, Staff_t *staff, gint x);

gint draw_barline_openrepeat(GtkWidget *area, Staff_t *staff, gint x);

gint draw_barline_closerepeat(GtkWidget *area, Staff_t *staff, gint x);

gint draw_barline_opencloserepeat(GtkWidget *area, Staff_t *staff, gint x);

gint draw_barline_endstaff(GtkWidget *area, Staff_t *staff);

#endif
