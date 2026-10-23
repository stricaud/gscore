/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * score_callbacks.c
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
#include <unistd.h>
#include <gettext.h>
#include <glade/glade-xml.h>

#include "gscore.h"
#include "score_callbacks.h"
#include "score.h"
#include "common.h"

#define OK     "OK"
#define CANCEL "Cancel"


static GtkWidget *get_toplevel(GtkWidget *widget)
{
  
        GladeXML *xml = glade_get_widget_tree(widget);

        if(xml == gladexml) {
                return glade_xml_get_widget(xml, "main_window");
        } else {
                return  glade_xml_get_widget(xml, "score_window");
        }
}

extern void
set_tempo(GtkButton *widget, gpointer user_data)
{
        GtkWidget *spinbutton;
        GtkWidget *entry = NULL;
	Score_t *score;
	GtkWidget *area;
	
/* 	spinbutton =  glade_xml_get_widget (gladexml, "tempo_spinbutton"); */
/* 	entry =  glade_xml_get_widget (gladexml, "tempo_entry"); */
	score = score_get_from_widget(GTK_WIDGET(widget));
	area = score_get_area_from_widget(GTK_WIDGET(widget));
        spinbutton = score_get_tempo_spinbutton_from_widget(GTK_WIDGET(widget));
        entry = score_get_tempo_entry_from_widget(GTK_WIDGET(entry));
	
        score->tempo = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(spinbutton));

/* 	radiobutton =  glade_xml_get_widget (gladexml, "tempo_quarter_rb"); */
/*         g_print("state: %d\n", gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radiobutton))); */

        score->tempo_text = g_string_new(gtk_entry_get_text((GtkEntry *)entry));

	refresh(area);
}

extern void 
on_set_header_activate_default(GtkButton *widget, gpointer user_data)
{
	GtkWidget *title;
	GtkWidget *subtitle;
	GtkWidget *composer;
	Score_t *score;
	
	title    = score_get_title_entry_from_widget(GTK_WIDGET(widget));
	subtitle = score_get_subtitle_entry_from_widget(GTK_WIDGET(widget));
	composer = score_get_composer_entry_from_widget(GTK_WIDGET(widget));
	score    = score_get_from_widget(GTK_WIDGET(widget));

	gtk_entry_set_text(GTK_ENTRY(title), score->Identity->title->str);
	gtk_entry_set_text(GTK_ENTRY(subtitle), score->Identity->subtitle->str);
	gtk_entry_set_text(GTK_ENTRY(composer), score->Identity->composer->str);

}

extern void 
score_set_header_ok(GtkButton *widget, gpointer user_data)
{
	GtkWidget *title;
	GtkWidget *subtitle;
	GtkWidget *composer;
	Score_t *score;

/* 	title = glade_xml_get_widget(gladexml, "title_entry"); */
/* 	subtitle = glade_xml_get_widget(gladexml, "subtitle_entry"); */
/* 	composer = glade_xml_get_widget(gladexml, "composer_entry"); */
	title    = score_get_title_entry_from_widget(GTK_WIDGET(widget));
	subtitle = score_get_subtitle_entry_from_widget(GTK_WIDGET(widget));
	composer = score_get_composer_entry_from_widget(GTK_WIDGET(widget));
	score = score_get_from_widget(GTK_WIDGET(title));

	score->Identity->title = g_string_assign(score->Identity->title, 
                                                 gtk_entry_get_text(GTK_ENTRY(title)));
	score->Identity->subtitle = g_string_assign(score->Identity->subtitle, 
                                                    gtk_entry_get_text(GTK_ENTRY(subtitle)));
	score->Identity->composer = g_string_assign(score->Identity->composer, 
                                                    gtk_entry_get_text(GTK_ENTRY(composer)));

}

extern gboolean 
score_activate_callback(GtkWidget *widget)
{
        static const char *props[] = {
                "score", "area", "selection", "cursor", "display",
                NULL};
        int i;
        GtkWidget *main_window;
        GtkWidget *score_window;
  
        main_window = glade_xml_get_widget(gladexml, "main_window");
        score_window = get_toplevel(widget);
  
        for(i=0; props[i] != NULL; ++i) {
                g_object_set_data(G_OBJECT(main_window), props[i],
                                  g_object_get_data(G_OBJECT(score_window),
                                                    props[i]));
    
        }
        return TRUE;
  
}

