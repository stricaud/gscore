/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * help.c
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2004 Sebastien Tricaud
 * e-mail : toady@gscore.org
 * URL    : http://www.gscore.org
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; version 2 of the License.
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

#include <stdlib.h>
#include <gtk/gtk.h>

#include "gscore.h"
#include "macros.h"

#define LICENSE_TEXT "\
This program is free software; you can redistribute it and/or modify\n\
it under the terms of the GNU General Public License as published by\n\
the Free Software Foundation; version 2 of the License.\n\n\
\
This program is distributed in the hope that it will be useful,\n\
but WITHOUT ANY WARRANTY; without even the implied warranty of\n\
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the\n\
GNU General Public License for more details.\n\n\
\
You should have received a copy of the GNU General Public License\n\
along with this program; if not, write to the Free Software\n\
Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA 02111-1307, USA.\n\
"

extern void
on_help_about_activate(void)
{
	GtkWidget *about;
	GdkPixbuf *logo;

	const gchar *authors[] = {"Sebastien Tricaud", 
				  "Jerome Marrec", 
				  "Frederic Motte", 
				  "Alastair Porter", 
				  NULL};
	
	const gchar *artists[] = {"Damien Cante (logos)", 
				  "Audrey Maurin (pixmaps from lilypond)",
				  NULL};

	gchar *filename;
	filename = get_file_from_data_dir("pixmaps/gscore.xpm");
	logo = gtk_image_get_pixbuf((GtkImage *)gtk_image_new_from_file(filename));

	about = gtk_about_dialog_new();

	gtk_about_dialog_set_name(GTK_ABOUT_DIALOG(about), "Gscore");
	gtk_about_dialog_set_version(GTK_ABOUT_DIALOG(about), VERSION);
	gtk_about_dialog_set_logo(GTK_ABOUT_DIALOG(about), logo);
	gtk_about_dialog_set_copyright(GTK_ABOUT_DIALOG(about), "Copyright (C) Sebastien Tricaud 2001-2005");
	gtk_about_dialog_set_license(GTK_ABOUT_DIALOG(about), LICENSE_TEXT);
	gtk_about_dialog_set_website(GTK_ABOUT_DIALOG(about), "http://www.gscore.org");
	gtk_about_dialog_set_comments(GTK_ABOUT_DIALOG(about), "A musical score editor");
	gtk_about_dialog_set_authors(GTK_ABOUT_DIALOG(about), authors);
	gtk_about_dialog_set_artists(GTK_ABOUT_DIALOG(about), artists);

	gtk_widget_show(about);

}

/* void */
/* help_help_callback(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */
/*      const gchar *helpdir;  */
/*      gchar *helpindex = NULL, *command;  */

/*      helpindex = get_file_from_data_dir("doc/index.html"); */

/*      g_print("helpindex = %s\n", helpindex); */
     
/*      command = getenv("BROWSER"); */
/*      command = g_strdup_printf("%s 'file://%s' &", command ? command : "mozilla", helpindex); */
/*      system(command); */

/* } */

void
help_report_bug(gpointer callback_data, guint callback_action, GtkWidget *widget)
{
	GtkWidget *dialog;

	dialog = gtk_message_dialog_new(NULL, GTK_DIALOG_MODAL,
					GTK_MESSAGE_INFO, GTK_BUTTONS_OK,
					"Visit http://www.gscore.org/bugs\n\nPlease specify:\n- The version of gscore you are using\n- Your operating system\n- How the bug happens\n- If you can reproduce it\n\nThanks.");
	gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_CENTER);
	if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_OK)
		gtk_widget_destroy(dialog);
}

