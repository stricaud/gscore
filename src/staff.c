/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * staff.c
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

/** 
 * @file
 * 
 * Functions to deal with staves
 */

#include <gtk/gtk.h>

#include "gscore.h"
#include "common.h"
#include "constants.h"
#include "draw.h"
#include "debug.h"
#include "macros.h"
#include "spacings.h"
#include "objects.h"
#include "staff.h"
#include "score.h"

/* GtkWidget * vbox; */
/* GtkWidget * key_signature_pixmap; */
/* GtkWidget * label; */
/* GtkWidget * pixmap_wid; */

/* GtkImage  * pixmap; */
/* GdkBitmap * mask; */
/* GdkColor    c; */


/* gint old_key_signature = 0; */
gboolean already_packed = FALSE;
static gint key_signature;


/* gboolean staff_set_time_signature(gint staff, gint signature_type, gint number_of_beats, gint beat_duration); */


/**
 * Create a staff
 *
 * @param nb_lines is the number of lines
 * @param space_btw_lines is the space (in pixels) left between the lines
 * @param extremity_begin_x is the x position of the staff
 * @param extremity_begin_y is the y position of the staff
 *
 * @return TRUE if the staff is created, FALSE if not
 */
gboolean create_staff(Score_t *score, guint8 nb_lines, guint8 space_btw_lines,
                      gdouble extremity_begin_x, gdouble extremity_begin_y)
{

/*         g_print("*** Staff Creation:\n\t%d lines\n\t%dpixels between lines\n\t%f.0 extremity_begin_x\n\t%f.0 extremity_begin_y\n***", */
/*                 nb_lines, space_btw_lines, extremity_begin_x, extremity_begin_y); */

        score->Staff = g_malloc(sizeof(Staff_t));
 
        score->Staff->nb_lines = nb_lines;
        score->Staff->space_btw_lines = space_btw_lines;
        score->Staff->is_selected = FALSE;
        score->Staff->key = NO_KEY;
        score->Staff->key_signature = KEY_SIGNATURE_EMPTY;
        score->Staff->time_signature[0] = TIME_SIGNATURE_NORMAL;
        score->Staff->time_signature[1] = 4;
        score->Staff->time_signature[2] = 4;
        score->Staff->measure_number = 1;
        score->Staff->total_measures = score->Staff->measure_number;
        score->Staff->extremity_begin_x = extremity_begin_x;
        score->Staff->extremity_begin_y = extremity_begin_y;

        score->Staff->current_x = extremity_begin_x + Spacings.Clefs.sb + STANDARD_KEY_SIZE + Spacings.Clefs.sa;
	score->Staff->start_x = extremity_begin_x + Spacings.Clefs.sb + STANDARD_KEY_SIZE + Spacings.Clefs.sa;

        score->Staff->midi_instrument = MIDI_GRAND_PIANO;
        score->Staff->Object_list = NULL;

        score->Staff_list =
                g_list_append(score->Staff_list, score->Staff);

        if (!score->Staff_list)
                return FALSE;

        score->nb_staves++;

        return TRUE;
}

extern void 
set_staff_selected(Score_t *score, gint staff)
{

        Staff_t *staff_data;
	staff_data = g_list_nth_data(score->Staff_list, staff);

        if (staff_data)
                staff_data->is_selected = TRUE;

}

extern void 
set_staff_unselect(Score_t *score, gint staff)
{

        Staff_t *staff_data;
	staff_data = g_list_nth_data(score->Staff_list, staff);

        if (staff_data)
                staff_data->is_selected = FALSE;

}

gboolean delete_staff(Score_t *score, gint staff)
{
        GList * listrunner;
        gboolean retval;

        listrunner = g_list_first(score->Staff_list);
        if (!listrunner) 
                retval = FALSE;
        else
                retval = TRUE;

        while (listrunner)
        {
                Staff_t *staff_data;

                staff_data = g_list_nth_data(score->Staff_list, staff);

                score->Staff_list = g_list_remove(score->Staff_list, staff_data);
        }

	return TRUE;
}

gint get_staff_selected(const Score_t *score)
{
        GList * listrunner;
        gint    counter;

        counter = 0;

        listrunner = g_list_first(score->Staff_list);

        while ( listrunner ) {
                        Staff_t *staff;
                        
                        staff = (Staff_t *)listrunner->data;

                        if ( staff->is_selected )
                                        return counter;
                               
                        counter++;
                        listrunner = g_list_next(listrunner);
                }


        return -1;
}

gint staff_get_key_signature(const Score_t *score, guint staff)
{
	Staff_t *staff_data;

/* 	gint retval = 0; */

	staff_data = g_list_nth_data(score->Staff_list, staff);

        if (staff_data)
                return staff_data->key_signature;

	return 0;
}

/* Returns the y position where the next staff should be */
gint staff_get_y_for_next(const Score_t *score)
{
        Staff_t *staff_data;
        gint retval = 0;
	gint selected;

	selected = get_staff_selected(score);
	if(selected == -1) {
	  GList *list = g_list_last(score->Staff_list);
	  if(list ) {
	    staff_data = list->data;
	  } else {
	    staff_data = NULL;
	  }
	} else {
	  staff_data = g_list_nth_data(score->Staff_list, selected);
	}

	if(staff_data) {
	  retval += staff_data->extremity_begin_y;
	  retval += (staff_data->nb_lines - 1) * staff_data->space_btw_lines + staff_data->nb_lines;
	}
	
        retval += Spacings.Measures.slbm;

        return retval;
}

/* Function called by the ADD/REMOVE Staff dialog box to update spinbuttons */
void staff_add_remove_options(void)
{

        GtkWidget *widget;
	Score_t *score;

        widget = glade_xml_get_widget(gladexml, "sb_extremityxbegin");
        gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Measures.xpsfm);
        widget = glade_xml_get_widget(gladexml, "sb_extremityybegin");
	score = score_get_from_widget(widget);
	
        gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), staff_get_y_for_next(score));

}

void staff_add_remove_ok(void)
{

        GtkWidget *widget;
	Score_t *score;
	GtkWidget *area;
	

        guint numberoflines;
        guint spacebetweenlines;
        guint extremityxbegin;
        guint extremityybegin;


        widget = glade_xml_get_widget(gladexml, "sb_numberoflines");
        numberoflines = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(widget));
        widget = glade_xml_get_widget(gladexml, "sb_spacebetweenlines");
        spacebetweenlines = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(widget));
        widget = glade_xml_get_widget(gladexml, "sb_extremityxbegin");
        extremityxbegin = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(widget));
        widget = glade_xml_get_widget(gladexml, "sb_extremityybegin");
        extremityybegin = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(widget));

        widget = glade_xml_get_widget(gladexml, "staff_insert_below");
	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

        if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget))) {
                        /* Add a staff below ;) */
                        create_staff(score, numberoflines, spacebetweenlines,
                                     extremityxbegin, extremityybegin);
                }

        refresh(area);

}

void on_settime_ok_clicked(void)
{
        GtkWidget *widget;
	Score_t *score;
	GtkWidget *area;
	
        widget = glade_xml_get_widget(gladexml, "stime_radio_common");
	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);
	
	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)) ) {
		staff_set_time_signature(score, get_staff_selected(score), TIME_SIGNATURE_COMMON_TIME, 4, 4);

		score->staff_extremity_end_x += STANDARD_TIME_SIGNATURE_SIZE;
/* 		staff_set_current_x(get_staff_selected(),  */
/* 				    staff_get_current_x(get_staff_selected()) + STANDARD_TIME_SIGNATURE_SIZE + Spacings.TimeSignatures.sats); */
/* 		staff_set_start_x(get_staff_selected(), offset); */
	}

        widget = glade_xml_get_widget(gladexml, "stime_radio_cut");
	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)) ) {
		staff_set_time_signature(score, get_staff_selected(score), TIME_SIGNATURE_ALLA_BREVE, 2, 2);
	}

	refresh(area);

}

gboolean staff_update_statusbar(GladeXML *xml)
{
        GtkWidget *widget;
	Score_t *score;

        guint cid;              /* Context ID */

        widget = glade_xml_get_widget(xml, "statusbar");
	score = score_get_from_widget(widget);

        cid = gtk_statusbar_get_context_id(GTK_STATUSBAR(widget), "staff");

/*         printf("staff_selected= %d\n", get_staff_selected()); */

        cid = gtk_statusbar_push(GTK_STATUSBAR(widget), cid, g_strdup_printf("Staff: %d", get_staff_selected(score)));


	return TRUE;
}

gboolean staff_set_start_x(Score_t *score, gint staff, gint start_x)
{
        Staff_t *staff_data;

        staff_data = g_list_nth_data(score->Staff_list, staff);
        
        if ( staff_data ) {
                staff_data->start_x = start_x;
		return TRUE;
	}

	return FALSE;
}

/* gint staff_get_start_x(Staff_t *staff) */
/* { */
/*         Staff_t *staff_data; */


/*         staff_data = g_list_nth_data(score->Staff_list, staff); */

/* 	if ( staff_data ) */
/* 		return staff_data->start_x; */

/* 	return -1; */
/* } */

gboolean staff_set_key(Score_t *score, gint staff, gint key)
{
        Staff_t *staff_data;

        staff_data = (Staff_t *)g_list_nth_data(score->Staff_list, staff);
        
        if ( staff_data ) {
                staff_data->key = key;
		return TRUE;
	}

	return FALSE;
}

extern void 
staff_set_key_callback(GtkButton *button, gpointer user_data)
{
/*         GtkWidget *widget; */
	Score_t *score;
	GtkWidget *area;
        GtkWidget *treble_w;
        GtkWidget *bass_w;
        GtkWidget *alto_w;
        GtkWidget *tenor_w;

/*         widget = glade_xml_get_widget(gladexml, "setkey_treble_rb"); */
        
/* 	score = score_get_from_widget(widget); */
/* 	area = score_get_area_from_widget(widget); */
	score = score_get_from_widget(GTK_WIDGET(button));
	area = score_get_area_from_widget(GTK_WIDGET(button));

        treble_w = score_get_setkey_treble_rb_from_widget(GTK_WIDGET(button));
        bass_w = score_get_setkey_bass_rb_from_widget(GTK_WIDGET(button));
        alto_w = score_get_setkey_alto_rb_from_widget(GTK_WIDGET(button));
        tenor_w = score_get_setkey_tenor_rb_from_widget(GTK_WIDGET(button));
	
	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(treble_w)) ) {
		staff_set_key(score, get_staff_selected(score), TREBLE_KEY);
        }

/*         widget = glade_xml_get_widget(gladexml, "setkey_bass_rb"); */
	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(bass_w)) ) {
		staff_set_key(score, get_staff_selected(score), BASS_KEY);
        }
/*         widget = glade_xml_get_widget(gladexml, "setkey_alto_rb"); */
	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(alto_w)) )
		staff_set_key(score, get_staff_selected(score), ALTO_KEY);

/*         widget = glade_xml_get_widget(gladexml, "setkey_tenor_rb"); */
	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(tenor_w)) )
		staff_set_key(score, get_staff_selected(score), TENOR_KEY);

	refresh(area);

}


gint staff_get_key(const Score_t *score, gint staff)
{
        Staff_t *staff_data;

        staff_data = g_list_nth_data(score->Staff_list, staff);
        
        if (staff_data) {
                return staff_data->key;
        }

        return -1;
}

gboolean staff_set_current_x(Score_t *score, gint staff, gint current_x)
{
        Staff_t *staff_data;

        staff_data = g_list_nth_data(score->Staff_list, staff);
        
        if (staff_data) {
                staff_data->current_x = current_x;
		return TRUE;
	}
	
	return FALSE;
}

gint staff_get_current_x(const Score_t *score, gint staff)
{
        Staff_t *staff_data;

        staff_data = g_list_nth_data(score->Staff_list, staff);
        
        if (staff_data)
                return staff_data->current_x;

        return -1;
}

gdouble get_staff_extremity_begin_x(const Score_t *score, gint staff)
{
        Staff_t *staff_data;

        staff_data = g_list_nth_data(score->Staff_list, staff);

	if(staff_data)
                return staff_data->extremity_begin_x;

        
        return 0;

}

gdouble get_staff_extremity_begin_y(const Score_t *score, gint staff)
{
        Staff_t *staff_data;

        staff_data = g_list_nth_data(score->Staff_list, staff);

	if(staff_data)
	  return staff_data->extremity_begin_y;

        
        return 0;

}


gboolean staff_set_key_signature(Score_t *score, gint staff, gint key_signature)
{

        Staff_t *staff_data;
        
        staff_data = g_list_nth_data(score->Staff_list, staff);
        
        if (staff_data) {
                staff_data->key_signature = key_signature;
                return TRUE;
        }

	return FALSE;

}

gboolean staff_set_time_signature(Score_t *score, gint staff,
				  gint signature_type, gint number_of_beats,
				  gint beat_duration)
{
        Staff_t *staff_data;
        
        staff_data = g_list_nth_data(score->Staff_list, staff);

        if (staff_data) {
                staff_data->time_signature[0] = signature_type;
                staff_data->time_signature[1] = number_of_beats;
                staff_data->time_signature[2] = beat_duration;
                return TRUE;
        }

        return FALSE;
}

gboolean staff_set_midi_instrument(Score_t *score, gint staff, gint midi_instrument)
{
	Staff_t *staff_data;
	
	staff_data = g_list_nth_data(score->Staff_list, staff);

	if (staff_data) {
                staff_data->midi_instrument = midi_instrument;
                return TRUE;
        }

        return FALSE;
}


/* /\* BEGIN: glade stuff *\/ */
/* void on_add_remove_staff_activate(GtkWidget *widget) */
/* { */
/* 	widget = glade_xml_get_widget (gladexml, "add_remove_staff"); */
/* 	gtk_widget_show(widget); */
/* } */

void ValueOfRange (GtkAdjustment *adj, gpointer user_data);

static GtkWidget *get_toplevel(GtkWidget *widget)
{
  
        GladeXML *xml = glade_get_widget_tree(widget);

        if(xml == gladexml) {
                return glade_xml_get_widget(xml, "main_window");
        } else {
                return  glade_xml_get_widget(xml, "score_window");
        }
}

void on_key_signature_activate(GtkWidget *widget)
{
	GtkWidget *hscale;
	GtkAdjustment *adj;
	Score_t *score = score_get_from_widget(widget);

        GtkWidget *sks_clef_label = score_get_sks_clef_label_from_widget(widget);
        GtkWidget *sks_image = score_get_sks_image_from_widget(widget);
        GtkWidget *sks_hbox = score_get_sks_hbox_from_widget(widget);

	gint value_adj = 0;

        /* The adjustment */

	switch(staff_get_key_signature(score, get_staff_selected(score))) {
	case KEY_SIGNATURE_TREBLE_EMPTY:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "C Major or A minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_empty.xpm"));

		value_adj = 0;
		break;
	case KEY_SIGNATURE_TREBLE_A_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "B Major or G sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_A_sharp.xpm"));

		value_adj = 5;
		break;
	case KEY_SIGNATURE_TREBLE_B_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "C sharp Major or A sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_B_sharp.xpm"));

		value_adj = 7;
		break;
	case KEY_SIGNATURE_TREBLE_C_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "D Major or B minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_C_sharp.xpm"));

		value_adj = 2;
		break;
	case KEY_SIGNATURE_TREBLE_D_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "E Major or C sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_D_sharp.xpm"));

		value_adj = 4;
		break;
	case KEY_SIGNATURE_TREBLE_E_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "F sharp Major or  D sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_E_sharp.xpm"));

		value_adj = 6;
		break;
	case KEY_SIGNATURE_TREBLE_F_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "G Major or E minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_F_sharp.xpm"));

		value_adj = 1;
		break;
	case KEY_SIGNATURE_TREBLE_G_SHARP:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "A Major or F sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_G_sharp.xpm"));

		value_adj = 3;
		break;
	case KEY_SIGNATURE_TREBLE_A_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "E flat Major or C minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_A_flat.xpm"));

		value_adj = -3;
		break;
	case KEY_SIGNATURE_TREBLE_B_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "F Major or D minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_B_flat.xpm"));

		value_adj = -1;
		break;
	case KEY_SIGNATURE_TREBLE_C_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "G flat Major or E flat minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_C_flat.xpm"));

		value_adj = -6;
		break;
	case KEY_SIGNATURE_TREBLE_D_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "A flat Major or F minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_D_flat.xpm"));

		value_adj = -4;
		break;
	case KEY_SIGNATURE_TREBLE_E_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "B flat Major or G minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_E_flat.xpm"));

		value_adj = -2;
		break;
	case KEY_SIGNATURE_TREBLE_F_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "D flat Major");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_F_flat.xpm"));

		value_adj = -7;
		break;
	case KEY_SIGNATURE_TREBLE_G_FLAT:
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "D flat Major or B minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_G_flat.xpm"));

		value_adj = -5;
		break;
	}



	adj = GTK_ADJUSTMENT(gtk_adjustment_new(value_adj, -7, 7, 1, 1, 0));
   
	if ( ! already_packed)
	{
		hscale = gtk_hscale_new (adj);
   
		gtk_scale_set_digits(GTK_SCALE(hscale), 0);
		gtk_scale_set_draw_value(GTK_SCALE(hscale), FALSE);
		GTK_RANGE(hscale)->round_digits = 0;


/* 		widget = glade_xml_get_widget(gladexml, "sks_hbox"); */
		gtk_box_pack_start_defaults(GTK_BOX(sks_hbox), hscale);

		g_signal_connect(GTK_OBJECT(adj),"value_changed",
				 G_CALLBACK(ValueOfRange), widget);

		gtk_widget_show(hscale);

		already_packed = TRUE;

	}
}

void on_key_signature_value_changed(void)
{
/* 	GtkWidget *widget; */

/* 	gint i = -1; */

/* 	switch (i) */
/* 	{ */
/* 	case -1: */
/* 		score->Staff[get_staff_selected()].key_signature = */
/* 			KEY_SIGNATURE_TREBLE_B_FLAT; */
		
/* 		widget = glade_xml_get_widget (gladexml, "sks_clef_label"); */
/* 		gtk_label_set_text(GTK_LABEL(widget), "F Major or D minor"); */

/* 		widget = glade_xml_get_widget (gladexml, "sks_image"); */
/* 		gtk_image_set_from_file(GTK_IMAGE(widget), get_file_from_data_dir("pixmaps/key_signature/treble_B_flat.xpm")); */
/* 		break; */
/* 	} */
}

/* END: glade stuff */

/* gint get_staff_selected(void); */

void ValueOfRange (GtkAdjustment *adj, gpointer user_data)
{

        GtkWidget *widget = (GtkWidget *) user_data;

        GtkWidget *sks_clef_label = score_get_sks_clef_label_from_widget(widget);
        GtkWidget *sks_image = score_get_sks_image_from_widget(widget);

	gint i;

	i = adj->value;
	
	switch (i) {
		
	case -1:
		/* 		score->Staff[get_staff_selected()].key_signature = */
		key_signature = KEY_SIGNATURE_TREBLE_B_FLAT;
		
		gtk_label_set_text(GTK_LABEL(sks_clef_label), "F Major or D minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_B_flat.xpm"));

		break;
	case -2:
		key_signature = KEY_SIGNATURE_TREBLE_E_FLAT;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "B flat Major or G minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_E_flat.xpm"));

		break;
	case -3:
		key_signature = KEY_SIGNATURE_TREBLE_A_FLAT;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "E flat Major or C minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_A_flat.xpm"));

		break;
	case -4:
		key_signature = KEY_SIGNATURE_TREBLE_D_FLAT;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "A flat Major or F minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_D_flat.xpm"));

		break;
	case -5:
		key_signature = KEY_SIGNATURE_TREBLE_G_FLAT;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "D flat Major or B minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_G_flat.xpm"));

		break;
	case -6:
		key_signature = KEY_SIGNATURE_TREBLE_C_FLAT;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "G flat Major or E flat minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_C_flat.xpm"));

		break;
	case -7:
		key_signature = KEY_SIGNATURE_TREBLE_F_FLAT;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "C flat Major or A flat minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_F_flat.xpm"));

		break;
	case 0:
		key_signature = KEY_SIGNATURE_TREBLE_EMPTY;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "C Major or A minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_empty.xpm"));

		break;
	case 1:
		key_signature = KEY_SIGNATURE_TREBLE_F_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "G Major or E minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_F_sharp.xpm"));

		break;
	case 2:
		key_signature = KEY_SIGNATURE_TREBLE_C_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "D Major or B minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_C_sharp.xpm"));

		break;
	case 3:
		key_signature = KEY_SIGNATURE_TREBLE_G_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "A Major or F sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_G_sharp.xpm"));

		break;
	case 4:
		key_signature = KEY_SIGNATURE_TREBLE_D_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "E Major or C sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_D_sharp.xpm"));

		break;
	case 5:
		key_signature = KEY_SIGNATURE_TREBLE_A_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "B Major or G sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_A_sharp.xpm"));

		break;
	case 6:
		key_signature = KEY_SIGNATURE_TREBLE_E_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "F sharp Major or  D sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_E_sharp.xpm"));

		break;
	case 7:
		key_signature = KEY_SIGNATURE_TREBLE_B_SHARP;

		gtk_label_set_text(GTK_LABEL(sks_clef_label), "C sharp Major or A sharp minor");
		gtk_image_set_from_file(GTK_IMAGE(sks_image), get_file_from_data_dir("pixmaps/treble_B_sharp.xpm"));

		break;
	}
}


void staff_key_signature(gpointer callback_data, guint callback_action, GtkWidget *widget)
{

/*      static GtkWidget * key_signature_dialog = NULL; */

/*      if (key_signature_dialog) */
/*      { */
/*           if (!GTK_WIDGET_VISIBLE(key_signature_dialog)) */
/*                gtk_widget_show(key_signature_dialog); */
/*           else */
/*                gdk_window_raise(key_signature_dialog->window); */
/*      } else */
/*      { */
/*           gint        value_adj = 0; */
/*           GtkWidget * hscale; */

/*           GtkAdjustment * adj; */

          
/*           key_signature_dialog = gtk_dialog_new_with_buttons("Set key signature", NULL, */
/*                                                         GTK_DIALOG_MODAL, */
/*                                                         GTK_STOCK_CANCEL, GTK_RESPONSE_REJECT, */
/*                                                         GTK_STOCK_OK, GTK_RESPONSE_ACCEPT, */
/*                                                         NULL); */
/*           g_signal_connect(G_CALLBACK(key_signature_dialog), "destroy", */
/*                            G_CALLBACK(gtk_widget_destroyed), */
/*                            &key_signature_dialog); */
          
/*           /\* The main part *\/ */
/*           vbox = gtk_vbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(GTK_DIALOG(key_signature_dialog)->vbox), vbox); */
/*           gtk_widget_show(vbox); */

/*           /\* The adjustment *\/ */
/*           switch (score->Staff[get_staff_selected()].key_signature) */
/*           { */
/*           case KEY_SIGNATURE_TREBLE_EMPTY: */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_empty.xpm")); */
/*                label = gtk_label_new("C Major or A minor"); */
/*                value_adj = 0; */
/*                break;  */
/*           case KEY_SIGNATURE_TREBLE_A_SHARP: */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_A_sharp.xpm")); */
/*                label = gtk_label_new("B Major or G sharp minor"); */
/*                value_adj = 5; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_B_SHARP:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_B_sharp.xpm")); */
/*                label = gtk_label_new("C sharp Major or A sharp minor"); */
/*                value_adj = 7; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_C_SHARP:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_C_sharp.xpm")); */
/*                label = gtk_label_new("D Major or B minor"); */
/*                value_adj = 2; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_D_SHARP:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_D_sharp.xpm")); */
/*                label = gtk_label_new("E MAjor or C sharp minor"); */
/*                value_adj = 4; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_E_SHARP:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_E_sharp.xpm")); */
/*                label = gtk_label_new("F sharp Major or  D sharp minor"); */
/*                value_adj = 6; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_F_SHARP:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_empty.xpm")); */
/*                label = gtk_label_new("G Major or E minor"); */
/*                value_adj = 1; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_G_SHARP:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_G_sharp.xpm")); */
/*                label = gtk_label_new("A Major or F sharp minor"); */
/*                value_adj = 3; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_A_FLAT:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_A_flat.xpm")); */
/*                label = gtk_label_new("E flat Major or C minor"); */
/*                value_adj = -3; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_B_FLAT:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_B_flat.xpm")); */
/*                label = gtk_label_new("F Major or D minor"); */
/*                value_adj = -1; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_C_FLAT:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_C_flat.xpm")); */
/*                label = gtk_label_new("G flat Major or E flat minor"); */
/*                value_adj = -6; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_D_FLAT:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_D_flat.xpm")); */
/*                label = gtk_label_new("A flat Major or F minor"); */
/*                value_adj = -4; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_E_FLAT:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_E_flat.xpm")); */
/*                label = gtk_label_new("B flat Major or G minor"); */
/*                value_adj = -2; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_F_FLAT:  */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_F_flat.xpm")); */
/*                label = gtk_label_new("D flat Major"); */
/*                value_adj = -7; */
/*                break; */
/*           case KEY_SIGNATURE_TREBLE_G_FLAT: */
/*                pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/treble_G_flat.xpm")); */
/*                label = gtk_label_new("D flat Major or B minor"); */
/*                value_adj = -5; */
/*                break; */
/*           } */


         
/*           adj = GTK_ADJUSTMENT(gtk_adjustment_new(value_adj, -7, 7, 1, 1, 0)); */

/*           hscale = gtk_hscale_new (adj); */
/*           gtk_scale_set_digits(GTK_SCALE(hscale), 0); */
/*           gtk_scale_set_draw_value(GTK_SCALE(hscale), FALSE); */
/*           GTK_RANGE(hscale)->round_digits = 0; */
/*           gtk_box_pack_end_defaults(GTK_BOX(vbox), hscale); */
/*           gtk_widget_show(hscale); */

/*           g_signal_connect(GTK_OBJECT(adj),"value_changed", */
/*                            G_CALLBACK(ValueOfRange), NULL); */

/*           /\* the label *\/ */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), label); */
/*           gtk_widget_show(label); */

/*           /\* The pixmap widget *\/ */

/*           gtk_widget_show(pixmap_wid); */
/*           gtk_box_pack_start(GTK_BOX(vbox), GTK_WIDGET(pixmap_wid), */
/*                              TRUE, FALSE, 0); */

/*           /\* Execute the OK button action *\/ */
/*           if (gtk_dialog_run(GTK_DIALOG(key_signature_dialog))==GTK_RESPONSE_ACCEPT) */
/*           { */
/*                g_message("Accept!"); */
/*           } */
/*           gtk_widget_destroy(key_signature_dialog); */
/*      } */

}

void staff_time_signature(gpointer callback_data, guint callback_action, GtkWidget *widget)
{

/*      static GtkWidget * time_signature_dialog = NULL; */

     
/*      old_key_signature = score->Staff[get_staff_selected()].time_signature; */

/*      if (time_signature_dialog) */
/*      { */
/*           if (!GTK_WIDGET_VISIBLE(time_signature_dialog)) */
/*                gtk_widget_show(time_signature_dialog); */
/*           else */
/*                gdk_window_raise(time_signature_dialog->window); */
/*      } else */
/*      { */
/*           GtkWidget * vbox, * hbox; */
/*           GtkWidget * drawingarea; */
/*           GtkWidget * NumeratorEntry, * DenominatorEntry; */
/*           GtkWidget * radio_24, * radio_68, * radio_common_time, * radio_alla_breve, * radio_custom; */
/*           GtkWidget * pixmap_wid; */
/*           GtkWidget * hseparator; */

/*           GSList    * group; */

/*           time_signature_dialog = gtk_dialog_new_with_buttons("Set time signature", NULL, */
/*                                                         GTK_DIALOG_MODAL, */
/*                                                         GTK_STOCK_CANCEL, GTK_RESPONSE_REJECT, */
/*                                                         GTK_STOCK_OK, GTK_RESPONSE_ACCEPT, */
/*                                                         NULL); */
/*           g_signal_connect(G_CALLBACK(time_signature_dialog), "destroy", */
/*                            G_CALLBACK(gtk_widget_destroyed), */
/*                            &time_signature_dialog); */
          
/*           /\* The main part *\/ */
/*           vbox = gtk_vbox_new(FALSE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(GTK_DIALOG(time_signature_dialog)->vbox), vbox); */
/*           gtk_widget_show(vbox); */

/*           /\* The widgets *\/ */

/*           /\* New hbox *\/ */
/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           radio_24 = gtk_radio_button_new_with_label(NULL, "2/4"); */
/*           group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radio_24)); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_24); */
/*           gtk_widget_show(radio_24); */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/time_signature/p_24.xpm")); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), pixmap_wid); */
/*           gtk_widget_show(pixmap_wid); */

/*           radio_68 = gtk_radio_button_new_with_label(group,"6/8"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_68); */
/*           gtk_widget_show(radio_68); */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/time_signature/p_68.xpm")); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), pixmap_wid); */
/*           gtk_widget_show(pixmap_wid); */

/*           hseparator = gtk_hseparator_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hseparator); */
/*           gtk_widget_show(hseparator); */

/*           /\* New hbox *\/ */
/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           radio_common_time = gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(radio_68),"Common Time"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_common_time); */
/*           gtk_widget_show(radio_common_time); */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/time_signature/common_time.xpm")); */
/*           gtk_widget_show(pixmap_wid); */

/*           radio_alla_breve = gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(radio_common_time),"Alla Breve"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_alla_breve); */
/*           gtk_widget_show(radio_alla_breve); */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/time_signature/alla_breve.xpm")); */
/*           gtk_widget_show(pixmap_wid); */

/*           hseparator = gtk_hseparator_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hseparator); */
/*           gtk_widget_show(hseparator); */

/*           /\* New hbox *\/ */
/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           radio_custom = gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(radio_alla_breve),"custom"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_custom); */
/*           gtk_widget_show(radio_custom); */

/*           NumeratorEntry = gtk_entry_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), NumeratorEntry); */
/*           gtk_widget_show(NumeratorEntry); */
          
/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           drawingarea = gtk_drawing_area_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), drawingarea); */
/*           gtk_widget_show(drawingarea); */

/*           DenominatorEntry = gtk_entry_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), DenominatorEntry); */
/*           gtk_widget_show(DenominatorEntry); */
          
/*           /\* Execute the OK button action *\/ */
/*           if (gtk_dialog_run(GTK_DIALOG(time_signature_dialog))==GTK_RESPONSE_ACCEPT) */
/*           { */
/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_24))) */
/*                     score->Staff[get_staff_selected()].time_signature = TIME_SIGNATURE_24; */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_68))) */
/*                     score->Staff[get_staff_selected()].time_signature = TIME_SIGNATURE_68; */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_common_time))) */
/*                     score->Staff[get_staff_selected()].time_signature = TIME_SIGNATURE_COMMON_TIME; */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_alla_breve))) */
/*                     score->Staff[get_staff_selected()].time_signature = TIME_SIGNATURE_ALLA_BREVE; */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_custom))) */
/*                { */
/*                     g_message("custom"); */
/*                } */

/*                score->Staff[get_staff_selected()].startx += 10; */

/*           } else */
/*           { */
/*                score->Staff[get_staff_selected()].time_signature = old_key_signature; */
/*           } */
/*           gtk_widget_destroy(time_signature_dialog); */
/*      } */

}

void staff_display_measures_numbers(gpointer callback_data, guint callback_action, GtkWidget *widget)
{
/*      static GtkWidget * display_measures_numbers_dialog = NULL; */

/*      if (display_measures_numbers_dialog) */
/*      { */
/*           if (!GTK_WIDGET_VISIBLE(display_measures_numbers_dialog)) */
/*                gtk_widget_show(display_measures_numbers_dialog); */
/*           else */
/*                gdk_window_raise(display_measures_numbers_dialog->window); */
/*      } else */
/*      { */
/*           GtkWidget * vbox; */
/*           GtkWidget * radio_never, * radio_all_staffs, * radio_only_this_staff; */
/*           GSList    * group = NULL; */
          
/*           display_measures_numbers_dialog = gtk_dialog_new_with_buttons("Display measures numbers", NULL, */
/*                                                                         GTK_DIALOG_MODAL, */
/*                                                                         GTK_STOCK_CANCEL, GTK_RESPONSE_REJECT, */
/*                                                                         GTK_STOCK_OK, GTK_RESPONSE_ACCEPT, */
/*                                                                         NULL); */
/*           g_signal_connect(G_CALLBACK(display_measures_numbers_dialog), "destroy", */
/*                            G_CALLBACK(gtk_widget_destroyed), */
/*                            &display_measures_numbers_dialog); */
          
/*           /\* The main part *\/ */
/*           vbox = gtk_vbox_new(FALSE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(GTK_DIALOG(display_measures_numbers_dialog)->vbox), vbox); */
/*           gtk_widget_show(vbox); */
          
/*           /\* the three radio buttons *\/ */
/*           radio_never = gtk_radio_button_new_with_label(group, */
/*                                                         "Never display measures numbers"); */
/*           group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radio_never)); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), radio_never); */
/*           gtk_widget_show(radio_never); */
/*           radio_all_staffs = gtk_radio_button_new_with_label(group, */
/*                                                             "Display measures numbers for all staffs"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), radio_all_staffs); */
/*           gtk_widget_show(radio_all_staffs); */
          
/*           radio_only_this_staff = gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(radio_all_staffs), */
/*                                                                               "Display measures numbers for this staff only"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), radio_only_this_staff); */
/*           gtk_widget_show(radio_only_this_staff); */
          
/*           if (gtk_dialog_run(GTK_DIALOG(display_measures_numbers_dialog))==GTK_RESPONSE_ACCEPT) */
/*           { */
/*                g_message("Apply!"); */
               
/*           } */
/*           gtk_widget_destroy(display_measures_numbers_dialog); */
/*      } */

}


gdouble get_staff_extremity_end_y(const Score_t *score, gint staff_id)
{
        Staff_t *staff_data;
        gdouble retval = 0;

        staff_data = g_list_nth_data(score->Staff_list, staff_id);

        if (staff_data) {
                retval += staff_data->extremity_begin_y;
                retval += (staff_data->nb_lines - 1) * staff_data->space_btw_lines + staff_data->nb_lines;
        return retval;
        }

        return 0;
}

/* Accordingly to the y position of the cursor, this function returns the staff number */
gint get_staff_id(Score_t *score, gdouble y)
{
        GList * listrunner;
        
        Staff_t *staff_data;
        guint staff_counter = 0;

        guint staff_id = 0;
 
        listrunner = g_list_first(score->Staff_list);

        while (listrunner) {

		staff_data = (Staff_t *)listrunner->data;

		if ( ! staff_data ) {
			debug_msg("|staff.c| get_staff_id(). Unable to browse score->Staff_list. Memory Exhausted ?\n");
		}

		if ( y >= staff_data->extremity_begin_y ) {
			if ( y <= get_staff_extremity_end_y(score, staff_counter) ) {
				staff_id = staff_counter;
			}
		}

		staff_counter++;
                                        
		listrunner = g_list_next(listrunner);
	}
        
        g_list_free(listrunner);


	return staff_id;

}

void deselect_all_staffs(void)
{
/*      gint i = 0; */

/*      for ( i = 0; i < score->nb_staves; i++ ) */
/*      { */
/*           score->Staff[i].is_selected = FALSE; */
/*      } */

}

extern gint
staff_remove_staff(gpointer callback_data, guint callback_action, GtkWidget *widget)
{
/*      gint i; */

/*      gint staff_number = get_staff_selected(); */

/*      score->nb_staves--; */

/*      for ( i = 0; i < score->nb_staves; i++) */
/*      { */
/*           if (i >= staff_number) */
/*           { */
/*                score->Staff[i].nb_lines = score->Staff[i+1].nb_lines; */
/*                score->Staff[i].space_btw_lines = score->Staff[i+1].space_btw_lines; */
/*                score->Staff[i].is_selected = score->Staff[i].is_selected; */
/*                score->Staff[i].key = score->Staff[i+1].key; */
/*                score->Staff[i].key_signature = score->Staff[i+1].key_signature; */
/*                score->Staff[i].time_signature = score->Staff[i+1].time_signature; */
/*                score->Staff[i].time_signature_x = score->Staff[i+1].time_signature_x; */
/*                score->Staff[i].endbar = score->Staff[i+1].endbar; */
/*                score->Staff[i].extremity_begin_x = score->Staff[i+1].extremity_begin_x; */
/*                score->Staff[i].extremity_end_x = score->Staff[i+1].extremity_end_x; */
/*                score->Staff[i].extremity_begin_y = score->Staff[i].extremity_begin_y; */
/*                score->Staff[i].extremity_end_y = score->Staff[i].extremity_begin_y - 1 +  */
/*                     (score->Staff[i].nb_lines + ((score->Staff[i].nb_lines - 1) * score->Staff[i].space_btw_lines)); */
/*                score->Staff[i].space_with_staff_up = score->Staff[i+1].space_with_staff_up; */
/* /\*                score->Staff[i].middle = score->Staff[i+1].extremity_begin_y + 17; *\/ */
/*                score->Staff[i].total_measures = score->Staff[i+1].total_measures; */
/*                score->Staff[i].startx = score->Staff[i+1].startx; */

/*                score->Staff[i].Object_list = g_list_copy(score->Staff[i+1].Object_list); */

/*           } */
/*      } */

/*      refresh(); */

     return 0;
}

extern gint
get_staff_key(const Score_t *score, gint staff)
{

/*      gint staff_type = -1; */
/*      gint i; */

/*      for ( i = 0; i < score->nb_staves; i++ ) */
/*           staff_type = score->Staff[staff].key; */

/*      return staff_type; */

	return 0;
}

/* void staff_set_midi_instrument(gpointer callback_data, guint callback_action, GtkWidget *widget) */
/* { */

/*      static GtkWidget * dialog = NULL; */

/*      if (dialog) */
/*      { */
/*           if (!GTK_WIDGET_VISIBLE(dialog)) */
/*                gtk_widget_show(dialog); */
/*           else */
/*                gdk_window_raise(dialog->window); */
/*      } else */
/*      { */
/*           GtkWidget * vbox; */
/*           GtkWidget * label; */
/*           GtkWidget * spinbutton; */

/*           GtkAdjustment * adj; */

          
/*           dialog = gtk_dialog_new_with_buttons("Set MIDI Instrument", NULL, */
/*                                                GTK_DIALOG_MODAL, */
/*                                                GTK_STOCK_CANCEL, GTK_RESPONSE_REJECT, */
/*                                                GTK_STOCK_OK, GTK_RESPONSE_ACCEPT, */
/*                                                NULL); */
/*           g_signal_connect(G_CALLBACK(dialog), "destroy", */
/*                            G_CALLBACK(gtk_widget_destroyed), */
/*                            &dialog); */
          
/*           /\* The main part *\/ */
/*           vbox = gtk_vbox_new(FALSE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(GTK_DIALOG(dialog)->vbox), vbox); */
/*           gtk_widget_show(vbox); */
          
/*           /\* the three radio buttons *\/ */
/*           label = gtk_label_new("Set MIDI Instrument (1..128)"); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), label); */
/*           gtk_widget_show(label); */


/*           adj = GTK_ADJUSTMENT(gtk_adjustment_new(score->Staff[get_staff_selected()].midi_instrument, */
/*                                                   1, 128, */
/*                                                   1.0, 10.0, 10.0 )); */

/*           spinbutton = gtk_spin_button_new(adj, 1.0, 0); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), spinbutton); */
/*           gtk_widget_show(spinbutton); */

/*           if (gtk_dialog_run(GTK_DIALOG(dialog))==GTK_RESPONSE_ACCEPT) */
/*           { */

/*                score->Staff[get_staff_selected()].midi_instrument = gtk_spin_button_get_value_as_int(GTK_SPIN_BUTTON(spinbutton)); */

/*           } */
/*           gtk_widget_destroy(dialog); */
/*      } */

/*      g_print("Set midi\n"); */
/* } */

void
staff_add_staff (gpointer callback_data, guint callback_action, GtkWidget *widget)
{

/*      static GtkWidget * add_staff_dialog = NULL; */

/*      if (add_staff_dialog) */
/*      { */
/*           if (!GTK_WIDGET_VISIBLE(add_staff_dialog)) */
/*                gtk_widget_show(add_staff_dialog); */
/*           else */
/*                gdk_window_raise(add_staff_dialog->window); */
/*      } else */
/*      { */

/*           GtkWidget * vbox, * hbox; */
/*           GtkWidget * radio_treble, * radio_bass, * radio_alto; */
/*           GtkWidget * pixmap_wid; */
/*           GtkWidget * hseparator; */

/*           GSList    * group; */


/*           add_staff_dialog = gtk_dialog_new_with_buttons("Add Staff", NULL, */
/*                                                         GTK_DIALOG_MODAL, */
/*                                                         GTK_STOCK_CANCEL, GTK_RESPONSE_REJECT, */
/*                                                         GTK_STOCK_OK, GTK_RESPONSE_ACCEPT, */
/*                                                         NULL); */
/*           g_signal_connect(G_CALLBACK(add_staff_dialog), "destroy", */
/*                            G_CALLBACK(gtk_widget_destroyed), */
/*                            &add_staff_dialog); */
          
/*           /\* The main part *\/ */
/*           vbox = gtk_vbox_new(FALSE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(GTK_DIALOG(add_staff_dialog)->vbox), vbox); */
/*           gtk_widget_show(vbox); */

/*           /\* The widgets *\/ */
/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           radio_treble = gtk_radio_button_new_with_label(NULL, "Treble key"); */
/*           group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radio_treble)); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_treble); */
/*           gtk_widget_show(radio_treble); */

/* /\*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/lilypond/clefs-G.xpm")); *\/ */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/clefs/clefs-G.xpm")); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), pixmap_wid); */
/*           gtk_widget_show(pixmap_wid); */

/*           hseparator = gtk_hseparator_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hseparator); */
/*           gtk_widget_show(hseparator); */

/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           radio_bass = gtk_radio_button_new_with_label(group, "Bass key"); */
/*           group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radio_bass)); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_bass); */
/*           gtk_widget_show(radio_bass); */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/clefs/clefs-F.xpm")); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), pixmap_wid); */
/*           gtk_widget_show(pixmap_wid); */

/*           hseparator = gtk_hseparator_new(); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hseparator); */
/*           gtk_widget_show(hseparator); */

/*           hbox = gtk_hbox_new(TRUE, 4); */
/*           gtk_box_pack_start_defaults(GTK_BOX(vbox), hbox); */
/*           gtk_widget_show(hbox); */

/*           radio_alto = gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(radio_bass), "Alto key"); */
/*           group = gtk_radio_button_get_group(GTK_RADIO_BUTTON(radio_alto)); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), radio_alto); */
/*           gtk_widget_show(radio_alto); */

/*           pixmap_wid = gtk_image_new_from_file(get_file_from_data_dir("pixmaps/clefs/clefs-C.xpm")); */
/*           gtk_box_pack_start_defaults(GTK_BOX(hbox), pixmap_wid); */
/*           gtk_widget_show(pixmap_wid); */

/*           /\* Execute the OK button action *\/ */
/*           if (gtk_dialog_run(GTK_DIALOG(add_staff_dialog))==GTK_RESPONSE_ACCEPT) */
/*           { */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_treble))) { */

/*                     score->Staff[score->nb_staves].endbar = TRUE; */
/*                     score->Staff[score->nb_staves].is_selected = FALSE; */
/*                     score->Staff[score->nb_staves].nb_lines = 5; */
/*                     score->Staff[score->nb_staves].space_btw_lines = 8; */
/*                     score->Staff[score->nb_staves].space_with_staff_up = 50; */
/*                     score->Staff[score->nb_staves].key = TREBLE_KEY; */
/*                     score->Staff[score->nb_staves].extremity_begin_x = score->Staff[score->nb_staves-1].extremity_begin_x; */
/*                     score->Staff[score->nb_staves].extremity_begin_y = score->Staff[score->nb_staves-1].extremity_end_y + score->Staff[score->nb_staves].space_with_staff_up; */
/*                     score->Staff[score->nb_staves].extremity_end_y   = score->Staff[score->nb_staves].extremity_begin_y+36; */
/* /\*                     score->Staff[score->nb_staves].middle = score->Staff[score->nb_staves].extremity_begin_y+17; *\/ */
/*                     score->Staff[score->nb_staves].extremity_end_x = score->Staff[score->nb_staves-1].extremity_end_x; */
/*                     score->Staff[score->nb_staves].key_signature = KEY_SIGNATURE_TREBLE_EMPTY; */
/*                     score->Staff[score->nb_staves].startx = 100; */
/*                     score->Staff[score->nb_staves].time_signature_x = 70; */

/*                     score->Staff[score->nb_staves].Object_list = NULL; */

/*                     score->height = score->Staff[score->nb_staves].extremity_end_y + 100; */

/*                     deselect_all_staffs(); */
/*                     score->Staff[score->nb_staves].is_selected = TRUE; */
/*                     score_new_size();  */
                    
/*                     score->nb_staves++; */

/*                } */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_bass))) { */

/*                     score->Staff[score->nb_staves].endbar = TRUE; */
/*                     score->Staff[score->nb_staves].is_selected = FALSE; */
/*                     score->Staff[score->nb_staves].nb_lines = 5; */
/*                     score->Staff[score->nb_staves].space_btw_lines = 8; */
/*                     score->Staff[score->nb_staves].space_with_staff_up = 50; */
/*                     score->Staff[score->nb_staves].key = BASS_KEY; */
/*                     score->Staff[score->nb_staves].extremity_begin_x = score->Staff[score->nb_staves-1].extremity_begin_x; */
/*                     score->Staff[score->nb_staves].extremity_begin_y = score->Staff[score->nb_staves-1].extremity_end_y + score->Staff[score->nb_staves].space_with_staff_up; */
/*                     score->Staff[score->nb_staves].extremity_end_y   = score->Staff[score->nb_staves].extremity_begin_y+36; */
/* /\*                     score->Staff[score->nb_staves].middle = score->Staff[score->nb_staves].extremity_begin_y+17; *\/ */
/*                     score->Staff[score->nb_staves].extremity_end_x = score->Staff[score->nb_staves-1].extremity_end_x; */
/*                     score->Staff[score->nb_staves].key_signature = KEY_SIGNATURE_TREBLE_EMPTY; */
/*                     score->Staff[score->nb_staves].startx = 100; */
/*                     score->Staff[score->nb_staves].time_signature_x = 70; */

/*                     score->Staff[score->nb_staves].Object_list = NULL; */

/*                     score->height = score->Staff[score->nb_staves].extremity_end_y + 100; */
                    
/*                     score->nb_staves++; */

/*                } */

/*                if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(radio_alto))) { */

/*                     score->Staff[score->nb_staves].endbar = TRUE; */
/*                     score->Staff[score->nb_staves].is_selected = FALSE; */
/*                     score->Staff[score->nb_staves].nb_lines = 5; */
/*                     score->Staff[score->nb_staves].space_btw_lines = 8; */
/*                     score->Staff[score->nb_staves].space_with_staff_up = 50; */
/*                     score->Staff[score->nb_staves].key = ALTO_KEY; */
/*                     score->Staff[score->nb_staves].extremity_begin_x = score->Staff[score->nb_staves-1].extremity_begin_x; */
/*                     score->Staff[score->nb_staves].extremity_begin_y = score->Staff[score->nb_staves-1].extremity_end_y + score->Staff[score->nb_staves].space_with_staff_up; */
/*                     score->Staff[score->nb_staves].extremity_end_y   = score->Staff[score->nb_staves].extremity_begin_y+36; */
/* /\*                     score->Staff[score->nb_staves].middle = score->Staff[score->nb_staves].extremity_begin_y+17; *\/ */
/*                     score->Staff[score->nb_staves].extremity_end_x = score->Staff[score->nb_staves-1].extremity_end_x; */
/*                     score->Staff[score->nb_staves].key_signature = KEY_SIGNATURE_TREBLE_EMPTY; */
/*                     score->Staff[score->nb_staves].startx = 100; */
/*                     score->Staff[score->nb_staves].time_signature_x = 70; */

/*                     score->Staff[score->nb_staves].Object_list = NULL; */

/*                     score->height = score->Staff[score->nb_staves].extremity_end_y + 100; */
                    
/*                     score->nb_staves++; */
                    
/*                } */
               
/*                gtk_widget_destroy(add_staff_dialog); */
/*           } else */
/*           { */
/*                gtk_widget_destroy(add_staff_dialog); */
/*           } */
     
/*      } */

/*      refresh(); */

}

extern 
void update_key_signature(GtkButton *widget, gpointer user_data)
{

        Score_t *score;
        Staff_t *staff_data;
	GtkWidget *area;

        score = score_get_from_widget(GTK_WIDGET(widget));
        area  = score_get_area_from_widget(GTK_WIDGET(widget));

	gint offset = 0;


        staff_data = g_list_nth_data(score->Staff_list, get_staff_selected(score));

        if ( ! staff_data ) return;

	staff_data->key_signature = key_signature;

	switch(key_signature) {
	case KEY_SIGNATURE_TREBLE_EMPTY:
		/* We don't need to update the spacing here :) */
		break;
	case KEY_SIGNATURE_TREBLE_A_SHARP:
		offset += (5 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_B_SHARP:
		offset += (7 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_C_SHARP:
		offset += (2 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_D_SHARP:
		offset += (6 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_E_SHARP:
		offset += (4 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_F_SHARP:
		offset += (1 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_G_SHARP:
		offset += (3 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_A_FLAT:
		offset += (3 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_B_FLAT:
		offset += (1 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_C_FLAT:
		offset += (6 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_D_FLAT:
		offset += (4 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_E_FLAT:
		offset += (2 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_F_FLAT:
		offset += (7 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	case KEY_SIGNATURE_TREBLE_G_FLAT:
		offset += (5 * Spacings.KeySignatures.sbksa) + Spacings.KeySignatures.saks;
		break;
	}

	score->staff_extremity_end_x += offset;
        staff_set_current_x(score, get_staff_selected(score),
			    staff_get_current_x(score, get_staff_selected(score)) + offset);
	staff_set_start_x(score, get_staff_selected(score), offset);

	/* TODO: find a way to refresh ALL drawing areas displaying score */
	refresh(area);
}

extern
void staff_change_key_callback(void)
{
/*         GtkWidget *widget; */

/*         widget = glade_xml_get_widget(gladexml, "changekey_treble_rb"); */
/* 	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)) ) */
/* 		add_object(get_staff_selected(), */
/* 			   TREBLE_KEY, NULL, NULL, 0, */
/* 			   NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, FALSE); */

/*         widget = glade_xml_get_widget(gladexml, "changekey_bass_rb"); */
/* 	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)) ) */
/* 		add_object(get_staff_selected(), */
/* 			   BASS_KEY, NULL, NULL, 0, */
/* 			   NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, FALSE); */

/*         widget = glade_xml_get_widget(gladexml, "changekey_alto_rb"); */
/* 	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)) ) */
/* 		add_object(get_staff_selected(), */
/* 			   ALTO_KEY, NULL, NULL, 0, */
/* 			   NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, FALSE); */

/*         widget = glade_xml_get_widget(gladexml, "changekey_tenor_rb"); */
/* 	if ( gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)) ) */
/* 		add_object(get_staff_selected(), */
/* 			   TENOR_KEY, NULL, NULL, 0, */
/* 			   NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, FALSE); */

/* 	gtk_widget_set_size_request(GTK_WIDGET(score->area), score->width, score->height); */

/* 	refresh(area); */
}

Staff_t *staff_selected_get_from_score(Score_t *score)
{
        Staff_t *tmpstaff;

        tmpstaff = (Staff_t *)
                g_list_nth_data(score->Staff_list, 
                                get_staff_selected(score));
                                
        if ( ! tmpstaff ) return NULL;

        return tmpstaff;
}
