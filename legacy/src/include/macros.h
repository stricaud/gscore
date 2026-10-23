/*
 * macros.h: a crazy file for a lazy boy
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
 *
 */

#include "gscore.h"


#define get_file_from_data_dir(file) g_strconcat(DATA_DIR, G_DIR_SEPARATOR_S, "gscore", G_DIR_SEPARATOR_S, file, NULL)

/* #define get_plugins_path() g_strconcat(DATA_DIR, G_DIR_SEPARATOR_S, "share", G_DIR_SEPARATOR_S, "gscore", G_DIR_SEPARATOR_S, "plugins", NULL) */

#define for_all_staves(count) for ( count = 0; count < Score.nb_staves; count++ )
