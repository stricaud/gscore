/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * gscore.h
 * gscore - a musical score editor
 *
 * (C) Copyright 2001-2005 Sebastien Tricaud
 * e-mail : toady@gscore.og
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
 * Score structure and Clipboard related things 
 */

#ifndef _GSCORE_H_
#define _GSCORE_H_

#include <glib.h>
#include <gtk/gtkwidget.h>
#include <glade/glade.h>

/* Set debug to 0 to avoid the g_messages */
#define DEBUG 0

#define VERSION "0.0.9"

/* Few extern variables */
extern GladeXML *gladexml;
extern GList    *plugins_list;
extern gint      nb_plugins;

/* extern struct Score_t         Score; */
extern struct Staff_t         Staff;
extern struct Object_t        Object;
extern struct Selection_t     Selection;
extern struct Clipboard_t     Clipboard;
extern struct Display_t       Display;
extern enum   ActionType_t    ActionType;


/* What we should display on the score */
typedef enum accidentals_e
{

	A_NONE        = 1 << 0,
	A_SHARP       = 1 << 1,
	A_DOUBLESHARP = 1 << 2,
	A_FLAT        = 1 << 3,
	A_DOUBLEFLAT  = 1 << 4,
	A_NATURAL     = 1 << 5,
	A_PARENTHESIS = 1 << 6,

} accidentals_e;

typedef enum object_e
{

	O_NONE            = 1 << 0,
	O_BEAMED          = 1 << 1,
        O_LAST_BEAMED     = 1 << 2,
	O_TIE_START       = 1 << 3, 
	O_TIE_STOP        = 1 << 4,
	O_AUTOSTEM        = 1 << 5,
	O_STEMUP          = 1 << 6,
	O_STEMDOWN        = 1 << 7,
	O_DURATION        = 1 << 8,
	O_STACCATO        = 1 << 9,
	O_GRACE           = 1 << 10,
	O_MARCATOUP       = 1 << 11,
	O_MARCATODOWN     = 1 << 12,
	O_ACCENT          = 1 << 13,
	O_STACCATISSIMO   = 1 << 14,
	O_TENUTO          = 1 << 15,


} object_e;

typedef struct Display_t
{

        gboolean measure_number;
        gboolean instruments;
        gboolean clefs;
        gboolean score_expressions;
        gboolean barlines;
        gboolean ending_bar;
        gboolean key_signature;
        gboolean time_signature;
        gboolean tempo;

} Display_t;

typedef struct Selection_t
{


	accidentals_e  accidentals;
	object_e       nature;

	gint           object_type;

        
} Selection_t;

typedef struct 
{
        /** Returns the mice x position */
        gdouble        x; 
        /** Returns the mice y position */
        gdouble        y;

        /** Used to draw the selection */
        gdouble           x_origin;
        /** Used to draw the selection */
        gdouble           y_origin;
        /** Used to draw the selection */
        gdouble           staff_selected_begin;
        /** Used to draw the selection */
        gdouble           staff_selected_end;
} Score_selection_t;


typedef struct Object_t
{
        /** Unique ID */
        gulong         id;      

        /** key_signature, note (quarter, rest, ...), tab */
        gint           type;
        /** Stem Up/Down, Staccato, Grace... ? */ 
        object_e       nature;  
	accidentals_e  accidentals;

	gulong         group_id; /* the id where the object is grouped (ie.: a chord) */

        gdouble           x;
        gdouble           y;
        gdouble           x2; /* we need supplementary coordinates for */
        gdouble           y2; /* crescendo etc..     */
        gdouble           x3; /* and this is used for slurs, tied notes */
        gdouble           y3; /* BTW: and YES, I don't like that! */

        gint           pitch;	/* You know, the pitch stands for a note, to know where it is accordingly to the key */

        gint           tab_number;

        /** Is the object selected ? if so, change its color */
        gboolean       is_selected;  

} Object_t;

typedef struct Staff_t
{

        /** How many lines for the staff (usually 5) */
        guint8     nb_lines;    
        /** How far are the lines with each other */
        guint8     space_btw_lines; 

        /** TRUE if the staff is selected */
        gboolean   is_selected; 

        /** The key for the staff */
        gint       key;        
        /** The key signature for the staff */ 
        gint       key_signature; 

        /** The time signature for the staff 
         * time_signature[0] = TIME_SIGNATURE_NORMAL | TIME_SIGNATURE_COMMON_TIME | TIME_SIGNATURE_ALLA_BREVE 
         * time_signature[1] = Number of beats 
         * time_signature[2] = Beat duration */
        guint8     time_signature[2]; 

        /** The number of the measure on which we are working */
        gint       measure_number; 
        /** How many measures in total */
        gint       total_measures; 

        /** Useful positions */
        /** Position X where the staff begins */
        gdouble    extremity_begin_x;
        /** Position Y where the staff begins */ 
        gdouble    extremity_begin_y; 
        /** The current position */
	/** It is where the next object should be drawn */
        gint       current_x;   
	gint       start_x;

        /** The Instrument for the staff (MIDI) */
        gint       midi_instrument; 

        GList    * Object_list;
        Object_t * Object;

} Staff_t;

typedef struct Identity_t
{
        
	/** Score's title */
        GString * title;
	/** Score's subtitle */
        GString * subtitle;
	/** Score's composer */
        GString * composer;
	/** Score's copyright */
        GString * copyright;

	/** Store the name of the file */
        GString * filename;
        
} Identity_t;

typedef struct Score_t
{

	/** The title of the window: <Untitled score> by default */
	gchar       * windows_title;
        /** Drawing area widget */
/*         GtkWidget   * area;        */
	/** Tempo of the score */
        gint          tempo;             
	/** The note refered by the tempo: 0->doublewhole, 1->whole, 2->half, 4->quarter, 8->eighth ... */
        gint          tempo_note; 
	/** Optional Text for the tempo */
        GString     * tempo_text; 
	/** Every object in the score has a *unique* ID */
        gulong        object_id;    
	/** Has the score been modified since we saved ? useful for save/save as and quit */
        gboolean      is_modified;  
	/** How many staves in the score */
        gint          nb_staves; 
	/** How wide is the drawing area (score) widget ? */     
        gdouble       width;     
	/** How high is the drawing area (score) widget ? */
        gdouble       height;       
	/** The number of the staff selected */
        gint          staff_selected; 
	/** Position X where the staff ends */
        gdouble       staff_extremity_end_x;
	/** Where we start to put the notes */
        gdouble       staff_startx;
        struct Identity_t  * Identity; /* Useful informations */
/*         struct Display_t   * Display; */

        /** Staves tracked in a list, to have many staves as your memory allow you to have */
        GList       * Staff_list;
        Staff_t     * Staff;

} Score_t;


/***
 *** Clipboard related things
 ***/

typedef enum ActionType_t         /* Used by the clipboard */
{

        ADD_OBJECT    = 1 << 1,  
        MOVE_OBJECT   = 1 << 2,
	DEL_OBJECT    = 1 << 4,

} ActionType_t;

typedef struct Undo_t
{

        gdouble           object_id;
        ActionType_t      action_type;
        
} Undo_t;

typedef struct Redo_t
{

        gdouble         object_id;
        ActionType_t    action_type;

} Redo_t;

typedef struct Clipboard_t
{

        GList    * Undo_list;
        GList    * Redo_list;
        GList    * Object_list;

        Undo_t   * Undo;
        Redo_t   * Redo;
        Object_t * Object;

} Clipboard_t;

#endif
