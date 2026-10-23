/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * score.c
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
#include "display.h"
#include "draw.h"
#include "draw_barline.h"
#include "constants.h"
#include "score.h"
#include "staff.h"
#include "tab_transposition.h"
#include "common.h"
#include "macros.h"
#include "spacings.h"
#include "key_event.h"
#include "key_cursor.h"
#include "realize_area.h"
#include "mouse_event.h"

#define OK     "OK"
#define CANCEL "Cancel"


/* GType */
/* score_get_type(void) */
/* { */
/*         static GType object_type = 0; */

/*         if ( ! object_type ) { */
/*                 static const GTypeInfo object_info = */
/*                         { */
/*                                 sizeof(Score_t), */
/*                                 (GBaseInitFunc) NULL, */
/*                                 (GBaseFinalizeFunc) NULL, */
/*                                 (GClassInitFunc) NULL, */
/*                                 NULL,           /\* class_finalize *\/ */
/*                                 NULL,           /\* class_data *\/ */
/*                                 sizeof (Score_t), */
/*                                 0,              /\* n_preallocs *\/ */
/*                                 (GInstanceInitFunc) NULL /\* init *\/ */
/*                         }; */

/*                 object_type = g_type_register_static(G_TYPE_OBJECT,  */
/*                                                      "Score", */
/*                                                      &object_info, */
/*                                                      0); */
/*         } */

/*         return object_type; */
/* } */

/*
 * BEGIN: GTK WIDGETS
 * This are temporary GTK Widget so that I can make the release and strop strugling with
 * glade to get data from some random boxes
 * That shouldn't be there, it will be dropped latter.
 */
extern void 
set_header_widget(GtkButton *widget, gpointer user_data)
{
        GtkWidget *dialog;
        GtkWidget *vbox, *hbox;
        GtkWidget *label;
        GtkWidget *title_entry;
        GtkWidget *subtitle_entry;
        GtkWidget *composer_entry;

        gint response;

        Score_t *score;
	score = score_get_from_widget(GTK_WIDGET(widget));

        
        dialog = gtk_dialog_new_with_buttons("Set Header", NULL,
                                             GTK_DIALOG_MODAL, 
                                             GTK_STOCK_CANCEL,
                                             GTK_RESPONSE_REJECT,
                                             GTK_STOCK_OK,
                                             GTK_RESPONSE_ACCEPT,
                                             NULL);

        hbox = gtk_hbox_new(FALSE, 10);
        gtk_box_pack_start(GTK_BOX(GTK_DIALOG(dialog)->vbox), hbox,
                           TRUE, TRUE, 0);

        vbox = gtk_vbox_new(TRUE, 10);
        gtk_box_pack_start(GTK_BOX(hbox), vbox,
                           TRUE, TRUE, 0);

        label = gtk_label_new("Title: ");
        gtk_box_pack_start(GTK_BOX(vbox), label,
                           FALSE, FALSE, 5);

        label = gtk_label_new("Subtitle: ");
        gtk_box_pack_start(GTK_BOX(vbox), label,
                           FALSE, FALSE, 5);

        label = gtk_label_new("Composer: ");
        gtk_box_pack_start(GTK_BOX(vbox), label,
                           FALSE, FALSE, 5);

        vbox = gtk_vbox_new(TRUE, 10);
        gtk_box_pack_start(GTK_BOX(hbox), vbox,
                           TRUE, TRUE, 5);

        title_entry = gtk_entry_new();
        gtk_box_pack_start(GTK_BOX(vbox), title_entry,
                           FALSE, FALSE, 5);
	gtk_entry_set_text(GTK_ENTRY(title_entry), score->Identity->title->str);

        subtitle_entry = gtk_entry_new();
        gtk_box_pack_start(GTK_BOX(vbox), subtitle_entry,
                           FALSE, FALSE, 5);
	gtk_entry_set_text(GTK_ENTRY(subtitle_entry), score->Identity->subtitle->str);

        composer_entry = gtk_entry_new();
        gtk_box_pack_start(GTK_BOX(vbox), composer_entry,
                           FALSE, FALSE, 5);
	gtk_entry_set_text(GTK_ENTRY(composer_entry), score->Identity->composer->str);
    
        gtk_widget_show_all(dialog);

        response = gtk_dialog_run (GTK_DIALOG (dialog));

        if (response == GTK_RESPONSE_ACCEPT) {
                score->Identity->title = g_string_new(gtk_entry_get_text(GTK_ENTRY(title_entry)));
                score->Identity->subtitle = g_string_new(gtk_entry_get_text(GTK_ENTRY(subtitle_entry)));
                score->Identity->composer = g_string_new(gtk_entry_get_text(GTK_ENTRY(composer_entry)));
        }

        gtk_widget_destroy (dialog);
}
/* 
 * END: GTK WIDGETS
 */


/*
 * FIXME: Should be backported to libgscore
 * Creates a new score object
 * @Returns: the score object
 */ 
Score_t *gscore_score_new(void)
{

        Score_t *score;


        score = g_malloc(sizeof(Score_t));

        if ( ! score ) {
                printf("Memory exhausted!\n");
                return NULL;
        }

        score->Staff_list = NULL;   /* Staff list */
                
        score->object_id = 0;
        score->tempo = 60;
        score->tempo_text = g_string_new("");

        score->Identity = g_malloc(sizeof(Identity_t));
        
        if ( ! score->Identity ) {
                printf("Memory exhausted!\n");
                return NULL;
        }

        score->Identity->title = g_string_new(_("My Score"));
        score->Identity->subtitle = g_string_new(_("made with gscore"));
        score->Identity->composer = g_string_new(_("gscore's user"));

        score->staff_extremity_end_x = 100; /* Just a value to start, nothing really important */
        score->staff_startx = Spacings.Clefs.sb + STANDARD_KEY_SIZE + Spacings.Clefs.sa + 
                get_key_signature_spacing(KEY_SIGNATURE_TREBLE_EMPTY) + Spacings.KeySignatures.saks +
                STANDARD_TIME_SIGNATURE_SIZE + Spacings.TimeSignatures.sats;


        /* Create a Staff, to have something when we start the software */

        if( ! create_staff(score, 5, 8, Spacings.Measures.xpsfm, Spacings.Measures.ypsfm) )
                printf("ERROR CREATING STAFF");

        set_staff_selected(score, 0);

        staff_set_key(score, get_staff_selected(score), TREBLE_KEY);

        staff_set_key_signature(score, get_staff_selected(score), KEY_SIGNATURE_TREBLE_EMPTY);

        /* 	set_staff_unselect(get_staff_selected()); */
        /* 	set_staff_selected(1); */

        return score;

}

static GtkWidget *get_toplevel(GtkWidget *widget)
{
        GladeXML *xml = glade_get_widget_tree(widget);

        if(xml == gladexml) {
                return glade_xml_get_widget(xml, "main_window");
        } else {
                return  glade_xml_get_widget(xml, "score_window");
        }
}


Score_t* score_get_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (Score_t *) g_object_get_data(G_OBJECT(parent), "score");
}

extern gboolean 
score_set_to_widget(Score_t *score, GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if (parent == NULL) return FALSE;
        g_object_set_data(G_OBJECT(parent), "score", score);
        
        return TRUE;
        
}
  
GtkWidget *score_get_area_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "area");
}


Display_t *score_get_display_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (Display_t *) g_object_get_data(G_OBJECT(parent), "display");
}

Score_selection_t *score_get_selection_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (Score_selection_t *) g_object_get_data(G_OBJECT(parent), "selection");
}

KeyCursor_t *score_get_cursor_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (KeyCursor_t *) g_object_get_data(G_OBJECT(parent), "cursor");
}

GtkScrolledWindow *score_get_scrolled_window_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkScrolledWindow *) g_object_get_data(G_OBJECT(parent), "scrolledwindow");
}

GtkWidget *score_get_setkey_treble_rb_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "setkey_treble_rb");
}

GtkWidget *score_get_setkey_bass_rb_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "setkey_bass_rb");
}

GtkWidget *score_get_setkey_alto_rb_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "setkey_alto_rb");
}

GtkWidget *score_get_setkey_tenor_rb_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "setkey_tenor_rb");
}

GtkWidget *score_get_sks_clef_label_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "sks_clef_label");
}

GtkWidget *score_get_sks_image_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "sks_image");
}

GtkWidget *score_get_sks_hbox_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "sks_hbox");
}

GtkWidget *score_get_tempo_spinbutton_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "tempo_spinbutton");
}

GtkWidget *score_get_tempo_entry_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "tempo_entry");
}

GtkWidget *score_get_title_entry_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "title_entry");
}

GtkWidget *score_get_subtitle_entry_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "subtitle_entry");
}

GtkWidget *score_get_composer_entry_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "composer_entry");
}

GtkWidget *score_get_insert_text_entry_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "insert_text_entry");
}

GtkWidget *score_get_insert_text_font_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "insert_text_font");
}

GtkWidget *score_get_insert_text_color_from_widget(GtkWidget *widget)
{
        GtkWidget *parent;
        parent = get_toplevel(widget);
        if(parent == NULL) return NULL;
        return (GtkWidget *) g_object_get_data(G_OBJECT(parent), "insert_text_color");
}


void score_create_window(Score_t *score)
{
        Display_t *Display;
        Score_selection_t *selection;
        KeyCursor_t *KeyCursor;
        GladeXML *xml;
        GtkWidget *window;
        GtkWidget *area;

        /* Ugly, but needed for now :( */
        GtkWidget *setkey_treble_rb;
        GtkWidget *setkey_bass_rb;
        GtkWidget *setkey_alto_rb;
        GtkWidget *setkey_tenor_rb;
        GtkWidget *sks_clef_label;
        GtkWidget *sks_image;
        GtkWidget *sks_hbox;

        GtkWidget *tempo_spinbutton;
        GtkWidget *tempo_entry;

        GtkWidget *title_entry;
        GtkWidget *subtitle_entry;
        GtkWidget *composer_entry;

        GtkWidget *insert_text_entry;
        GtkWidget *insert_text_font;
        GtkWidget *insert_text_color;
        /* End of the Ugliest stuff I've ever done */

        GtkScrolledWindow *scrolled_window;
  
        selection = g_malloc(sizeof(Score_selection_t));
        selection->x_origin = 0;
        selection->y_origin = 0;
        selection->x = 0;
        selection->y = 0;
  
        if(score == NULL) {
                score = gscore_score_new();
        } 

        KeyCursor = g_malloc(sizeof(KeyCursor_t));
  
        KeyCursor->position = 0;



        Display = gscore_display_new();

        /*   xml = glade_xml_new(get_file_from_data_dir("glade/gscore.glade"), */
        /* 		      "score_window", NULL); */
        /* BEGIN Dirty hack */
        xml = glade_xml_new(get_file_from_data_dir("glade/gscore.glade"),
                            NULL, NULL);
        window = glade_xml_get_widget(xml, "main_window");
        gtk_widget_hide(window);
        /* END Dirty hack */
        window = glade_xml_get_widget(xml, "score_window");
        glade_xml_signal_autoconnect(xml) ;
  
        g_signal_connect(GTK_OBJECT(window), "key_press_event",
                         G_CALLBACK(score_key_press_event), NULL);

        area = glade_xml_get_widget(xml, "sw_score_da");

        scrolled_window = glade_xml_get_widget(xml, "sw_score_sw");

        setkey_treble_rb = glade_xml_get_widget(xml, "setkey_treble_rb");
        setkey_bass_rb = glade_xml_get_widget(xml, "setkey_bass_rb");
        setkey_alto_rb = glade_xml_get_widget(xml, "setkey_alto_rb");
        setkey_tenor_rb = glade_xml_get_widget(xml, "setkey_tenor_rb");

        sks_clef_label = glade_xml_get_widget(xml, "sks_clef_label");
        sks_image      = glade_xml_get_widget(xml, "sks_image"); 
        sks_hbox       = glade_xml_get_widget(xml, "sks_hbox");

        tempo_spinbutton = glade_xml_get_widget(xml, "tempo_spinbutton");
        tempo_entry = glade_xml_get_widget(xml, "tempo_entry");
        
	title_entry = glade_xml_get_widget(gladexml, "title_entry");
	subtitle_entry = glade_xml_get_widget(gladexml, "subtitle_entry");
	composer_entry = glade_xml_get_widget(gladexml, "composer_entry");

        insert_text_entry = glade_xml_get_widget(gladexml, "insert_text_entry");
        insert_text_font = glade_xml_get_widget(gladexml, "insert_text_font");
        insert_text_color = glade_xml_get_widget(gladexml, "insert_text_color");


        g_object_set_data(G_OBJECT(window), "score", score);
        g_object_set_data(G_OBJECT(window), "area", area);
        g_object_set_data(G_OBJECT(window), "selection", selection);
        g_object_set_data(G_OBJECT(window), "cursor", KeyCursor);
        g_object_set_data(G_OBJECT(window), "display", Display);
        g_object_set_data(G_OBJECT(window), "scrolledwindow", scrolled_window);

        g_object_set_data(G_OBJECT(window), "setkey_treble_rb", setkey_treble_rb);
        g_object_set_data(G_OBJECT(window), "setkey_bass_rb", setkey_bass_rb);
        g_object_set_data(G_OBJECT(window), "setkey_alto_rb", setkey_alto_rb);
        g_object_set_data(G_OBJECT(window), "setkey_tenor_rb", setkey_tenor_rb);

        g_object_set_data(G_OBJECT(window), "sks_clef_label", sks_clef_label);
        g_object_set_data(G_OBJECT(window), "sks_image", sks_image);
        g_object_set_data(G_OBJECT(window), "sks_hbox", sks_hbox);

        g_object_set_data(G_OBJECT(window), "tempo_spinbutton", tempo_spinbutton);
        g_object_set_data(G_OBJECT(window), "tempo_entry", tempo_entry);

        g_object_set_data(G_OBJECT(window), "title_entry", title_entry);
        g_object_set_data(G_OBJECT(window), "subtitle_entry", subtitle_entry);
        g_object_set_data(G_OBJECT(window), "composer_entry", composer_entry);

        g_object_set_data(G_OBJECT(window), "insert_text_entry", insert_text_entry);
        g_object_set_data(G_OBJECT(window), "insert_text_font", insert_text_font);
        g_object_set_data(G_OBJECT(window), "insert_text_color", insert_text_color);

        /*   glade_set_widgets(xml); */
        score->height = 500;
        score->width = 500;

        g_signal_connect(GTK_OBJECT(area), "expose_event",
                         G_CALLBACK(score_area_callback), NULL);
        g_signal_connect(GTK_OBJECT(area), "button_press_event",
                         G_CALLBACK(mouse_button_press_event), NULL);
        g_signal_connect(GTK_OBJECT(area), "button_release_event",
                         G_CALLBACK(mouse_button_release_event), NULL);
        /*   g_signal_connect(GTK_OBJECT(area), "motion_notify_event", */
        /* 		   G_CALLBACK(mouse_motion_event), NULL); */
	
        staff_update_statusbar(xml);

        gtk_widget_show(window);
  
        /* Set the white color to the drawing area */
        /* We want the score to be white */
        if ( ! area ) {
                printf(_("Error: The score layout cannot be drawn\n"));
                /*     return -1; */
        } else {
                colorize_drawingarea(area, 65535, 65535, 65535);
        }
}

gboolean score_set_staff_extremity_end_x(Score_t *score, gdouble extremity_end_x)
{
        score->staff_extremity_end_x = extremity_end_x;

	return TRUE;
}

