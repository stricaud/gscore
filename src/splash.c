/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * splash.c
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * e-mail : toady@gscore.org
 * URL    : http://www.gscore.org
 *
 * The code inside this file is copyrighted by:
 * The GIMP -- an image manipulation program
 * Copyright (C) 1995 Spencer Kimball and Peter Mattis
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
#include <stdlib.h>

#include "gscore.h"
#include "constants.h"
#include "macros.h"

/* the following code comes from The Gimp */

static GtkWidget *splash = NULL;
static GtkWidget *label_action = NULL;
static GtkWidget *label_plugin = NULL;
static GtkWidget *progress = NULL;

static void splash_map (void);

void
splash_create(void)
{

	GtkWidget *vbox;

	GtkWidget *logo;
	GtkWidget *frame;
	GtkWidget *label;
	GtkWidget *align;

	gchar str[256];

	splash = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_type_hint (GTK_WINDOW (splash),
				  GDK_WINDOW_TYPE_HINT_SPLASHSCREEN);
	gtk_window_set_title (GTK_WINDOW (splash), "Gscore Startup");
	gtk_window_set_resizable (GTK_WINDOW (splash), FALSE);
	gtk_window_set_position (GTK_WINDOW (splash), GTK_WIN_POS_CENTER);

	g_signal_connect_swapped (splash, "delete_event",
				  G_CALLBACK (exit), GINT_TO_POINTER (0));

	gtk_window_set_auto_startup_notification (FALSE);
	g_signal_connect (splash, "map",
			  G_CALLBACK (splash_map),
			  NULL);

	frame = gtk_frame_new (NULL);
	gtk_frame_set_shadow_type (GTK_FRAME (frame), GTK_SHADOW_OUT);
	gtk_container_add (GTK_CONTAINER (splash), frame);

	vbox = gtk_vbox_new (FALSE, 4);
	gtk_container_add (GTK_CONTAINER (frame), vbox);

	/* add the pixmap */
	logo = gtk_image_new_from_file(get_file_from_data_dir(LOGO_GILBERT));

	align =  gtk_alignment_new (0.5, 0.5, 1.0, 1.0);
	gtk_box_pack_start (GTK_BOX (vbox), align, FALSE, TRUE, 0);
 
	gtk_container_add (GTK_CONTAINER (align), logo);

	g_snprintf(str, sizeof(str), "Gscore v %s\n\n", VERSION);
	label = gtk_label_new (str);
	gtk_box_pack_start(GTK_BOX(vbox), label, FALSE, TRUE, 1);

	label_action = gtk_label_new("Loading plugins");
	gtk_box_pack_start(GTK_BOX(vbox), label_action, FALSE, TRUE, 1);

	label_plugin = gtk_label_new("");
	gtk_box_pack_start(GTK_BOX(vbox), label_plugin, FALSE, TRUE, 1);

	progress = gtk_progress_bar_new();
	gtk_box_pack_start(GTK_BOX(vbox), progress, FALSE, TRUE, 1);

	gtk_widget_show_all(splash);

}

void
splash_destroy (void)
{

	if (splash) {
		gtk_widget_destroy (splash);
		splash = NULL;
	}

}


void
splash_update (const gchar *text_action,
               const gchar *text_plugin,
	       gdouble      scale)
{

	if (!splash)
		return;

	if (text_action)
		gtk_label_set_text (GTK_LABEL (label_action), text_action);

	if (text_plugin)
		gtk_label_set_text (GTK_LABEL (label_plugin), text_plugin);

	/* if there's no scale (ie. = no plugins), they are already loaded ;) */
	if (scale != 0)
		gtk_progress_bar_set_fraction (GTK_PROGRESS_BAR (progress),
					       1.0 / scale);
	else
		gtk_progress_bar_set_fraction (GTK_PROGRESS_BAR (progress),
					       1.0);

	while (gtk_events_pending ())
		gtk_main_iteration ();

}


/*  private functions  */

static void
splash_map (void)
{

  /*  Reenable startup notification after the splash has been shown
   *  so that the next window that is mapped sends the notification.  
   */
   gtk_window_set_auto_startup_notification (TRUE);

}

