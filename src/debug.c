/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * debug.c: function for debugging purpose
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
#include "debug.h"

void print_objects_staff(Score_t *score, gint staff)
{
        Staff_t *staff_data;
        GList *listrunner;


        staff_data = g_list_nth_data(score->Staff_list, staff);

        listrunner = g_list_first(staff_data->Object_list);
        while (listrunner) {
                        Object_t *object_data;

                        object_data = (Object_t *)listrunner->data;

                        printf("Object->id = %lu\n", object_data->id);
                        printf("Object->group_id = %lu\n", object_data->group_id);
                        printf("Object->type = %d\n", object_data->type);
                        printf("Object->nature = %d\n", object_data->nature);
                        printf("Object->accidentals = %d\n", object_data->accidentals);
                        printf("Object->pitch = %d\n", object_data->pitch);
                        printf("Object->tab_number = %d\n", object_data->tab_number);
                        printf("Object->is_selected = %d\n", object_data->is_selected);

                        listrunner = g_list_next(listrunner);
                }

        g_list_free(listrunner);

}

void print_object(Object_t *object)
{

        printf("\n*** Object:\n");
        printf("Object->id = %lu\n", object->id);
        printf("Object->group_id = %lu\n", object->group_id);
        printf("Object->type = %d\n", object->type);
        printf("Object->nature = %d\n", object->nature);
        printf("Object->accidentals = %d\n", object->accidentals);
        printf("Object->pitch = %d\n", object->pitch);
        printf("Object->tab_number = %d\n", object->tab_number);
        printf("Object->is_selected = %d\n", object->is_selected);

}

void print_staves_list(Score_t *score)
{
        GList *listrunner;

        listrunner = g_list_first(score->Staff_list);
        while (listrunner)
                {
                        Staff_t *staff_data;

                        staff_data = (Staff_t *)listrunner->data;

                        printf("Staff->nb_lines = %d\n", staff_data->nb_lines);
                        printf("Staff->space_btw_lines = %d\n", staff_data->space_btw_lines);
                        printf("Staff->is_selected = %d\n", staff_data->is_selected);
                        printf("Staff->key = %d\n", staff_data->key);
                        printf("Staff->key_signature = %d\n", staff_data->key_signature);
                        printf("Staff->time_signature[0] = %d\n", staff_data->time_signature[0]);
                        printf("Staff->time_signature[1] = %d\n", staff_data->time_signature[1]);
                        printf("Staff->time_signature[2] = %d\n", staff_data->time_signature[2]);
                        printf("Staff->extremity_begin_x = %f\n", staff_data->extremity_begin_x);
                        printf("Staff->extremity_begin_y = %f\n", staff_data->extremity_begin_y);
                        printf("Staff->start_x = %d\n", staff_data->start_x);
                        printf("Staff->current_x = %d\n", staff_data->current_x);
                        printf("Staff->midi_instrument = %d\n", staff_data->midi_instrument);

                        listrunner = g_list_next(listrunner);
                }

        g_list_free(listrunner);
}


/* void print_note_list(gpointer *data, gpointer *user_data) */
/* { */

/*         if (DEBUG == 0) */
/*                 { */
/*                         Note_t *note; */

/*                         note = (Note_t *)data; */

/*                         g_print("Note->id = %f\n", note->id); */
/*                         g_print("Note->x = %f\n", note->x); */
/*                         g_print("Note->selected = %d\n", note->selected); */
/*                         g_print("Note->pitch = %d\n", note->pitch); */
/*                         g_print("Note->type = %d\n", note->type); */
/*                         g_print("Note->accidental = %d\n", note->accidental); */
/*                         g_print("Note->up = %d\n", note->up); */
/*                         g_print("Note->beam = %d\n", note->beam); */
/*                         g_print("Note->last_beam = %d\n", note->last_beam); */
/*                         g_print("Note->invisible = %d\n\n", note->invisible); */
/*                 } */
/* } */

void debug_msg(gchar *message)
{
/* #ifdef DEBUG */
/*                 g_print(g_strconcat("*** DEBUG INFORMATION: ", message, "\n", NULL)); */
/* #endif */
}

static gboolean 
kill_tooltip(gpointer *data)
{

        gtk_widget_destroy(GTK_WIDGET(data));

        return FALSE;
}

void show_object_tooltip(Object_t *object)
{
        GtkWidget *tooltip_window;
        GtkWidget *label;
        GtkWidget *vbox;

        if ( ! object ) return;

        tooltip_window = gtk_window_new(GTK_WINDOW_POPUP);
        gtk_window_set_position(GTK_WINDOW(tooltip_window), GTK_WIN_POS_CENTER);
        
        vbox = gtk_vbox_new(FALSE, 0);
        gtk_container_add(GTK_CONTAINER(tooltip_window), vbox);

        label = gtk_label_new(g_strdup_printf("id: %lu\n", object->id));
        gtk_box_pack_start_defaults(GTK_BOX(vbox), label);
        label = gtk_label_new(g_strdup_printf("type: %d\n", object->type));
        gtk_box_pack_start_defaults(GTK_BOX(vbox), label);
        label = gtk_label_new(g_strdup_printf("pitch: %d\n", object->pitch));
        gtk_box_pack_start_defaults(GTK_BOX(vbox), label);
        label = gtk_label_new(g_strdup_printf("group id: %lu\n", object->group_id));
        gtk_box_pack_start_defaults(GTK_BOX(vbox), label);
        label = gtk_label_new(g_strdup_printf("is selected: %d\n", object->is_selected));
        gtk_box_pack_start_defaults(GTK_BOX(vbox), label);

        g_timeout_add(3000, (GSourceFunc)kill_tooltip, tooltip_window);

        gtk_widget_show_all(tooltip_window);
}
