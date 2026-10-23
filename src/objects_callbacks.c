/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * objects_callbacks.c: callbacks for the toolbar and the menu
 * gscore - a music score editor
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

#include "gscore.h"
#include "constants.h"
#include "common.h"
#include "draw.h"
#include "objects.h"
#include "score.h"
#include "objects_callbacks.h"
#include "staff.h"

#include "cursors.h"


void object_cursor_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area= score_get_area_from_widget(widget);
  
	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = CURSOR;

	/* 	clean_statusbar(); */

}

void object_doublewhole_callback(GtkWidget *widget, GdkEventExpose *event)
{
  Score_t *score = score_get_from_widget(widget);
  GtkWidget *area = score_get_area_from_widget(widget);
  
	if ( object_selected_change_type(score, DOUBLEWHOLE) ) {
		refresh(area);
	} else {
		draw_cursor(area, cursor_doublewhole_bits, cursor_doublewhole_mask_bits, 16, 8);
		Selection.object_type = DOUBLEWHOLE;
	}

}

void object_whole_callback(GtkWidget *widget, GdkEventExpose *event)
{
  Score_t *score = score_get_from_widget(widget);
  GtkWidget *area = score_get_area_from_widget(widget);

	if ( object_selected_change_type(score, WHOLE) ) {
		refresh(area);
	} else {
		draw_cursor(area, cursor_whole_bits, cursor_whole_mask_bits, 14, 8);
		Selection.object_type = WHOLE;
	}

}

void object_half_callback(GtkWidget *widget, GdkEventExpose *event)
{
  Score_t *score = score_get_from_widget(widget);
  GtkWidget *area = score_get_area_from_widget(widget);
  
	if ( object_selected_change_type(score, HALF) ) {
		refresh(area);
	} else {
		draw_cursor(area, cursor_half_bits, cursor_half_bits, 10, 29);
		Selection.object_type = HALF;
	}
  
}

void object_quarter_callback(GtkWidget *widget, GdkEventExpose *event)
{
  Score_t *score = score_get_from_widget(widget);
  GtkWidget *area = score_get_area_from_widget(widget);

	if ( object_selected_change_type(score, QUARTER) ) {
		refresh(area);
	} else {
        draw_cursor(area, cursor_quarter_bits, cursor_quarter_mask_bits, 9, 29);
	Selection.object_type = QUARTER;
	}

}


void object_eighth_callback(GtkWidget *widget, GdkEventExpose *event)
{
  Score_t *score = score_get_from_widget(widget);
  GtkWidget *area = score_get_area_from_widget(widget);

	if ( object_selected_change_type(score, EIGHTH) ) {
		refresh(area);
	} else {
		draw_cursor(area, cursor_eighth_bits, cursor_eighth_mask_bits, 15, 29);
		Selection.object_type = EIGHTH;
	}

}

void object_sixteenth_callback(GtkWidget *widget, GdkEventExpose *event)
{
  Score_t *score = score_get_from_widget(widget);
  GtkWidget *area = score_get_area_from_widget(widget);

	if ( object_selected_change_type(score, SIXTEENTH) ) {
		refresh(area);
	} else {
		draw_cursor(area, cursor_sixteenth_bits, cursor_sixteenth_mask_bits, 15, 29);
		Selection.object_type = SIXTEENTH;
	}

}

void object_thirtysecond_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, ic_thirtysecond_bits, ic_thirtysecond_mask_bits, 22, 22);

	/*      Selection.object = THIRTYSECOND; */
	/*      Selection.object_type = THIRTYSECOND; */

	/*      clean_statusbar(); */
}

void object_sixtyfourth_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);
	draw_cursor(area, ic_sixtyfourth_bits, ic_sixtyfourth_mask_bits, 22, 22);

	/*      Selection.object = SIXTYFOURTH; */
	/*      Selection.object_type = SIXTYFOURTH; */

	/*      clean_statusbar(); */
}

void object_doublewholerest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, cursor_doublewholerest_bits, cursor_doublewholerest_mask_bits, 20, 10);

	/*      Selection.object = DOUBLEWHOLEREST; */
	Selection.object_type = DOUBLEWHOLEREST;

	/*      clean_statusbar(); */
}

void object_wholerest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, cursor_wholerest_bits, cursor_wholerest_mask_bits, 20, 6);

	/*      Selection.object = WHOLEREST; */
	Selection.object_type = WHOLEREST;

	/*      clean_statusbar(); */
}

void object_halfrest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, cursor_halfrest_bits, cursor_halfrest_mask_bits, 20, 6);
	
	Selection.object_type = HALFREST;
	
	/* 	clean_statusbar(); */
}

void object_quarterrest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, cursor_quarterrest_bits, cursor_quarterrest_mask_bits, 8, 20);

	Selection.object_type = QUARTERREST;
	
	/* 	clean_statusbar(); */

	/* 	update_statusbar("toolbar","Click where you want to put the quarter rest"); */

}

void object_eighthrest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, cursor_eighthrest_bits, cursor_eighthrest_mask_bits, 7, 13);
	
	Selection.object_type = EIGHTHREST;
	
	/* 	clean_statusbar(); */
}

void object_sixteenthrest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, cursor_sixteenthrest_bits, cursor_sixteenthrest_mask_bits, 10, 20);

	Selection.object_type = SIXTEENTHREST;

	/*   clean_statusbar(); */
}

void object_thirtysecondrest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

     draw_cursor(area, cursor_thirty_secondrest_bits, cursor_thirty_secondrest_mask_bits, 11, 27);

/*      Selection.object_type = THIRTYSECONDREST; */


/*   clean_statusbar(); */
}

void object_sixtyfourthrest_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

     draw_cursor(area, cursor_sixtyfourthrest_bits, cursor_sixtyfourthrest_mask_bits, 12, 34);

/*      Selection.object_type = SIXTYFOURTHREST; */


/*   clean_statusbar(); */
}


void object_sharp_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	GtkWidget *btnw;
        gboolean something_selected = FALSE;


	btnw = glade_xml_get_widget(gladexml, "mw_sharp_tb");

	if (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))
		Selection.accidentals |= A_SHARP;
	else
		Selection.accidentals -= A_SHARP;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;

                if ((note->is_selected) && (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))) {
                        note->accidentals = 0;
                        note->accidentals |= A_SHARP;

                        something_selected = TRUE;
                }

                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

        if (something_selected) gtk_toggle_tool_button_set_active(GTK_TOGGLE_TOOL_BUTTON(btnw), FALSE);

	refresh(area);

}
void object_doublesharp_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	GtkWidget *btnw;
        gboolean something_selected = FALSE;


	btnw = glade_xml_get_widget(gladexml, "mw_doublesharp_tb");

	if (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))
		Selection.accidentals |= A_DOUBLESHARP;
	else
		Selection.accidentals -= A_DOUBLESHARP;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;
                
                if ((note->is_selected) && (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))) {
                        note->accidentals = 0;
                        note->accidentals |= A_DOUBLESHARP;

                        something_selected = TRUE;
                }
                                  
                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

        if (something_selected) gtk_toggle_tool_button_set_active(GTK_TOGGLE_TOOL_BUTTON(btnw), FALSE);

	refresh(area);

}

void object_flat_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	GtkWidget *btnw;
        gboolean something_selected = FALSE;

	btnw = glade_xml_get_widget(gladexml, "mw_flat_tb");

	if (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))
		Selection.accidentals |= A_FLAT;
	else
		Selection.accidentals -= A_FLAT;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;
                
                if ((note->is_selected) && (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))) {
                        note->accidentals = 0;
                        note->accidentals |= A_FLAT;

                        something_selected = TRUE;
                }
                                  
                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

        if (something_selected) gtk_toggle_tool_button_set_active(GTK_TOGGLE_TOOL_BUTTON(btnw), FALSE);

	refresh(area);

}

void object_doubleflat_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	GtkWidget *btnw;
        gboolean something_selected = FALSE;


	btnw = glade_xml_get_widget(gladexml, "mw_doubleflat_tb");

	if (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))
		Selection.accidentals |= A_DOUBLEFLAT;
	else
		Selection.accidentals -= A_DOUBLEFLAT;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;

                if ((note->is_selected) && (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))) {
                        note->accidentals = 0;
                        note->accidentals |= A_DOUBLEFLAT;

                        something_selected = TRUE;
                }
                                  
                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

        if (something_selected) gtk_toggle_tool_button_set_active(GTK_TOGGLE_TOOL_BUTTON(btnw), FALSE);

	refresh(area);

}

void object_natural_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	GtkWidget *btnw;
        gboolean something_selected = FALSE;

	btnw = glade_xml_get_widget(gladexml, "mw_natural_tb");

	if (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))
		Selection.accidentals |= A_NATURAL;
	else
		Selection.accidentals -= A_NATURAL;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;
                
                if ((note->is_selected) && (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))) {
                        note->accidentals = 0;
                        note->accidentals |= A_NATURAL;

                        something_selected = TRUE;
                }
                                  
                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

        if (something_selected) gtk_toggle_tool_button_set_active(GTK_TOGGLE_TOOL_BUTTON(btnw), FALSE);

	refresh(area);

}

void object_parenthesis_callback(GtkWidget *widget, GdkEventExpose *event)
{
	GtkWidget *btnw;

	btnw = glade_xml_get_widget(gladexml, "mw_parenthesis_tb");

	if (gtk_toggle_tool_button_get_active(GTK_TOGGLE_TOOL_BUTTON(btnw)))
		Selection.accidentals |= A_PARENTHESIS;
	else
		Selection.accidentals -= A_PARENTHESIS;

}

void object_beam_callback(GtkWidget *widget, GdkEventExpose *event)
{

/*      Selection.beam = gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)); */

/*      clean_statusbar(); */
}


void object_tie_callback(GtkWidget *widget, GdkEventExpose *event)
{

/*      Selection.tie = gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)); */
/*      update_statusbar("toolbar","Sorry, this function is not coded yet"); */

/*      clean_statusbar(); */
}

void object_autoupdown_callback(GtkWidget *widget, GdkEventExpose *event)
{

/*      Selection.autoupdown = gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)); */
/*      update_statusbar("toolbar","Automaticaly steam up or down accordingly to your staff"); */

/*      clean_statusbar(); */
}

void object_up_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;

                if (note->is_selected) {
                        note->nature &= O_STEMUP;
                        note->nature &= O_STEMDOWN;

                        note->nature |= O_STEMUP;
                }
                                  
                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

	refresh(area);

}

void object_down_callback(GtkWidget *widget, GdkEventExpose *event)
{

	Staff_t *staff_data;
	Score_t *score;
	GtkWidget *area;
	
	GList *listrunner;

	score = score_get_from_widget(widget);
	area = score_get_area_from_widget(widget);

	staff_data = (Staff_t *) g_list_nth_data(score->Staff_list,
						 get_staff_selected(score));

	listrunner = g_list_first(staff_data->Object_list);

	while (listrunner) {

                Object_t *note;

                note = (Object_t *)listrunner->data;

                if (note->is_selected) {
                        note->nature &= O_STEMUP;
                        note->nature &= O_STEMDOWN;

                        note->nature |= O_STEMDOWN;
                }
                                  
                listrunner = g_list_next(listrunner);

        }

	g_list_free(listrunner);

	refresh(area);

}

void object_duration_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_DURATION;
}

void object_noarticulation_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
}

void object_staccato_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_STACCATO;
}

void object_marcatoup_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_MARCATOUP;
}

void object_marcatodown_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_MARCATODOWN;
}

void object_accent_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_ACCENT;
}

void object_staccatissimo_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_STACCATISSIMO;
}

void object_tenuto_callback(GtkWidget *widget, GdkEventExpose *event)
{
	Selection.nature &= O_NONE;
	Selection.nature |= O_TENUTO;
}

void object_staccato_op_callback(GtkWidget *widget, GdkEventExpose *event)
{

/*      Selection.staccato_op = gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(widget)); */
/*      update_statusbar("toolbar","Sorry, this function is not coded yet"); */

/*      clean_statusbar(); */
}

void object_barline_single_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = BARLINE_SINGLE;

}

void object_barline_double_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = BARLINE_DOUBLE;

}

void object_barline_openrepeat_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = BARLINE_OPENREPEAT;

}

void object_barline_closerepeat_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = BARLINE_CLOSEREPEAT;

}

void object_barline_opencloserepeat_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = BARLINE_OPENCLOSEREPEAT;

}


/***
 *** DYNAMICS
 ***/
void object_pppp_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_PPPP;

}

void object_ppp_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_PPP;

}

void object_pp_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);
	Selection.object_type = DYNAMIC_PP;

}

void object_p_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_P;

}

void object_mp_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_MP;

}

void object_mf_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_MF;

}

void object_f_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_F;

}

void object_ff_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_FF;

}

void object_fff_callback(GtkWidget *widget, GdkEventExpose *event)
{
  GtkWidget *area = score_get_area_from_widget(widget);

	draw_cursor(area, mouse_bits, mouse_mask_bits, 32, 32);

	Selection.object_type = DYNAMIC_FFF;

}

void object_notes_show_callback(GtkWidget *widget, GdkEventExpose *event)
{
	GtkWidget *btnw;
	GtkWidget *mw_handlebox1;
	GtkWidget *mw_handlebox2;


	btnw = glade_xml_get_widget(gladexml, "mw_notes_toggle");

	mw_handlebox1 = glade_xml_get_widget(gladexml, "mw_handlebox1");
	mw_handlebox2 = glade_xml_get_widget(gladexml, "mw_handlebox2");

	if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(btnw))) {
		gtk_widget_show(mw_handlebox1);
		gtk_widget_show(mw_handlebox2);
	} else {
		gtk_widget_hide(mw_handlebox1);
		gtk_widget_hide(mw_handlebox2);
	}


}

void object_barlines_show_callback(GtkWidget *widget, GdkEventExpose *event)
{
	GtkWidget *btnw;
	GtkWidget *mw_handlebox3;


	btnw = glade_xml_get_widget(gladexml, "mw_barlines_toggle");

	mw_handlebox3 = glade_xml_get_widget(gladexml, "mw_handlebox3");

	if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(btnw))) {
		gtk_widget_show(mw_handlebox3);
	} else {
		gtk_widget_hide(mw_handlebox3);
	}

}

void object_dynamics_show_callback(GtkWidget *widget, GdkEventExpose *event)
{
	GtkWidget *btnw;
	GtkWidget *mw_handlebox4;
	GtkWidget *mw_handlebox5;


	btnw = glade_xml_get_widget(gladexml, "mw_dynamics_toggle");

	mw_handlebox4 = glade_xml_get_widget(gladexml, "mw_handlebox4");
	mw_handlebox5 = glade_xml_get_widget(gladexml, "mw_handlebox5");

	if (gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(btnw))) {
		gtk_widget_show(mw_handlebox4);
		gtk_widget_show(mw_handlebox5);
	} else {
		gtk_widget_hide(mw_handlebox4);
		gtk_widget_hide(mw_handlebox5);
	}

}

