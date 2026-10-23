/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * position.c
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
#include "constants.h"
#include "common.h"
#include "debug.h"
#include "spacings.h"
#include "position.h"
#include "score.h"

gint get_pitch_from_y(gint staff, gint y)
{

/*      switch(Score.Staff[staff].key) */
/*      { */
/*      case TREBLE_KEY: */

/*           break; */
/*      } */

	return 0;

}

gint get_position(gint key, gint staff, gint y)
{
     /* The purpose of this function is to get the y position of any element
      * according to its key signature.
      * You may use it to redraw your objects after
      */

/*      gint value = 0; */

/*      gint position_zero = 0; */

/*      if ( key == TREBLE_KEY ) { */
/*           gint i;  */
/*           gint j = 12; /\* the returned value *\/ */

/*           position_zero = Score.Staff[staff].extremity_begin_y + 28; */

/*     /\* the following returns the position of an object *\/ */

/*           for ( i = -55; j >= -10 ; j-- ) */
/*           { */
               
/*                if (( y >= position_zero+i ) && ( y < position_zero+i+4 )) */
/*                     value = j; */

/*                i = i + 4; */
                    
/*           } */
/*      } */

/*      if ( key == BASS_KEY ) { */


/*      } */

/*      if ( key == ALTO_KEY ) { */


/*      } */

/*      if ( key == TAB_KEY ) { */


/*      } */

/*      return value; */

	return 0;

}

/* extern */
/* gdouble get_y_from_position_no_key(guint16 space_btwn_lines, gint zero, gdouble staff_begin_y, gdouble height, gint position) */
/* { */

/*      gint y_for_zero = staff_begin_y + (( zero - 1 ) * ( 1 + space_btwn_lines )); */

/*      gdouble value = 0; */

/*      gint i; */


/* /\*      g_printf(g_strdup_printf("position.c,\n get_y_from_position_no_key,\n \ *\/ */
/* /\*                                 Staff_begin_y %f, y_for_zero %f, space_btwn_lines %d, zero %d, Height, %f, Position: %d, AZ: %d\n", *\/ */
/* /\*                                 staff_begin_y, y_for_zero, space_btwn_lines, zero, height, position, around_zero(position))); *\/ */


/*      if (around_zero(position)) */
/*      { */
/*           switch (position) */
/*           { */
/*           case -2: */
/*                value += ( height / 2 ) + 1; */
/*                break; */
/*           case -1: */
/*                value += 1; */
/*                break; */
/*           case 0: */
/*                value -= ( height / 2 ); */
/*                break; */
/*           case 1: */
/*                value -= height; */
/*                break; */

/*           } */
/*      } else */
/*      { */

/*           if ( position >= 0 ) */
/*           { */
/*                for ( i = 0; i <= position; i++) */
/*                { */
/*                     if (is_even(i)) */
/*                          value -= ( height / 2 ) - 1; */
/*                     else */
/*                          value -= ( height / 2 ); */
/*                } */
/*           } */
          
/*           if ( position < 0 ) */
/*           { */
/*                if (is_even(position)) */
/*                     value += ( height / 2 ); */
/*                else */
/*                     value += ( height / 2) + 2; */
/*           } */
/*      } */


/*      return y_for_zero + value; */

/* } */


extern
gdouble get_y_from_position_no_key(guint16 space_btwn_lines, gint zero, gdouble staff_begin_y, gdouble height, gint position)
{
/***
 *** space_btw_lines = space between lines for the staff
 *** zero = the reference line (the position 0) ex.: for treble key = 4
 *** staff_begin_y = the y position of the staff (the first line)
 *** height = the height of the object (in general = 8)
 *** position = the position of the note
 ***/
     gdouble value = 0;

     gdouble starty_position = 0;

     gint i = 0;

     /***
      *** if it is an even number, the object _is_ on a line and its height should be divided by two
      *** to return the correct y position
      ***/

     /***
      *** According to the key, the ref. note can change (the ref. note is a _G_ for the _G key_ :o)
      ***/
     starty_position = staff_begin_y + (zero - 1) + ((zero - 1) * space_btwn_lines);

     debug_msg(g_strdup_printf("starty_position = %f\n", starty_position));

     value = starty_position;


     if ( position >= 0 ) {
          while ( i <= position ) {
               if ( i == 0 ) {
                    value -= (height / 2) - 1;
                    i++;
                    continue;
               }

               if ( is_even(i) ) {
                    value -= height / 2;
                    i++;
               } else {
                    value -= (height / 2) + 1;
                    i++;
               }


          } /* while ( i <= position ) */
     } else {
          /* When the position is negative... */

          i = -1;
          while ( i >= position ) {
               if ( i == -1 ) {
                    value += 1;
                    i--;
                    continue;
               }
               
/*                if ( i == -2 ) { */
/*                     value += (height / 2) + 1; */
/*                } */

               if ( ! is_even(i) ) {
                    value += (height / 2);
                    i--;
               } else {
                    value += (height / 2) + 1;
                    i--;
               }

          }
     }

     if (DEBUG == 1)
          g_print("value = %f\n", value);

     return value;
}

extern
gdouble get_y_from_position(gint key, gdouble staff_begin_y, gint position)
{
     gdouble value = 0;

     switch (key) {
     case TREBLE_KEY:
	     value = get_y_from_position_no_key(8, 4, staff_begin_y, 8, position);
	     break;
     default:
/* 	     printf("Gscore Warning: No Key!\n"); */
	     value = get_y_from_position_no_key(8, 4, staff_begin_y, 8, position);
     }

/*      if ( key == TREBLE_KEY ) */
/*           value = get_y_from_position_no_key(8, 4, staff_begin_y, 8, position); */


     return value;
}

extern gdouble
get_spacing_for_object(gint type)
{
/* /\*      gdouble spacing; *\/ */

/*      switch (type) */
/*      { */
/*      case DOUBLEWHOLE: */
/* 	     return Spacings.NotesRests.sa_doublewhole; */
/* 	     break; */
/*      case DOUBLEWHOLEREST: */
/* 	     return Spacings.NotesRests.sa_doublewholerest; */
/* 	     break; */
/*      case WHOLE: */
/* 	     return Spacings.NotesRests.sa_whole; */
/* 	     break; */
/*      case WHOLEREST: */
/* 	     return Spacings.NotesRests.sa_wholerest; */
/* 	     break; */
/*      case HALF: */
/* 	     return Spacings.NotesRests.sa_half; */
/* 	     break; */
/*      case HALFREST: */
/* 	     return Spacings.NotesRests.sa_halfrest; */
/* 	     break; */
/*      case QUARTER: */
/* 	     return Spacings.NotesRests.sa_quarter; */
/* 	     break; */
/*      case QUARTERREST: */
/* 	     return Spacings.NotesRests.sa_quarterrest; */
/* 	     break; */
/*      case BARLINE_SINGLE: */
/*      case BARLINE_DOUBLE: */
/*      case BARLINE_OPENREPEAT: */
/*      case BARLINE_CLOSEREPEAT: */
/*      case BARLINE_OPENCLOSEREPEAT: */
/* 	     return Spacings.Barlines.sab; */
/* 	     break; */
/*      case EIGHTH: */
/* 	     return Spacings.NotesRests.sa_eighth; */
/* 	     break; */
/*      case EIGHTHREST: */
/* 	     return Spacings.NotesRests.sa_eighthrest; */
/* 	     break; */
/*      case SIXTEENTH:  */
/* 	     return Spacings.NotesRests.sa_sixteenth; */
/* 	     break; */
/*      case SIXTEENTHREST: */
/* 	     return Spacings.NotesRests.sa_sixteenthrest; */
/* 	     break; */
/*      default: */
/* 	     g_warning("No Spacing returned (default): see position.c"); */
/* 	     return 0; */
/* /\* 	     return Spacings.NotesRests.sa_quarter; *\/ */
/*      } */

	return 0;

}

gdouble set_x_pos(gint staff, gint x, gint type)
{
/*      gdouble value = 0; */

/*      GList *listrunner_object; */
/*      GList *listrunner_note; */

/*      gint barline_x = 0; */
/*      gint note_x = 0; */

/*      gboolean Insertion = FALSE;        /\* TRUE if there are elements after the current x position *\/ */
/*                                         /\* This means that you are inserting an element *\/ */
/*      gboolean Chord = FALSE; */
/*      gdouble spacing; */


/*      Insertion = FALSE; */

/*      if (Insertion == TRUE) */
/*      { */
/*           g_message("Sorry, the Insertion of an object is not written yet!\n"); */
/*           value = 0; */
/*      } */

/*      if (Insertion == FALSE) */
/*      { */
/*           value = Score.Staff[staff].current_x; */
/*           spacing = get_spacing_for_object(type); */
 
/*           Score.Staff[staff].current_x += spacing; */
/*      } */

/*      return value; */

	return 0;

}

gdouble get_x_pos(gint staff, gint x)
{
/*      GList *listrunner; */
/*      gdouble value = 0; */


/*      listrunner = g_list_first(Score.Staff[staff].Object_list); */

/*      while (listrunner) */
/*      { */
/*           Object_t *object; */

/*           object = (Object_t *)listrunner->data; */

/*           if (object->x < x) */
/*           { */
/* /\*                value = object->x + object->space_after; *\/ */
/*                value = object->x + get_spacing_for_object(object->type); */
/*                g_print("value in the list: %f\n", value); */
/*           } */

/*           listrunner = g_list_next(listrunner); */
/*      } */

/*      return value; */

	return 0;

}

/* Follow the adjustment according to the cursor */
extern gboolean
position_set_adjustment(GtkWidget *widget, KeyCursor_t *cursor)
{
        GtkAdjustment *adj = NULL;
        GtkScrolledWindow *sw = NULL;

        sw = score_get_scrolled_window_from_widget(widget);
        adj = gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(sw));

        if ( ! GTK_IS_ADJUSTMENT(adj) )
                return FALSE;

        gtk_adjustment_set_value(adj, cursor->x_returned - 300);

        gtk_scrolled_window_set_hadjustment(GTK_SCROLLED_WINDOW(sw), adj);

        return TRUE;

}

extern gboolean
position_set_adjustment_x(GtkWidget *widget, gdouble x)
{
        GtkAdjustment *adj = NULL;
        GtkScrolledWindow *sw = NULL;

        sw = score_get_scrolled_window_from_widget(widget);
        adj = gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(sw));

        if ( ! GTK_IS_ADJUSTMENT(adj) )
                return FALSE;

        gtk_adjustment_set_value(adj, x);

        gtk_scrolled_window_set_hadjustment(GTK_SCROLLED_WINDOW(sw), adj);

        return TRUE;

}

