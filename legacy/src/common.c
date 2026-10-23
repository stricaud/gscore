/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * common.c
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
#include <stdio.h>

#include "gscore.h"
#include "macros.h"
#include "score.h"
#include "common.h"

/* extern Score_t *get_score(void) */
/* { */
/*      return &Score; */
/* } */

/* void clean_statusbar(void) */
/* { */

/*   if (Score.message_id_statusbar != 0) { */
/*        gtk_statusbar_remove(GTK_STATUSBAR(Score.statusbar), */
/*                             Score.context_statusbar, */
/*                             Score.message_id_statusbar); */
/*   } */

/* } */


void refresh(GtkWidget *area)
{

	gtk_widget_queue_draw(area);

  /* when you change the size of the drawingarea, a refresh is done */
}

/* Object_t * get_object_list(gint staff, gint index) */
/* { */

/*   GList * tmp_list; */
/*   Object_t * object_returned = NULL; */

/*   for ( tmp_list = Score.Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*     { */
/*       Object_t * object; */


/*       object = (Object_t *)tmp_list->data; */

/*       if (object->index == index) */
/*         object_returned = (Object_t *)tmp_list->data; */
/*     } */


/*   return (Object_t *)object_returned; */
/* } */

void score_new_size(GtkWidget *area)
{
  Score_t *score = score_get_from_widget(area);
  
/*   if (Score.newsize == TRUE) */
/*     { */
      gtk_widget_set_size_request(area,
                                  score->width,
                                  score->height);
/*       Score.newsize = FALSE; */
/*     } */

}


void   
gw_message_error(gchar *message)
{
     GtkWidget *dialog;

     dialog = gtk_message_dialog_new(NULL, GTK_DIALOG_MODAL,
                                     GTK_MESSAGE_ERROR, GTK_BUTTONS_OK,
                                     message);
     gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_CENTER);
     if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_OK)
          gtk_widget_destroy(dialog);
}

void   
gw_message_info(gchar *message)
{
     GtkWidget *dialog;

     dialog = gtk_message_dialog_new(NULL, GTK_DIALOG_MODAL,
                                     GTK_MESSAGE_INFO, GTK_BUTTONS_OK,
                                     message);
     gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_CENTER);
     if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_OK)
          gtk_widget_destroy(dialog);
}

gboolean file_exists(const gchar *filename)
{
     FILE *file;

     gboolean value = FALSE;


     if ((file = fopen(filename,"r")) == NULL)
          value = FALSE;
     else
          value = TRUE;

     return value;
     
}

gboolean is_even(gint number)
{
     gboolean value;

     if (number % 2 == 0)
          value = TRUE;
     else
          value = FALSE;

     return value;

}

