/*
 * common.h
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

/* #include "gscore.h" */

#ifndef _COMMON_H_
#define _COMMON_H_

#include "gscore.h"

void clean_statusbar(void);

void refresh(GtkWidget *area);

Score_t *get_score(void);

Object_t * get_object_list(gint staff, gint index);

void update_statusbar(gchar * context_id, gchar * message);

void score_new_size(GtkWidget *area);

void gw_message_error(gchar *message);

void gw_message_info(gchar *message);

void not_implemented_cb(void);

void not_implemented_menu_cb(gpointer callback_data, guint callback_action, GtkWidget *widget);

gchar *get_filename(const gchar *filename);

gchar *get_plugin_file(const gchar *filename);

gboolean file_exists(const gchar *filename);

gboolean is_even(gint number);

#endif
