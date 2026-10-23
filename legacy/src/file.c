/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * file.c
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
#include <string.h>

#include "file.h"
#include "debug.h"
#include "gscore.h"
#include "common.h"
#include "plugin.h"
#include "constants.h"
#include "score.h"

#include "gettext.h"

static void
open_set_extension(GtkObject *item)
{
/* 	DiaImportFilter *ifilter = gtk_object_get_user_data(item); */
/* 	GString *s; */
/* 	const gchar *text = gtk_entry_get_text(GTK_ENTRY(GTK_FILE_SELECTION(opendlg) */
/* 							 ->selection_entry)); */
/* 	const gchar *last_dot = strrchr(text, '.'); */
	
/* 	if (!ifilter || last_dot == text || text[0] == '\0' || */
/* 	    ifilter->extensions[0] == NULL) */
/* 		return; */
	
/* 	s = g_string_new(text); */
/* 	if (last_dot) */
/* 		g_string_truncate(s, last_dot-text); */
/* 	g_string_append(s, "."); */
/* 	g_string_append(s, ifilter->extensions[0]); */
/* 	gtk_entry_set_text(GTK_ENTRY(GTK_FILE_SELECTION(opendlg)->selection_entry), */
/* 			   s->str); */
/* 	g_string_free (s, TRUE); */
}

static GtkWidget *
create_open_save_menu(void)
{
	GtkWidget *menu;
	GtkWidget *item;

	GscorePlugin *plugin;
	plugin_pnf *ppnf;
	gchar *full_filename;

	gint i;
	
	menu = gtk_menu_new();
	item = gtk_menu_item_new_with_label("By extension");
	gtk_container_add(GTK_CONTAINER(menu), item);
	gtk_widget_show(item);
	item = gtk_menu_item_new();
	gtk_container_add(GTK_CONTAINER(menu), item);
	gtk_widget_show(item);


	for ( i = 0; i < nb_plugins; i++) {
		ppnf = g_list_nth_data(plugins_list, i);
		full_filename = g_strconcat(ppnf->path, G_DIR_SEPARATOR_S, ppnf->filename, NULL);
		
		
		GSCORE_PLUGIN_INIT(plugin);
		plugin = gscore_plugin_get(full_filename);

		item = gtk_menu_item_new_with_label(g_strconcat(plugin->info->name, " (.",plugin->filter->extensions[0], ") " , NULL));
		g_signal_connect(GTK_OBJECT(item), "activate",
				 GTK_SIGNAL_FUNC(open_set_extension), NULL);
		
		gtk_container_add(GTK_CONTAINER(menu), item);
		gtk_widget_show(item);

	}

  return menu;
}

extern void
import_score(GtkButton *button, gpointer user_data)
{
	const gchar *filename;
	const gchar *extension;
/* 	Display_t *display_save; */
/* 	GtkWidget *area_save; */
	

	GtkWidget *filechooser;

	GscorePlugin *plugin;
	Score_t     *pi;

/* 	display_save = Score.Display; */
/* 	area_save = Score.area; */

	filechooser = (GtkWidget *)user_data;

	filename = gtk_entry_get_text(GTK_ENTRY(GTK_FILE_SELECTION(filechooser)
						->selection_entry));

	printf("File choosen:%s\n", filename);

	extension = strrchr(filename, '.');

	if ( ! extension ) {
		gw_message_error(_("The file has no extension,\n unable to guess which plugin to use"));
		return;
	}

	filename = gtk_file_selection_get_filename(GTK_FILE_SELECTION(filechooser));

	GSCORE_PLUGIN_INIT(plugin);
	plugin = gscore_plugin_get_from_extension(extension);

	if ( ! plugin ) {
	    gw_message_error(_("No plugin available for this file,\nfeel free to write one.\n\nVisit www.gscore.org/wiki for more informations"));
	    return;
	  }

	plugin->filter->import(&pi, filename, NULL);

/*         Score = *pi; */
/* 	Score.Display = display_save; */
/* 	Score.area = area_save; */
	score_create_window(pi);
	
/* 	g_print("Test: %d\n", pi->tempo); */

}

extern void
file_open(GtkWidget *widget)
{
	GtkWidget *frame;
	GtkWidget *hbox;
	GtkWidget *omenu;
	GtkWidget *label;

	widget = glade_xml_get_widget(gladexml, "file_open");
	gtk_widget_show(widget);


	frame = gtk_frame_new("Open options");
	gtk_frame_set_shadow_type(GTK_FRAME(frame), GTK_SHADOW_ETCHED_IN);
	gtk_widget_show(frame);

	hbox = gtk_hbox_new(TRUE, 1);
	gtk_container_set_border_width(GTK_CONTAINER(hbox), 5);
	gtk_container_add(GTK_CONTAINER(frame), hbox);
	gtk_widget_show(hbox);

	label = gtk_label_new ("Determine file type:");
	gtk_box_pack_start (GTK_BOX (hbox), label, FALSE, TRUE, 0);
	gtk_widget_show (label);


	omenu = gtk_option_menu_new();
	gtk_box_pack_start(GTK_BOX(hbox), omenu, TRUE, TRUE, 0);
	gtk_widget_show(omenu);

	gtk_option_menu_set_menu(GTK_OPTION_MENU(omenu), create_open_save_menu());

	gtk_box_pack_end(GTK_BOX (GTK_FILE_SELECTION(widget)->main_vbox),
			 frame, FALSE, FALSE, 5);

	g_signal_connect(GTK_OBJECT(GTK_FILE_SELECTION(widget)->ok_button), "clicked",
			 GTK_SIGNAL_FUNC(import_score), widget);
	
}

extern void
export_score(GtkButton *button, gpointer user_data)
{
	GtkWidget *score_window_widget;
	Score_t *score;

	const gchar *filename;

	const gchar *extension;


	GtkWidget *filechooser;

	GscorePlugin *plugin;

	score = score_get_from_widget(GTK_WIDGET(button));
	
	filechooser = (GtkWidget *)user_data;

	filename = gtk_entry_get_text(GTK_ENTRY(GTK_FILE_SELECTION(filechooser)
						->selection_entry));

	extension = strrchr(filename, '.');

	if ( ! extension ) {
		gw_message_error("No extension chosen (NULL)");
		return;
	}

	filename = gtk_file_selection_get_filename(GTK_FILE_SELECTION(filechooser));

	GSCORE_PLUGIN_INIT(plugin);
	plugin = gscore_plugin_get_from_extension(extension);

	if ( ! plugin ) {
		g_print("Couldn't get the plugin structure from its extension (%s)\n", extension);
		return;
	}

/* 	printf("Extension: %s\n", extension); */
/* 	printf("plugin name = %s\n", plugin->info->name); */
	plugin->filter->export(score, filename, NULL);



	/* Set the title of the score window */
/* 	score_window_widget = glade_xml_get_widget(gladexml, "score_window"); */
	score_window_widget = gtk_widget_get_toplevel(GTK_WIDGET(button));
	
/* 	filename = gtk_entry_get_text(GTK_ENTRY(GTK_FILE_SELECTION(filechooser)->selection_entry)); */

	gtk_window_set_title(GTK_WINDOW(score_window_widget), filename);


/* 	plugin->filter->export(gscore_to_plugin(&Score), text, NULL); */
}

extern void
file_save_as(GtkWidget *widget)
{

	GtkWidget *frame;
	GtkWidget *hbox;
	GtkWidget *omenu;
	GtkWidget *label;


	widget = glade_xml_get_widget(gladexml, "file_save_as");
	gtk_widget_show(widget);

	frame = gtk_frame_new("Save as");
	gtk_frame_set_shadow_type(GTK_FRAME(frame), GTK_SHADOW_ETCHED_IN);
	gtk_widget_show(frame);

	hbox = gtk_hbox_new(TRUE, 1);
	gtk_container_set_border_width(GTK_CONTAINER(hbox), 5);
	gtk_container_add(GTK_CONTAINER(frame), hbox);
	gtk_widget_show(hbox);

	label = gtk_label_new ("Determine file type:");
	gtk_box_pack_start (GTK_BOX (hbox), label, FALSE, TRUE, 0);
	gtk_widget_show (label);


	omenu = gtk_option_menu_new();
	gtk_box_pack_start(GTK_BOX(hbox), omenu, TRUE, TRUE, 0);
	gtk_widget_show(omenu);

	gtk_option_menu_set_menu(GTK_OPTION_MENU(omenu), create_open_save_menu());

	gtk_box_pack_end(GTK_BOX (GTK_FILE_SELECTION(widget)->main_vbox),
			 frame, FALSE, FALSE, 5);


	/* Export the score (call the plugin */
	g_signal_connect(GTK_OBJECT(GTK_FILE_SELECTION(widget)->ok_button), "clicked",
			 GTK_SIGNAL_FUNC(export_score), widget);

	
}


void   
exit_gscore(gpointer callback_data, guint callback_action, GtkWidget *widget)
{
  gtk_main_quit();

/*   GtkWidget *dialog; */

/*   dialog = gtk_message_dialog_new(GTK_WINDOW(callback_data), GTK_DIALOG_MODAL, */
/* 				  GTK_MESSAGE_QUESTION, GTK_BUTTONS_YES_NO, */
/* 				  "This score isn't saved.\n" */
/* 				    "Do you really want to quit now?"); */
/*   gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_MOUSE); */
/*   if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_YES) */
/*     gtk_main_quit(); */
/*   gtk_widget_destroy(dialog); */

}
