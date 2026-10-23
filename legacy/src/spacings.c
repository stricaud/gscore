/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * spacings.c
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
#include "spacings.h"
#include "constants.h"
#include "common.h"
#include "score.h"

Spacings_t Spacings;

extern gint
get_key_signature_spacing(gint key_signature)
{

        gint retval = 0;

        switch(key_signature)
                {
                case KEY_SIGNATURE_TREBLE_EMPTY:
                        retval = 0;
                        break;
                case KEY_SIGNATURE_TREBLE_A_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_B_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_C_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_D_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_E_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_F_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_G_SHARP:
                        break;
                case KEY_SIGNATURE_TREBLE_A_FLAT:
                        break;
                case KEY_SIGNATURE_TREBLE_B_FLAT:
                        break;
                case KEY_SIGNATURE_TREBLE_C_FLAT:
                        break;
                case KEY_SIGNATURE_TREBLE_D_FLAT:
                        break;
                case KEY_SIGNATURE_TREBLE_E_FLAT:
                        break;
                case KEY_SIGNATURE_TREBLE_F_FLAT:
                        break;
                case KEY_SIGNATURE_TREBLE_G_FLAT:
                        break;
                default:
                        g_critical("Cannot guess the spacing of the key signature!");
                }

        return retval;
}

extern 
void set_spacings_init(void)
{
	/* The values are set in pixels, then converted in inches/cm/whatever if requested */

	/* Augmentation Dots */
	Spacings.AugmentationDots.sbdan = 4;
	Spacings.AugmentationDots.sbd = 1;
	Spacings.AugmentationDots.vaod = 1;

	/* Barlines */
	Spacings.Barlines.hlt     = 4;
	Spacings.Barlines.tlt     = 1;
	Spacings.Barlines.sbhatl  = 3;
	Spacings.Barlines.swdb    = 3;
	Spacings.Barlines.sblad   = 4;
	Spacings.Barlines.sbd     = 3;
	Spacings.Barlines.sab     = 35;

	/* Beams */
	Spacings.Beams.bt = 3;

	/* Clefs */
	Spacings.Clefs.sb = 5;
	Spacings.Clefs.sa = 30;

	/* Key Signatures */
	Spacings.KeySignatures.sbksa = 8;
	Spacings.KeySignatures.saks = 20;

        /* Measures */
        Spacings.Measures.xpsfm = 35;
        Spacings.Measures.ypsfm = 70;
        Spacings.Measures.slbm = 50;

	/* Notes and Rests */
	Spacings.NotesRests.sa_doublewhole      = 100;
	Spacings.NotesRests.sa_doublewholerest  = 100;
	Spacings.NotesRests.sa_whole            = 80;
	Spacings.NotesRests.sa_wholerest        = 80;
	Spacings.NotesRests.sa_half             = 50;
	Spacings.NotesRests.sa_halfrest         = 50;
	Spacings.NotesRests.sa_quarter          = 35;
	Spacings.NotesRests.sa_quarterrest      = 35;
	Spacings.NotesRests.sa_eighth           = 25;
	Spacings.NotesRests.sa_eighthrest       = 25;
	Spacings.NotesRests.sa_sixteenth        = 25;
	Spacings.NotesRests.sa_sixteenthrest    = 25;
	Spacings.NotesRests.sa_thirtysecond     = 25;
	Spacings.NotesRests.sa_thirtysecondrest = 25;
	Spacings.NotesRests.sa_sixtyfourth      = 25;
	Spacings.NotesRests.sa_sixtyfourthrest  = 25;

        /* Tempo */
        Spacings.Tempo.xpfm = 5;
        Spacings.Tempo.ypfm = 40;

	/* Time Signatures */
	Spacings.TimeSignatures.sats = 5;
}

extern 
void set_values_spacings_spinbuttons(void)
{
	GtkWidget *widget;
	GtkWidget *display;

	display = glade_xml_get_widget(gladexml, "spcg_rests");
	widget = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");
	gtk_widget_set_sensitive(display, !gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (widget)));

	/* Barlines */
	widget = glade_xml_get_widget(gladexml, "spcg_bl_hlt");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.hlt);
	widget = glade_xml_get_widget(gladexml, "spcg_bl_tlt");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.tlt);
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sbhatl");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.sbhatl);
	widget = glade_xml_get_widget(gladexml, "spcg_bl_swdb");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.swdb);
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sblad");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.sblad);
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sbd");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.sbd);
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sab");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Barlines.sab);

	/* Clefs */
	widget = glade_xml_get_widget(gladexml, "spcg_cl_sb");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Clefs.sb);
	widget = glade_xml_get_widget(gladexml, "spcg_cl_sa");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Clefs.sa);

	/* Key Signatures */
	widget = glade_xml_get_widget(gladexml, "spcg_ks_sbksa");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.KeySignatures.sbksa);
	widget = glade_xml_get_widget(gladexml, "spcg_ks_saks");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.KeySignatures.saks);

        /* Measures */
	widget = glade_xml_get_widget(gladexml, "spcg_m_xpsfm");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Measures.xpsfm);
	widget = glade_xml_get_widget(gladexml, "spcg_m_ypsfm");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Measures.ypsfm);
	widget = glade_xml_get_widget(gladexml, "spcg_m_slbm");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Measures.slbm);

	/* Notes and Rests */
	widget = glade_xml_get_widget(gladexml, "spcg_nr_doublewhole");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_doublewhole);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_doublewholerest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_doublewholerest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_whole");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_whole);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_wholerest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_wholerest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_half");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_half);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_halfrest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_halfrest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_quarter");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_quarter);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_quarterrest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_quarterrest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_eighth");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_eighth);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_eighthrest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_eighthrest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixteenth");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_sixteenth);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixteenthrest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_sixteenthrest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_thirtysecond");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_thirtysecond);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_thirtysecondrest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_thirtysecondrest);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixtyfourth");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_sixtyfourth);
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixtyfourthrest");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.NotesRests.sa_sixtyfourthrest);

        /* Tempo */
	widget = glade_xml_get_widget(gladexml, "spcg_t_xpfm");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Tempo.xpfm);
	widget = glade_xml_get_widget(gladexml, "spcg_t_ypfm");
	gtk_spin_button_set_value(GTK_SPIN_BUTTON(widget), Spacings.Tempo.ypfm);
	
}

extern 
void update_spacings_from_spinbuttons(void)
{
	GtkWidget *widget;
	GtkWidget *area;

	/* Barlines */
	widget = glade_xml_get_widget(gladexml, "spcg_bl_hlt");
	Spacings.Barlines.hlt = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_bl_tlt");
	Spacings.Barlines.tlt = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sbhatl");
	Spacings.Barlines.sbhatl = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_bl_swdb");
	Spacings.Barlines.swdb = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sblad");
	Spacings.Barlines.sblad = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sbd");
	Spacings.Barlines.sbd = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_bl_sab");
	Spacings.Barlines.sab = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));

	/* Clefs */
	widget = glade_xml_get_widget(gladexml, "spcg_cl_sb");
	Spacings.Clefs.sb = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_cl_sa");
	Spacings.Clefs.sa = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));

	/* Key Signatures */
	widget = glade_xml_get_widget(gladexml, "spcg_ks_sbksa");
	Spacings.KeySignatures.sbksa = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_ks_saks");
	Spacings.KeySignatures.saks = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));

        /* Measures */
	widget = glade_xml_get_widget(gladexml, "spcg_m_xpsfm");
	Spacings.Measures.xpsfm = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_m_ypsfm");
	Spacings.Measures.ypsfm = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_m_slbm");
	Spacings.Measures.slbm = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));


	/* Notes and Rests */
	widget = glade_xml_get_widget(gladexml, "spcg_nr_doublewhole");
	Spacings.NotesRests.sa_doublewhole = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_doublewholerest");
	Spacings.NotesRests.sa_doublewholerest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_whole");
	Spacings.NotesRests.sa_whole  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_wholerest");
	Spacings.NotesRests.sa_wholerest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_half");
	Spacings.NotesRests.sa_half  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_halfrest");
	Spacings.NotesRests.sa_halfrest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_quarter");
	Spacings.NotesRests.sa_quarter  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_quarterrest");
	Spacings.NotesRests.sa_quarterrest = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_eighth");
	Spacings.NotesRests.sa_eighth  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_eighthrest");
	Spacings.NotesRests.sa_eighthrest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixteenth");
	Spacings.NotesRests.sa_sixteenth  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixteenthrest");
	Spacings.NotesRests.sa_sixteenthrest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_thirtysecond");
	Spacings.NotesRests.sa_thirtysecond  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_thirtysecondrest");
	Spacings.NotesRests.sa_thirtysecondrest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixtyfourth");
	Spacings.NotesRests.sa_sixtyfourth  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_nr_sixtyfourthrest");
	Spacings.NotesRests.sa_sixtyfourthrest  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));

        /* Tempo */
	widget = glade_xml_get_widget(gladexml, "spcg_t_xpfm");
	Spacings.Tempo.xpfm  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));
	widget = glade_xml_get_widget(gladexml, "spcg_t_ypfm");
	Spacings.Tempo.ypfm  = gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget));

	area = score_get_area_from_widget(widget);
	
	refresh(area);
}


extern
void on_spcg_nr_nlwr_toggled (GtkWidget *widget, gpointer data)
{
	GtkWidget *display;

	display = glade_xml_get_widget(gladexml, "spcg_rests");
	gtk_widget_set_sensitive(display, !gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (widget)));
}


extern
void on_spcg_nr_doublewhole_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_doublewhole");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_doublewholerest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_whole_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_whole");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_wholerest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_half_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_half");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_halfrest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_quarter_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_quarter");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_quarterrest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_eighth_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_eighth");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_eighthrest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_sixteenth_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_sixteenth");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_sixteenthrest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_thirtysecond_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_thirtysecond");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_thirtysecondrest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}

extern
void on_spcg_nr_sixtyfourth_changed(void)
{
	GtkWidget *togglew;
	GtkWidget *widget;
	GtkWidget *widgetrest;

	togglew = glade_xml_get_widget(gladexml, "spcg_nr_nlwr");

	if (gtk_toggle_button_get_active (GTK_TOGGLE_BUTTON (togglew))) {
		widget = glade_xml_get_widget(gladexml, "spcg_nr_sixtyfourth");
		widgetrest = glade_xml_get_widget(gladexml, "spcg_nr_sixtyfourthrest");

		gtk_spin_button_set_value(GTK_SPIN_BUTTON(widgetrest), 
					  gtk_spin_button_get_value(GTK_SPIN_BUTTON(widget)));
	}
}
