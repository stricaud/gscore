/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * insert.c
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
#include <sys/types.h>
#include <sys/stat.h>
#include <unistd.h>
#include <string.h>

#include "gscore.h"
#include "draw.h"
#include "constants.h"
#include "insert.h"
#include "draw_barline.h"
#include "staff.h"
#include "common.h"
#include "macros.h"
#include "score.h"

void on_set_key_activate(void)
{

}

extern void 
insert_text_callback(GtkButton *widget, gpointer user_data)
{
        GtkWidget *entry;
        const gchar *data;
/*         Score_t *score = score_get_from_widget(widget); */

/*         GtkWidget *entry = score_get_insert_text_entry_from_widget(widget); */
/*         GtkWidget *font = score_get_insert_text_font_from_widget(widget); */
/*         GtkWidget *color = score_get_insert_text_color_from_widget(widget); */

        entry = glade_xml_get_widget(gladexml, "insert_text_entry");
        data = gtk_entry_get_text(GTK_ENTRY(entry));
        g_print("Text we got: %s\n", data);
        entry = score_get_insert_text_entry_from_widget(GTK_WIDGET(widget));
        data = gtk_entry_get_text(GTK_ENTRY(entry));
        g_print("Text we got: %s\n", data);
}

/* globals vars */
/* GtkWidget *dialog; */
/* GtkWidget *KeySignature; */
/* GtkWidget *table; */
/* GtkWidget *vbox, *hbox; */


/* gint barline_drawsingle(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      GdkGC * gc; */

/*      gc = gdk_gc_new(drawingarea->window); */

/*      gdk_draw_line(drawingarea->window, gc,  */
/*                    5, 0, */
/*                    5, 32); */

/*      return 0; */
/* } */

/* gint barline_drawdouble(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      GdkGC * gc; */

/*      gc = gdk_gc_new(drawingarea->window); */

/*      gdk_draw_line(drawingarea->window, gc,  */
/*                    5, 0, */
/*                    5, 32); */

/*      gdk_draw_line(drawingarea->window, gc,  */
/*                    8, 0, */
/*                    8, 32); */

/*      return 0; */
/* } */

/* gint barline_drawopenrepeat(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      GdkGC * gc; */
/*      gint i; */

/*      gc = gdk_gc_new(drawingarea->window); */

/*      for (i=0; i != 4; i++) { */
/*           gdk_draw_line(drawingarea->window, gc,  */
/*                         5+i, 0, */
/*                         5+i, 32); */
/*      } */

/*      gdk_draw_line(drawingarea->window, gc,  */
/*                    11, 0, */
/*                    11, 32); */

/*      gdk_draw_point (drawingarea->window, gc, 14, 13); */
/*      gdk_draw_point (drawingarea->window, gc, 14, 14); */
/*      gdk_draw_point (drawingarea->window, gc, 15, 13); */
/*      gdk_draw_point (drawingarea->window, gc, 15, 14); */

/*      gdk_draw_point (drawingarea->window, gc, 14, 19); */
/*      gdk_draw_point (drawingarea->window, gc, 14, 20); */
/*      gdk_draw_point (drawingarea->window, gc, 15, 19); */
/*      gdk_draw_point (drawingarea->window, gc, 15, 20); */

/*      return 0; */

/* } */

/* gint barline_drawcloserepeat(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */

/*      GdkGC * gc; */
/*      gint i; */

/*      gc = gdk_gc_new(drawingarea->window); */

/*      for (i=0; i != 4; i++) { */
/*           gdk_draw_line(drawingarea->window, gc,  */
/*                         12+i, 0, */
/*                         12+i, 32); */
/*      } */

/*      gdk_draw_line(drawingarea->window, gc,  */
/*                    9, 0, */
/*                    9, 32); */

/*      gdk_draw_point (drawingarea->window, gc, 5, 13); */
/*      gdk_draw_point (drawingarea->window, gc, 5, 14); */
/*      gdk_draw_point (drawingarea->window, gc, 6, 13); */
/*      gdk_draw_point (drawingarea->window, gc, 6, 14); */

/*      gdk_draw_point (drawingarea->window, gc, 5, 19); */
/*      gdk_draw_point (drawingarea->window, gc, 5, 20); */
/*      gdk_draw_point (drawingarea->window, gc, 6, 19); */
/*      gdk_draw_point (drawingarea->window, gc, 6, 20); */

/*      return 0; */

/* } */

/* void  */
/* insert_barline_single(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      Score.barline_choosed = BARLINE_SINGLE; */

/* /\*      set_selection_barline(B_SINGLE); *\/ */

/* /\*      menu_selection.insert_barline = TRUE;  *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = FALSE; *\/ */
/*      draw_cursor(mouse_bits, mouse_mask_bits, 32, 32); */

/*      Selection.object = CURSOR; */

/* /\*      update_statusbar("barline", "Select a position where to put the single barline"); *\/ */
/* /\*      Score.context_statusbar = gtk_statusbar_get_context_id(GTK_STATUSBAR(Score.statusbar), "barline"); *\/ */
/* /\*      Score.message_id_statusbar = gtk_statusbar_push(GTK_STATUSBAR(Score.statusbar), Score.context_statusbar, *\/ */
/* /\*                                                      "Select a position where to put the single barline"); *\/ */
/* } */

/* void  */
/* insert_barline_double(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      Score.barline_choosed = BARLINE_DOUBLE; */

/* /\*      set_selection_barline(B_DOUBLE); *\/ */

/* /\*      menu_selection.insert_barline = TRUE;  *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = FALSE; *\/ */
/*      draw_cursor(mouse_bits, mouse_mask_bits, 32, 32); */
/*      Selection.object = CURSOR; */
/* /\*      Selection.note = CURSOR; *\/ */

/* /\*      update_statusbar("barline","Select a position where to put the double barline"); *\/ */
/* /\*      Score.context_statusbar = gtk_statusbar_get_context_id(Score.statusbar, "barline"); *\/ */
/* /\*      Score.message_id_statusbar = gtk_statusbar_push(Score.statusbar, Score.context_statusbar, *\/ */
/* /\*                                                      "Select a position where to put the double barline"); *\/ */
/* } */

/* void  */
/* insert_barline_openrepeat(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      Score.barline_choosed = BARLINE_OPENREPEAT; */

/* /\*      set_selection_barline(B_OPENREPEAT); *\/ */

/* /\*      menu_selection.insert_barline = TRUE;  *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = FALSE; *\/ */
/*      draw_cursor(mouse_bits, mouse_mask_bits, 32, 32); */
/*      Selection.object = CURSOR; */
/* /\*      Selection.note = CURSOR; *\/ */

/* /\*      update_statusbar("barline","Select a position where to put the open repeat barline"); *\/ */
/* } */

/* void  */
/* insert_barline_closerepeat(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      Score.barline_choosed = BARLINE_CLOSEREPEAT; */

/* /\*      set_selection_barline(B_CLOSEREPEAT); *\/ */

/* /\*      menu_selection.insert_barline = TRUE;  *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = FALSE; *\/ */
/*      draw_cursor(mouse_bits, mouse_mask_bits, 32, 32); */
/*      Selection.object = CURSOR; */

/* /\*      update_statusbar("barline","Select a position where to put the close repeat barline"); *\/ */
/* } */

/* void  */
/* insert_barline_opencloserepeat(GtkWidget *drawingarea, GdkEventExpose *ev) */
/* { */
/*      Score.barline_choosed = BARLINE_OPENCLOSEREPEAT; */

/* /\*      set_selection_barline(B_OPENCLOSEREPEAT); *\/ */

/* /\*      menu_selection.insert_barline = TRUE;  *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = FALSE; *\/ */
/*      draw_cursor(mouse_bits, mouse_mask_bits, 32, 32); */
/*      Selection.object = CURSOR; */

/* /\*      update_statusbar("barline","Select a position where to put the open and close repeat barline"); *\/ */
/* } */

/* void  */
/* insert_key(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */
  
/*   GtkWidget * radiobutton_1, * radiobutton_2, * radiobutton_3; */
/*   GtkWidget * button_ok, * button_cancel; */
/*   GtkWidget * treble_key; */
/*   GtkWidget * bass_key; */
/*   GtkWidget * alto_key; */
/*   GtkWidget * table; */

/*   GSList * group; */


/*   dialog = gtk_window_new(GTK_WINDOW_TOPLEVEL); */
/*   gtk_window_set_title(GTK_WINDOW(dialog), "Key"); */
/* /\*   gtk_widget_set_usize(GTK_WIDGET(dialog), 300, 280); *\/ */
/*   gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_CENTER); */
/* /\*   gtk_window_set_policy(GTK_WINDOW(dialog), FALSE, FALSE, TRUE); *\/ */

/*   /\* transparency for the keys *\/ */
/*   /\* */
/*   transparency_mask = gdk_pixmap_new(Score.area->window, */
/* 				     Score.area->allocation.width, */
/* 				     Score.area->allocation.height, */
/* 				     1); */
/*   gc = gdk_gc_new(transparency_mask); */
/*   gdk_gc_set_foreground(gc, &black); */
/*   *\/ */

  
/*   // table 7 lines, 4 columns  */

/*   table = gtk_table_new(6,5,TRUE); */
/*   gtk_container_add(GTK_CONTAINER(dialog), table); */

/*   radiobutton_1 = gtk_radio_button_new_with_label(NULL,"Treble key"); */
/*   gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(radiobutton_1), TRUE); */
/*   group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radiobutton_1)); */

/*   // table::radiobutton_1 */

/*   gtk_table_attach(GTK_TABLE(table), radiobutton_1, 0,2, 1,2, */
/* 		   GTK_FILL|GTK_EXPAND, GTK_FILL|GTK_EXPAND, 5, 5); */

/* /\*   pixmap = gdk_pixmap_create_from_xpm_d(dialog->window, &mask, &c, clesol_xpm); *\/ */
/*   treble_key = gtk_image_new_from_file("clesol.xpm"); */

/*   // table::treble_key   */
  
/*   gtk_table_attach(GTK_TABLE(table), treble_key, 2,3, 1,2,  */
/* 		   GTK_FILL|GTK_EXPAND,GTK_FILL|GTK_EXPAND, 0, 0); */

/*   radiobutton_2 = gtk_radio_button_new_with_label(group,"Bass key"); */
/*   group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radiobutton_2)); */

/*   // table::radiobutton_2  */

/*   gtk_table_attach(GTK_TABLE(table), radiobutton_2, 0,2, 2,3, */
/* 		   GTK_FILL|GTK_EXPAND, GTK_FILL|GTK_EXPAND, 5, 5); */

/* /\*   pixmap = gdk_pixmap_create_from_xpm_d(dialog->window, &mask, &c, clefa_xpm); *\/ */
/* /\*   bass_key = gtk_pixmap_new(pixmap, mask); *\/ */
/*   bass_key = gtk_image_new_from_file("clefa.xpm"); */

/*   // table::bass_key  */

/*   gtk_table_attach(GTK_TABLE(table), bass_key, 2,3, 2,3,  */
/* 		   GTK_FILL|GTK_EXPAND,GTK_FILL|GTK_EXPAND, 0, 0); */

/*   radiobutton_3 = gtk_radio_button_new_with_label(group,"Alto key"); */
/*   group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radiobutton_3)); */

/*   // table::radiobutton_3  */

/*   gtk_table_attach(GTK_TABLE(table), radiobutton_3, 0,2, 3,4, */
/* 		   GTK_FILL|GTK_EXPAND, GTK_FILL|GTK_EXPAND, 5, 5); */

/* /\*   pixmap = gdk_pixmap_create_from_xpm_d(dialog->window, &mask, &c, cleut_xpm); *\/ */
/* /\*   alto_key = gtk_pixmap_new(pixmap, mask); *\/ */
/*   alto_key = gtk_image_new_from_file("cleut.xpm"); */

/*   // table::alto_key   */

/*   gtk_table_attach(GTK_TABLE(table), alto_key, 2,3, 3,4,  */
/* 		   GTK_FILL|GTK_EXPAND,GTK_FILL|GTK_EXPAND, 0, 0); */

/*   button_ok = gtk_button_new_with_label("OK"); */

/*   // table::button_ok   */

/*   gtk_table_attach(GTK_TABLE(table), button_ok, 1,2, 5,6,  */
/* 		   GTK_FILL|GTK_EXPAND,GTK_FILL|GTK_EXPAND, 10, 10); */
/*   button_cancel = gtk_button_new_with_label(" Cancel "); */

/*   // table::button_cancel   */

/*   /\* */
/*   gtk_table_attach(GTK_TABLE(table), button_cancel, 3,4, 5,6,  */
/* 		   GTK_FILL|GTK_EXPAND,GTK_FILL|GTK_EXPAND, 10, 10); */


/*   gtk_signal_connect(GTK_OBJECT(button_ok),"clicked", */
/* 		     GTK_SIGNAL_FUNC(insert_key_callback), */
/* 		     NULL); */
/*   *\/ */

/*   g_signal_connect_swapped(GTK_OBJECT(button_ok),"clicked", */
/* 			    G_CALLBACK(gtk_widget_destroy), */
/* 			    GTK_OBJECT(dialog)); */
  
/*   g_signal_connect_swapped(GTK_OBJECT(button_cancel),"clicked", */
/* 			    G_CALLBACK(gtk_widget_destroy), */
/* 			    GTK_OBJECT(dialog)); */

/*   gtk_widget_show_all(dialog); */

/* } */

/* void  */
/* flow_direction(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */

/* } */

/* void  */
/* insert_tempo(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */
/* /\*      GtkWidget * window; *\/ */
/* /\*      GtkWidget * vbox; *\/ */
/* /\*      GtkWidget * hbox; *\/ */
/* /\*      GtkWidget * bbox; *\/ */
/* /\*      GtkWidget * button; *\/ */

/* /\*      GtkImage  * pixmap; *\/ */
/* /\*      GdkBitmap * mask; *\/ */
/* /\*      GdkColor    c;  *\/ */


/* /\*      window = gtk_window_new(GTK_WINDOW_TOPLEVEL); *\/ */
/* /\*      gtk_window_set_title(GTK_WINDOW(window), "Set the tempo"); *\/ */
/* /\*      gtk_window_set_position(GTK_WINDOW(window), GTK_WIN_POS_CENTER); *\/ */

/* /\*      vbox = gtk_vbox_new(FALSE, 10); *\/ */
/* /\*      gtk_container_add(GTK_CONTAINER(window), vbox); *\/ */

/* /\*      hbox = gtk_hbox_new(FALSE, 10); *\/ */
/* /\*      gtk_box_pack_start (GTK_BOX (vbox), hbox, TRUE, TRUE, 10); *\/ */

/* /\*      pixmap = gdk_pixmap_create_from_xpm_d(window->window, &mask,  *\/ */
/* /\*                                            &c, *\/ */
/* /\*                                            ic_noire_xpm); *\/ */
/* /\*      gtk_box_pack_start (GTK_BOX (hbox), pixmapwid, FALSE, FALSE, 5); *\/ */


/* /\*      bbox = gtk_hbutton_box_new(); *\/ */
/* /\*      gtk_box_pack_start (GTK_BOX (vbox), bbox, TRUE, TRUE, 10); *\/ */
     
/* /\*      gtk_container_set_border_width (GTK_CONTAINER (bbox), 5); *\/ */
/* /\*      gtk_container_add (GTK_CONTAINER (window), bbox); *\/ */

/* /\*      /\\* Set the appearance of the Button Box *\\/ *\/ */
/* /\*      gtk_button_box_set_layout (GTK_BUTTON_BOX (bbox), GTK_BUTTONBOX_EDGE); *\/ */
/* /\*      gtk_box_set_spacing (GTK_BOX (bbox), 10); *\/ */
     
/* /\*      button = gtk_button_new_from_stock (GTK_STOCK_HELP); *\/ */
/* /\*      gtk_container_add (GTK_CONTAINER (bbox), button); *\/ */

/* /\*      button = gtk_button_new_from_stock (GTK_STOCK_CANCEL); *\/ */
/* /\*      gtk_container_add (GTK_CONTAINER (bbox), button); *\/ */

/* /\*      button = gtk_button_new_from_stock (GTK_STOCK_OK); *\/ */
/* /\*      gtk_container_add (GTK_CONTAINER (bbox), button); *\/ */

/* /\*      gtk_widget_show_all(window); *\/ */


/*   /\* */
/*   GtkImage * pixmap; */
/*   GdkBitmap * mask; */
/*   GdkColormap * colormap; */
/*   gint width = 0; */
/*   gint height = 0; */

/*   GtkWidget * ok_button; */
/*   GtkWidget * cancel_button; */


/*   dialog = gtk_window_new(GTK_WINDOW_TOPLEVEL); */
/*   gtk_window_set_title(GTK_WINDOW(dialog), "Tempo"); */
/*   gtk_widget_set_usize(GTK_WIDGET(dialog), 300, 110); */
/*   gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_CENTER); */
/*   gtk_window_set_policy(GTK_WINDOW(dialog), FALSE, FALSE, TRUE); */


/*   ok_button = gtk_button_new_with_label("OK"); */
/*   cancel_button = gtk_button_new_with_label("OK"); */

/*   g_signal_connect_swapped(GTK_OBJECT(cancel_button),"clicked", */
/* 			   G_CALLBACK(gtk_widget_destroy),GTK_OBJECT(dialog)); */

/*   pixmap = gdk_pixmap_create_from_xpm_d(dialog->window, &mask, NULL, */
/* 					tempo_xp); */
/*   gdk_drawable_get_size(GTK_DRAWABLE(pixmap), &width, &height); */




/*   gtk_widget_show_all (dialog); */
/*   *\/ */
/* } */

/* void */
/* insert_text(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */
  

/* /\*   GtkWidget *label; *\/ */
/* /\*   GtkWidget *entry; *\/ */
/* /\*   GtkWidget *ok_button; *\/ */
/* /\*   GtkWidget *cancel_button; *\/ */

/* /\*   /\\* window *\\/ *\/ */
/* /\*   dialog = gtk_window_new(GTK_WINDOW_TOPLEVEL); *\/ */
/* /\*   gtk_window_set_title(GTK_WINDOW(dialog), "Inserer un texte"); *\/ */
/* /\*   gtk_window_set_position(GTK_WINDOW(dialog), GTK_WIN_POS_CENTER); *\/ */
/* /\*   gtk_widget_show (dialog); *\/ */

/* /\*   /\\* label *\\/ *\/ */
/* /\*   label = gtk_label_new("Tapez le texte a inserer :"); *\/ */
/* /\*   gtk_widget_show(label); *\/ */

/* /\*   /\\* entry *\\/ *\/ */
/* /\*   entry = gtk_entry_new(); *\/ */
/* /\*   gtk_widget_show(entry); *\/ */

/* /\*   /\\* buttons *\\/ *\/ */
/* /\*   ok_button     = gtk_button_new_with_label("OK"); *\/ */
/* /\*   cancel_button = gtk_button_new_with_label("Cancel"); *\/ */
/* /\*   g_signal_connect_swapped(GTK_OBJECT(ok_button),"clicked", *\/ */
/* /\* 			    G_CALLBACK(gtk_widget_destroy),GTK_OBJECT(dialog)); *\/ */
/* /\*   g_signal_connect_swapped(GTK_OBJECT(cancel_button),"clicked", *\/ */
/* /\* 			    G_CALLBACK(gtk_widget_destroy),GTK_OBJECT(dialog)); *\/ */
/* /\*   gtk_widget_show(ok_button); *\/ */
/* /\*   gtk_widget_show(cancel_button); *\/ */

/* /\*   /\\* table 3 lines, 4 columns *\\/ *\/ */
/* /\*   table = gtk_table_new(3,4,TRUE); *\/ */
/* /\*   gtk_container_add(GTK_CONTAINER(dialog), table); *\/ */
/* /\*   gtk_widget_show(table); *\/ */
/* /\*   /\\* table::label *\\/  *\/ */
/* /\*   gtk_table_attach(GTK_TABLE(table), label, 0,4, 0,1,  *\/ */
/* /\* 		   GTK_SHRINK,GTK_SHRINK, 0, 0); *\/ */
/* /\*   /\\* table::entry *\\/  *\/ */
/* /\*   gtk_table_attach(GTK_TABLE(table), entry, 0,4, 1,2,  *\/ */
/* /\* 		   GTK_SHRINK,GTK_SHRINK, 5, 0); *\/ */
/* /\*   /\\* table::ok_button *\\/  *\/ */
/* /\*   gtk_table_attach(GTK_TABLE(table), ok_button, 2,3, 2,3,  *\/ */
/* /\* 		   GTK_SHRINK,GTK_SHRINK, 4, 4); *\/ */
/* /\*   /\\* table::cancel_button *\\/  *\/ */
/* /\*   gtk_table_attach(GTK_TABLE(table), cancel_button, 1,2, 2,3,  *\/ */
/* /\* 		   GTK_SHRINK,GTK_SHRINK, 4, 4); *\/ */

/* /\*   //  g_free(KeySignature); *\/ */
/* /\*   //  gtk_widget_unref(KeySignature); *\/ */
/* /\*   g_object_unref(KeySignature); *\/ */
/* } */

/* void */
/* insert_dyn_mezzopiano(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */

/* /\*   draw_cursor(ic_mezzopiano_bits, ic_mezzopiano_mask_bits, 19, 10); *\/ */

/* } */

/* gint insert_dynamic_alt_cresc(gpointer callback_data, guint callback_action,  */
/*                                 GtkWidget *widget) */
/* { */
/* /\*      menu_selection.insert_barline = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = TRUE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = FALSE; *\/ */

/*      return 0; */
/* } */


/* gint insert_dynamic_alt_decresc(gpointer callback_data, guint callback_action,  */
/*                                 GtkWidget *widget) */
/* { */
/* /\*      menu_selection.insert_barline = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_cresc = FALSE; *\/ */
/* /\*      menu_selection.insert_dynamic_alt_decresc = TRUE; *\/ */

/*      return 0; */
/* } */
