/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * chords.c
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
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

#include <gtk/gtk.h>

#include "gscore.h"
#include "position.h"
#include "constants.h"
#include "objects.h"
#include "chords.h"


gboolean is_chord(Staff_t *staff, Object_t *object)
{
        Object_t *object_data;
	GList *listrunner_object;
        

	listrunner_object = g_list_first(staff->Object_list);
	while ( listrunner_object ) {
		object_data = (Object_t *)listrunner_object->data;
		
		if (object_data) {
                        if (object->id == object_data->group_id)
                                return TRUE;
		}
		
		listrunner_object = g_list_next(listrunner_object);
	}

        return FALSE;

}

guint make_chord(Staff_t *staff, 
                 gulong objectid, gulong group_id, gint pitch, 
                 gboolean *stemup, gboolean *notehead_left)
{
	GList *listrunner_object;
	guint retval = 0;
	guint xpos = 0;
	gint y = 0;

	gdouble extremity_end_y = 0;
	gint average = 0;

	* stemup = TRUE;
        * notehead_left = TRUE;
	
	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;
	average = (extremity_end_y - staff->extremity_begin_y) / 2;

	listrunner_object = g_list_first(staff->Object_list);
	while ( listrunner_object ) {
		Object_t *object;
		object = (Object_t *)listrunner_object->data;
		
		if (object) {
			if (object->id == group_id) { /* Then, the chord is made with this note */
				retval = xpos;
				
				y = get_y_from_position(staff->key, staff->extremity_begin_y, object->pitch);
                                
				if (y < staff->extremity_begin_y + average) {
					* stemup = FALSE;
				} else {
					* stemup = TRUE;
				}
			} else {
                                /* FIXME: this doesn't handle the case where the main object->id has been deleted */
                        }

                        if (object->id != objectid) {
                                if ( ( ++pitch == object->pitch ) || ( --pitch == object->pitch ) )
                                        {
                                                * notehead_left = FALSE; 
                                        }
                        }

                        if (object->group_id == 0)
                                xpos += object_get_spacing(object->type);
		}

		
		listrunner_object = g_list_next(listrunner_object);
	}


	return retval;

}

