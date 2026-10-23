/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * view.c
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
#include "score.h"
#include "view.h"

extern void
on_view_measure_number_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;
	

/* 	widget =  glade_xml_get_widget (gladexml, "view_measure_number"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->measure_number = TRUE;
		refresh(area);
	} else {
                display->measure_number = FALSE;
		refresh(area);
	}

}

extern void
on_view_instruments_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;

/* 	widget =  glade_xml_get_widget (gladexml, "view_instruments"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
	if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->instruments = TRUE;
		refresh(area);
	} else {
                display->instruments = FALSE;
		refresh(area);
	}

}


extern void
on_view_tempo_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;

/* 	widget =  glade_xml_get_widget (gladexml, "view_tempo"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->tempo = TRUE;
		refresh(area);
	} else {
                display->tempo = FALSE;
		refresh(area);
	}

}

extern void
on_view_clefs_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;

/* 	widget =  glade_xml_get_widget (gladexml, "view_clefs"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);

        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->clefs = TRUE;
		refresh(area);
	} else {
                display->clefs = FALSE;
		refresh(area);
	}

}

extern void
on_view_score_expressions_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;
	
/* 	widget =  glade_xml_get_widget (gladexml, "view_score_expressions"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);

        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->score_expressions = TRUE;
		refresh(area);
	} else {
                display->score_expressions = FALSE;
		refresh(area);
	}

}

extern void
on_view_barlines_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;
	
/* 	widget =  glade_xml_get_widget (gladexml, "view_barlines"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->barlines = TRUE;
		refresh(area);
	} else {
                display->barlines = FALSE;
		refresh(area);
	}

}

extern void
on_view_ending_bar_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;
	
/* 	widget =  glade_xml_get_widget (gladexml, "view_ending_bar"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->ending_bar = TRUE;
		refresh(area);
	} else {
                display->ending_bar = FALSE;
		refresh(area);
	}

}

extern void
on_view_key_signature_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;
	
/* 	widget =  glade_xml_get_widget (gladexml, "view_key_signature"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->key_signature = TRUE;
		refresh(area);
	} else {
                display->key_signature = FALSE;
		refresh(area);
	}

}

extern void
on_view_time_signature_activate(GtkWidget *widget)
{

/*         GtkWidget *widget; */
	Display_t *display;
	GtkWidget *area;
	
/* 	widget =  glade_xml_get_widget (gladexml, "view_time_signature"); */
	display = score_get_display_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
        if ( gtk_check_menu_item_get_active((GtkCheckMenuItem *)widget) ) {
                display->time_signature = TRUE;
		refresh(area);
	} else {
                display->time_signature = FALSE;
		refresh(area);
	}

}

extern void
on_sw_score_headers_menuitem_activate(GtkWidget *widget)
{

/* 	GtkWidget *widget; */


/* 	widget = glade_xml_get_widget(gladexml, "sw_score_headers_da"); */

	gtk_widget_show(widget);

}

/* void */
/* toolbar_notes(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */
/*   GtkWidget * Bar; */
/*   GtkWidget * hbox; */
/*   GtkWidget * ic_button; */
/*   GtkWidget * pixmap_wid; */

/*   gint i; */
/*   gchar *note_bar[]={"cursors/ic_rondecarree.xpm", "cursors/ic_ronde.xpm", "cursors/ic_blanche.xpm",  */
/*                      "cursors/ic_noire.xpm", "cursors/ic_croche.xpm", "cursors/ic_doublecroche.xpm", */
/*                      "cursors/ic_triplecroche.xpm", "cursors/ic_croche4.xpm", "cursors/ic_croche5.xpm"}; */

/*   // window  */
/*   dialog = gtk_window_new(GTK_WINDOW_TOPLEVEL); */
/*   g_signal_connect(GTK_OBJECT(dialog), "delete_event", */
/* 		     G_CALLBACK(gtk_widget_destroy), NULL); */

/*   g_signal_connect(GTK_OBJECT(dialog), "destroy", */
/* 		     G_CALLBACK(gtk_widget_destroy), NULL); */

/*   /\* */
/*   g_signal_connect(GTK_OBJECT(dialog),g_object_weak_ref(), */
/* 		     G_CALLBACK(gtk_widget_destroy), NULL); */
/*   *\/ */
/*   // bar  */
/*   Bar = gtk_handle_box_new(); */
/*   gtk_container_add(GTK_CONTAINER(dialog), Bar);   */

/*   // hbox  */
/*   hbox = gtk_hbox_new(FALSE, 0); */
/*   gtk_container_add(GTK_CONTAINER(Bar), hbox); */

/*   // we put buttons in bar  */

/*   for ( i=0 ; i<9 ; i++ )  */
/*   { */
/* /\*   pixmap = gdk_pixmap_create_from_xpm_d(dialog->window, &mask, &c, *\/ */
/* /\* 					note_bar[i]); *\/ */
/* /\*   pixmapwid = gtk_pixmap_new(pixmap, mask); *\/ */
/*        pixmap_wid = gtk_image_new_from_file(note_bar[i]); */
/*        ic_button = gtk_button_new(); */
/*        gtk_container_add(GTK_CONTAINER(ic_button), pixmap_wid); */
/*        gtk_box_pack_start(GTK_BOX(hbox), ic_button, FALSE, FALSE, 0); */
/*   } */


/*   gtk_widget_show_all(dialog); */

/* } */


/* extern void */
/* view_measures_number(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */

/* /\*      if (score->Staff[get_staff_selected()].View.measures_number == TRUE) *\/ */
/* /\*           score->Staff[get_staff_selected()].View.measures_number = FALSE; *\/ */
/* /\*      else *\/ */
/* /\*           score->Staff[get_staff_selected()].View.measures_number = TRUE; *\/ */

/*      refresh(); */

/* } */
