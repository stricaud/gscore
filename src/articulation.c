/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * articulation.c: manage notes articulations
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

#include <gtk/gtk.h>

#include "gscore.h"
#include "common.h"
#include "constants.h"
#include "staff.h"
#include "debug.h"
#include "articulation.h"

void
staccato_notes_selected(void)
{
/*      GList    * listrunner; */
/*      Object_t * object; */


/*      listrunner = g_list_first(Score.Staff[get_staff_selected()].Object_list); */

/*      while (listrunner) */
/*      { */
/*           object = (Object_t *)listrunner->data; */

/*           if (object->is_selected) */
/*             object->nature &= O_STACCATO; */

/*           listrunner = g_list_next(listrunner); */
/*      }      */

/*      g_list_free(listrunner); */

/* /\*      refresh();                 /\\* refresh the drawing area *\\/ *\/ */
}

extern void
articulation_draw_staccato(gint staff, gboolean up, gint x, gint y)
{
/*      GdkGC *gc; */

/*      gc = gdk_gc_new(Score.area->window); */

/*      if (up) */
/*        { */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 4, y + 13); */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 4, y + 14); */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 3, y + 13); */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 3, y + 14); */
/*        } else */
/*          { */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 4, y - 6); */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 4, y - 5); */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 5, y - 6); */
/*          gdk_draw_point(Score.area->window, gc, */
/*                         Score.Staff[staff].startx + x + 5, y - 5); */
/*          } */

} 
