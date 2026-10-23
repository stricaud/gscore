/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * edit.c
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

#include <gtk/gtk.h>

#include "edit.h"
#include "constants.h"
#include "common.h"
#include "gscore.h"
#include "position.h"
#include "score.h"
#include "staff.h"
#include "objects.h"
#include "position.h"

extern Object_t *
measure_get_object(Staff_t *staff, guint measure_nb)
{
        Object_t *object_data;

        GList *objectrunner = NULL;

        guint count_measure = 0;

/*         g_print("measure_nb = %d\n", measure_nb); */

        objectrunner = g_list_first(staff->Object_list);

        while ( objectrunner ) {
                object_data = (Object_t *)objectrunner->data;

                if ( ! object_data ) {
                        g_warning("Gscore: no object can be retrieved");
                        return NULL;
                }

                switch(object_data->type) {
                case BARLINE:
                case BARLINE_SINGLE:
                case BARLINE_DOUBLE:
                case BARLINE_OPENREPEAT:
                case BARLINE_CLOSEREPEAT:
                case BARLINE_OPENCLOSEREPEAT:
                case BARLINE_OPEN:
                case BARLINE_CLOSE:
                        if (count_measure == measure_nb) {
                                return object_data;
                        } else {
                                count_measure++;
                        }
                        break;
                default:
                        break;
                }

                objectrunner = g_list_next(objectrunner);
        }

        return NULL;
}

extern void 
edit_jump_to_measure_widget(GtkButton *widget, gpointer user_data)
{
        Score_t *score = score_get_from_widget(GTK_WIDGET(widget));
        Staff_t *staff_data;
        Object_t *object_data;

        GList *objectrunner = NULL;

        guint nb_measures = 0;

        GtkWidget *dialog;
        GtkWidget *hbox;
        GtkWidget *label;
        GtkWidget *spinbutton;

        gint response;

        Object_t *MO;           /* Measure Object */
        gdouble x;

        /* First we need to know how many measures are on the staff selected */
        staff_data = g_list_nth_data(score->Staff_list, get_staff_selected(score));
        if ( ! staff_data ) {
                g_warning("Gscore: no staff can be retrieved");
                return;
        }

        objectrunner = g_list_first(staff_data->Object_list);

        while ( objectrunner ) {
                object_data = (Object_t *)objectrunner->data;

                if ( ! object_data ) {
                        g_warning("Gscore: no object can be retrieved");
                        return;
                }

                switch(object_data->type) {
                case BARLINE:
                case BARLINE_SINGLE:
                case BARLINE_DOUBLE:
                case BARLINE_OPENREPEAT:
                case BARLINE_CLOSEREPEAT:
                case BARLINE_OPENCLOSEREPEAT:
                case BARLINE_OPEN:
                case BARLINE_CLOSE:
                        nb_measures++;
                        break;
                default:
                        break;
                }

                objectrunner = g_list_next(objectrunner);
        }

        /* Now that we have the number of measures in the selected staff, we can set the spinbutton value */
        dialog = gtk_dialog_new_with_buttons("Jump to measure", NULL,
                                             GTK_DIALOG_MODAL, 
                                             GTK_STOCK_CANCEL,
                                             GTK_RESPONSE_REJECT,
                                             GTK_STOCK_OK,
                                             GTK_RESPONSE_ACCEPT,
                                             NULL);

        hbox = gtk_hbox_new(FALSE, 10);
        gtk_box_pack_start(GTK_BOX(GTK_DIALOG(dialog)->vbox), hbox,
                           TRUE, TRUE, 0);

        label = gtk_label_new("Jump to measure: ");
        gtk_box_pack_start_defaults(GTK_BOX(hbox), label);

        spinbutton = gtk_spin_button_new_with_range(1, nb_measures+1, 1);
        gtk_box_pack_start_defaults(GTK_BOX(hbox), spinbutton);

        gtk_widget_show_all(dialog);

        response = gtk_dialog_run (GTK_DIALOG (dialog));

        if (response == GTK_RESPONSE_ACCEPT) {

                MO = measure_get_object(staff_data, gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(spinbutton)) - 2);

                if ( ! MO ) {
                        g_warning("Gscore: Cannot get the measure object.");
                        position_set_adjustment_x(GTK_WIDGET(widget), staff_data->extremity_begin_x);
                } else {
                        x = object_get_x(staff_data, MO);
                        if ( x < 0 )
                                printf("Error getting the x position of the object");
                        else
                                position_set_adjustment_x(GTK_WIDGET(widget), x);
                }

                refresh(score_get_area_from_widget(GTK_WIDGET(widget)));

        }

        gtk_widget_destroy (dialog);

}

void edit_undo(void)
{
     
}

/* extern void  */
/* edit_cut(void) */
/* { */
/*      GList *listrunner; */
/*      gint i; */
/*      gboolean something_selected = FALSE; */

/*      listrunner = g_list_first(Score.Staff[Score.staff_selected].Note_list); */
     
/*      while (listrunner) */
/*      { */
/* 	     Note_t *note; */
	     
/* 	     note = (Note_t *)listrunner->data; */
	     
/* 	     if (note->selected) */
/* 	     { */

/* 		     something_selected = TRUE; */

/* /\* 		     Clipboard.note_list = g_malloc(sizeof(Note_t)); *\/ */
		     
/* /\* 		     Clipboard.Note_list = g_list_append(Clipboard.Note_list,  *\/ */
/* /\* 							 note); *\/ */

/* /\*   *\/ */
/* 		     Score.Staff[Score.staff_selected].current_x -= get_spacing_for_object(note->type); */

/* /\* 		     Score.Staff[Score.staff_selected].Note_list = g_list_remove_all(Score.Staff[Score.staff_selected].Note_list, (Note_t *)note); *\/ */
		     	     
/* 	     } */
	     
/* 	     listrunner = g_list_next(listrunner); */
/*      } */

/*      g_list_free(listrunner); */

/*      if (something_selected) */
/*           refresh(); */
/*      else */
/*           gw_message_info(_("You should select something if you want to cut")); */

/* } */

/* void */
/* edit_go_to_measure(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */
/*      GtkWidget * dialog; */
/*      GtkWidget * spinbutton; */

/*      GtkAdjustment * adj; */
     
          
/*      dialog = gtk_dialog_new_with_buttons(_("Go to measure"), NULL, */
/*                                           GTK_DIALOG_MODAL, */
/*                                           GTK_STOCK_CANCEL, GTK_RESPONSE_REJECT, */
/*                                           GTK_STOCK_OK, GTK_RESPONSE_ACCEPT, */
/*                                           NULL); */
     
/*      g_signal_connect(G_CALLBACK(dialog), "destroy", */
/*                       G_CALLBACK(gtk_widget_destroyed), */
/*                       &dialog); */
     

/*      adj = GTK_ADJUSTMENT(gtk_adjustment_new(Score.Staff[get_staff_selected()].measure_number, */
/*                                              1, Score.Staff[get_staff_selected()].total_measures, */
/*                                              1.0, 10.0, 10.0 )); */
/*      spinbutton = gtk_spin_button_new(adj, 1.0, 0); */
/*      gtk_box_pack_start_defaults(GTK_BOX(GTK_DIALOG(dialog)->vbox), spinbutton); */
/*      gtk_widget_show(spinbutton); */

/*      /\* Execute the OK button action *\/ */
/*      if (gtk_dialog_run(GTK_DIALOG(dialog))==GTK_RESPONSE_ACCEPT) */
/*      { */
/*           Score.tempo = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(spinbutton)); */
/*      } */
/*      gtk_widget_destroy(dialog); */

/* } */

