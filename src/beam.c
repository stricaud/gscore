/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * beam.c: manage the notes selected to be beamed
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
#include "spacings.h"
#include "score.h"
#include "beam.h"

extern void
beam_notes_selected(GtkButton *widget, gpointer user_data)
{

	Staff_t *staff_data;
	Score_t *score = NULL;
	GtkWidget *area;
	
	GList *listrunner = NULL;
	GList *listrunner_next = NULL;

	score = score_get_from_widget(GTK_WIDGET(widget));
        
        if ( ! score ) g_warning("No Score!");

	area = score_get_area_from_widget(GTK_WIDGET(widget));

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	listrunner_next = g_list_next(listrunner);

	while (listrunner) {
                Object_t *note;
                Object_t *next_note = NULL;

                note = (Object_t *)listrunner->data;

                if (listrunner_next)
                        next_note = (Object_t *)listrunner_next->data;

                if (note->is_selected) {
                        note->nature |= O_BEAMED;
                        if ( next_note ) {
                                if (( ! next_note->is_selected ) || ( ! listrunner_next)) {
                                        note->nature |= O_LAST_BEAMED;
                                }
                        } else {
                                if (!listrunner_next)
                                        note->nature |= O_LAST_BEAMED;
                        }
                }

                listrunner = g_list_next(listrunner);

                if ((listrunner_next) && (listrunner))
                        listrunner_next = g_list_next(listrunner);

        }

	g_list_free(listrunner);
	g_list_free(listrunner_next);

	refresh(area);

}

extern void
beam_draw_beam(GtkWidget *area, Staff_t *staff, Object_t *object, guint object_x, gint y, gint ynext)
{

	GdkGC *gc;

	guint thickness = 0;


	gc = gdk_gc_new(area->window);


	switch ( object->type ) {
        case EIGHTH:
                for ( thickness = 0; thickness < Spacings.Beams.bt; thickness++ ) {
                        gdk_draw_line(area->window, gc, 
                                      staff->start_x + object_x + 8, y - thickness,
                                      staff->start_x + object_x + Spacings.NotesRests.sa_quarter - 2, ynext - thickness);
                }

                break;
        case SIXTEENTH:
                for ( thickness = 0; thickness < Spacings.Beams.bt; thickness++ ) {
                        gdk_draw_line(area->window, gc, 
                                      staff->start_x + object_x + 8, y - 21 - thickness,
                                      staff->start_x + object_x + Spacings.NotesRests.sa_quarter - 2, ynext - 21 - thickness);
                }


                gdk_draw_line(area->window, gc, 
                              staff->start_x + object_x + 8, y - 21,
                              staff->start_x + object_x + 8, y - 25);
                gdk_draw_line(area->window, gc, 
                              staff->start_x + object_x + Spacings.NotesRests.sa_quarter - 2, y - 21,
                              staff->start_x + object_x + Spacings.NotesRests.sa_quarter - 2, y - 25);

                for ( thickness = 0; thickness < Spacings.Beams.bt; thickness++ ) {
                        gdk_draw_line(area->window, gc, 
                                      staff->start_x + object_x + 8, y - 25 - thickness,
                                      staff->start_x + object_x + Spacings.NotesRests.sa_quarter - 2, ynext - 25 - thickness);
                }

                break;
        }

} 
