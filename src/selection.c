/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * selection.c
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
#include "draw.h"
#include "spacings.h"
#include "staff.h"
#include "score.h"
#include "selection.h"

gint set_selection_origin(GtkWidget *widget, GdkEventButton *event)
{

  Score_selection_t *selection = score_get_selection_from_widget(widget);
  
        gdouble x = event->x;
        gdouble y = event->y;
/*         gint state = event->state; */

        selection->x_origin = x;
        selection->y_origin = y;
	return 0;
}

/* extern  */
/* void set_selection(selected_e selection) */
/* { */
/* 	selection->selected = selection; */
/* } */

/* extern */
/* selected_e get_selection(void) */
/* { */
/* 	return selection->selected; */
/* } */

/* extern  */
/* void set_selection_display(display_e selection) */
/* { */
/* 	selection->display = selection; */
/* } */

/* extern */
/* display_e get_selection_display(void) */
/* { */
/* 	return selection->display; */
/* } */

/* extern  */
/* void set_selection_note(note_e selection) */
/* { */
/* 	selection->note = selection; */
/* } */

/* extern */
/* note_e get_selection_note(void) */
/* { */
/* 	return selection->note; */
/* } */

/* extern  */
/* void set_selection_type(type_e selection) */
/* { */
/* 	selection->type = selection; */
/* } */

/* extern */
/* type_e get_selection_type(void) */
/* { */
/* 	return selection->type; */
/* } */

/* extern  */
/* void set_selection_accidentals(accidentals_e selection) */
/* { */
/* 	selection->accidentals = selection; */
/* } */

/* extern */
/* accidentals_e get_selection_accidentals(void) */
/* { */
/* 	return selection->accidentals; */
/* } */

/* extern  */
/* void set_selection_barline(barline_e selection) */
/* { */
/* 	selection->barline = selection; */
/* } */

/* extern */
/* barline_e get_selection_barline(void) */
/* { */
/* 	return selection->barline; */
/* } */

/* extern  */
/* void set_selection_object(object_e selection) */
/* { */
/* 	selection->object = selection; */
/* } */

/* extern */
/* object_e get_selection_object(void) */
/* { */
/* 	return selection->object; */
/* } */
 

/* extern gint  */
/* set_selection_origin(GtkWidget *widget, GdkEventButton *event) */
/* { */

/*      if ((selection->object == CURSOR) || (selection->note == CURSOR)) */
/*      { */
/*           selection->x_origin = event->x; */
/*           selection->y_origin = event->y; */
/*      } */

/*      if (DEBUG == 1) */
/* 	     g_print("selection->x (origin) = %f\nselection->y (origin) = %f\n", selection->x_origin, selection->y_origin); */

/*      return 0; */
/* } */

extern gint
highlight_selection(Score_t *score, gdouble x_origin, gdouble y_origin, gdouble x, gdouble y)
{

	GList *listrunner;
	Staff_t *staff = NULL;

	gdouble start_x = 0;
	gdouble object_x = 0;

        staff = g_list_nth_data(score->Staff_list, get_staff_selected(score));

        if ( ! staff ) return -1;

	/* Parsing Objects structure */
	listrunner = g_list_first(staff->Object_list);
	while ( listrunner ) {
		Object_t *object;
		object = (Object_t *)listrunner->data;
               
		start_x = staff->extremity_begin_x + Spacings.Clefs.sb + STANDARD_KEY_SIZE + Spacings.Clefs.sa;

		switch (staff->key_signature) {
		case KEY_SIGNATURE_TREBLE_EMPTY:
			break;
		case KEY_SIGNATURE_TREBLE_A_SHARP:
			start_x += 
				4 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_A_FLAT:
			start_x += 
				2 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_B_SHARP:
			start_x += 
				6 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_B_FLAT:
			start_x += 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_C_SHARP:
			start_x += 
				4 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_C_FLAT:
			start_x += 
				5 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_D_SHARP:
			start_x += 
				3 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_D_FLAT:
			start_x += 
				3 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_E_SHARP:
			start_x += 
				5 * Spacings.KeySignatures.sbksa + 
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_E_FLAT:
			start_x += 
				Spacings.KeySignatures.saks;
		
			break;
		case KEY_SIGNATURE_TREBLE_F_FLAT:
			start_x += 
				6 * Spacings.KeySignatures.sbksa +
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_G_SHARP:
			start_x += 
				2 * Spacings.KeySignatures.sbksa +
				Spacings.KeySignatures.saks;

			break;
		case KEY_SIGNATURE_TREBLE_G_FLAT:
			start_x += 
				4 * Spacings.KeySignatures.sbksa +
				Spacings.KeySignatures.saks;

			break;

		}

                
                if (object->group_id == 0 ) {

                        switch (object->type) { /* This is beurk, but this is! */
                                /* RESTS */
                        case DOUBLEWHOLE:
                                object_x += Spacings.NotesRests.sa_doublewhole;
                                break;
                        case WHOLE:
                                object_x += Spacings.NotesRests.sa_whole;
                                break;
                        case HALF:
                                object_x += Spacings.NotesRests.sa_half;
                                break;
                        case QUARTER:
                                object_x += Spacings.NotesRests.sa_quarter;
                                break;
                        case EIGHTH:
                                object_x += Spacings.NotesRests.sa_eighth;
                                break;
                        case SIXTEENTH:
                                object_x += Spacings.NotesRests.sa_sixteenth;
                                break;

                                /* RESTS */
                        case DOUBLEWHOLEREST:
                                object_x += Spacings.NotesRests.sa_doublewholerest;
                                break;
                        case WHOLEREST:
                                object_x += Spacings.NotesRests.sa_wholerest;
                                break;
                        case HALFREST:
                                object_x += Spacings.NotesRests.sa_halfrest;
                                break;
                        case QUARTERREST:
                                object_x += Spacings.NotesRests.sa_quarterrest;
                                break;
                        case EIGHTHREST:
                                object_x += Spacings.NotesRests.sa_eighthrest;
                                break;
                        case SIXTEENTHREST:
                                object_x += Spacings.NotesRests.sa_sixteenthrest;
                                break;

                                /* BARLINES */
                        case BARLINE_SINGLE:
                                object_x += Spacings.Barlines.sab;
                                break;
                        case BARLINE_DOUBLE:
                                object_x += Spacings.Barlines.sab;
                                break;
                        case BARLINE_OPENREPEAT:
                                object_x += Spacings.Barlines.sab;
                                break;
                        case BARLINE_CLOSEREPEAT:
                                object_x += Spacings.Barlines.sab;
                                break;
                        case BARLINE_OPENCLOSEREPEAT:
                                object_x += Spacings.Barlines.sab;
                                break;

                        } /* switch */

                }

		/* Now, we now the x position where the object should be ;) */
		if ( ((object_x + start_x - 30) >= x_origin) && ((object_x + start_x - 30) <= x) )
			object->is_selected = TRUE;


		listrunner = g_list_next(listrunner);
	}
	
	return 0;

}


extern gint
undo_selection(Score_t *score)
{

	GList *listrunner;
	Staff_t *staff;

        staff = g_list_nth_data(score->Staff_list, get_staff_selected(score));

        if (staff) {
	/* Parsing Objects structure */
	listrunner = g_list_first(staff->Object_list);
	while ( listrunner ) {
		Object_t *object;
		object = (Object_t *)listrunner->data;

		object->is_selected = FALSE;

		listrunner = g_list_next(listrunner);
	}

	g_list_free(listrunner);

	return 0;

        }

        return -1;
}

extern gint
undo_selection_widget(GtkWidget *widget, gpointer user_data)
{

	GList *listrunner;
        Score_t *score = score_get_from_widget(widget);
	Staff_t *staff;

        staff = g_list_nth_data(score->Staff_list, get_staff_selected(score));

        if (staff) {
	/* Parsing Objects structure */
	listrunner = g_list_first(staff->Object_list);
	while ( listrunner ) {
		Object_t *object;
		object = (Object_t *)listrunner->data;

		object->is_selected = FALSE;

		listrunner = g_list_next(listrunner);
	}

	g_list_free(listrunner);

	return 0;

        }

        return -1;
}


gint get_selection_object_type(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/*                         return object->type;  */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}

gint get_selection_id(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/*                         return object->id;  */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}

gdouble get_selection_x_selection(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/* /\*                         return object->x_selection;  *\/ */
/*                      printf("Figure out what x_selection is ;)\n"); */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}

gdouble get_selection_y_selection(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/* /\*                         return object->y_selection;  *\/ */
/*                      printf("Figure out what y_selection is ;) \n"); */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}

gboolean get_selection_with_sharp(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/* /\*                         return object->with_sharp;  *\/ */
/*                      printf("What is with_sharp ?\n"); */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}

gboolean get_selection_with_flat(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/*                      printf("What is with_flat ?\n"); */
/* /\*                         return object->with_flat;  *\/ */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}

gboolean get_selection_with_natural(gint x, gint y, gint staff)
{
/*      GList *tmp_list; */
     

/*      for ( tmp_list = score->Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/*      { */
/*          Object_t *object; */
/*          gint i = 0; */
/*          gint j = 0; */
         

/*          object = (Object_t *)tmp_list->data; */

/* /\* 9 x 9 *\/ */
/*          for ( i = -4 ; i < 10 ; i++ ) */
/*          { */
/*               for (j = -4; j < 10; j++){ */
                   
/*               if ( object->x+i == x)  { */
/*                    if (object->y+j == y) */
/*                    { */
/* /\*                         return object->with_natural;  *\/ */
/*                      printf("plouf return object->with_natural, but what is it?!?\n"); */
/*                    } */
/*               } */
              
/*               }     */

/*          } */
         
/*      } */

     return 0;
     
}
