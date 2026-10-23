/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * key_event.c
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
#include <gdk/gdkkeysyms.h>
#include <stdio.h>

#include "gscore.h"
#include "constants.h"
#include "debug.h"
#include "spacings.h"
#include "key_cursor.h"
#include "key_event.h"
#include "selection.h"
#include "common.h"
#include "staff.h"
#include "objects.h"
#include "score.h"
#include "notes.h"
#include "rests.h"
#include "position.h"

static gboolean debug_mode = FALSE;

/* static Score_t *ur_score = NULL; */

extern gboolean
score_key_press_event(GtkWidget *widget, GdkEventKey *event)
{
        Score_t *score = NULL;
        Staff_t *staffobj = NULL;
        Object_t *tmpobj = NULL;
        GtkWidget *area;
        KeyCursor_t *cursor = NULL;

        score = score_get_from_widget(widget);
        area = score_get_area_from_widget(widget);
        cursor = score_get_cursor_from_widget(widget);

        if ( ! cursor ) {
                printf("No Cursor!\n");
                return FALSE;
        }
        
        switch (event->keyval) {
                        
        case GDK_space: /* We add the object selected */
                switch ( Selection.object_type ) {
                case CURSOR:
                        gw_message_error("No object selected");
                        break;  
                case DOUBLEWHOLE:
                case WHOLE:
                case HALF:
                case QUARTER:
                case EIGHTH:
                case SIXTEENTH:
                case DOUBLEWHOLEREST:
                case WHOLEREST:
                case HALFREST:
                case QUARTERREST:
                case EIGHTHREST:
                case SIXTEENTHREST:
                case BARLINE_SINGLE:
                case BARLINE_DOUBLE:
                case BARLINE_OPENREPEAT:
                case BARLINE_CLOSEREPEAT:
                case BARLINE_OPENCLOSEREPEAT:

                        /* The scrollbar should move accordingly */
                        position_set_adjustment(widget, cursor);

                        /* If Ctrl is pressed, then we make a chord */
                        if (event->state & GDK_CONTROL_MASK) { /* Make the chord */
                                staffobj = (Staff_t *)
                                        g_list_nth_data(score->Staff_list, 
                                                        get_staff_selected(score));

                                if ( ! staffobj ) {
                                        g_warning("There is no staff object!");
                                        return FALSE;
                                }

                                tmpobj = (Object_t *)
                                        object_get_left(staffobj, cursor->x_returned + 20);

                                if ( ! tmpobj ) {
                                        g_warning("There is no object!");
                                        return FALSE;
                                }

                                if (tmpobj->pitch == cursor->position) {
                                        gw_message_error("You cannot make a chord with the same pitch.");
                                        return FALSE;
                                }

                                if (is_note(tmpobj->type)) {
                                        add_object(score, get_staff_selected(score), 
                                                   Selection.object_type, 
                                                   Selection.accidentals, Selection.nature, tmpobj->id, 
                                                   0, 0, 0, 0, 0, 0, cursor->position, 0, FALSE);
                                }
                        } else { /* If we don't make a chord */
                                tmpobj = (Object_t *) 
                                        object_get_right((Staff_t *)
                                                         g_list_nth_data(score->Staff_list, 
                                                                         get_staff_selected(score)),
                                                         cursor->x_returned);


                                if ( ! tmpobj )
                                        add_object(score, get_staff_selected(score), 
                                                   Selection.object_type, 
                                                   Selection.accidentals, Selection.nature, 0, 
                                                   0, 0, 0, 0, 0, 0, cursor->position, 0, FALSE);
                                else
                                        object_insert(score, tmpobj, get_staff_selected(score),
                                                      Selection.object_type,
                                                      Selection.accidentals, Selection.nature, 0,
                                                      cursor->position, 0, FALSE);
                        }

                        gtk_widget_set_size_request(GTK_WIDGET(score_get_area_from_widget(widget)), 
                                                    score->width, score->height);

                        break;
                case DYNAMIC_PPPP:
                case DYNAMIC_PPP:
                case DYNAMIC_PP:
                case DYNAMIC_P:
                case DYNAMIC_MP:
                case DYNAMIC_MF:
                case DYNAMIC_F:
                case DYNAMIC_FF:
                case DYNAMIC_FFF:
                        add_object(score, get_staff_selected(score), 
                                   Selection.object_type, 0, 0, 0,
                                   staff_get_current_x(score, get_staff_selected(score)), 
                                   0, 0, 0, 0, 0, cursor->position, 0, FALSE);
                        break;
                }

                refresh(area);
                break;
        case GDK_Return:
                score_set_to_widget(NULL, widget);
                break;
/*         case GDK_w: */
/*                 printf("Save the score\n"); */
/*                 ur_score = (Score_t *)score_get_from_widget(widget); */
/*                 score_set_to_widget(NULL, widget); */
/*                 refresh(area); */
/*                 break; */
/*         case GDK_x: */
/*                 printf("Get the saved score\n"); */
/*                 score_set_to_widget(NULL, widget); */
/*                 score_set_to_widget((Score_t*)ur_score, widget); */
/*                 refresh(area); */
/*                 break; */
        case GDK_Delete:
                remove_object_selected(score);
                refresh(area);
                break;
        case GDK_Up:
                object_selected_pitch_up(score);
                cursor->position++;
                refresh(area);
                break;
        case GDK_Down:
                object_selected_pitch_down(score);
                cursor->position--;
                refresh(area);
                break;
        case GDK_Left:
                tmpobj = (Object_t *)
                        object_get_left((Staff_t *)
                                        g_list_nth_data(score->Staff_list, 
                                                        get_staff_selected(score)), 
                                        cursor->x_returned);

                if ( ! tmpobj ) { 
                        printf("There's no note on the left!\n");
                        return FALSE;
                }

/*                 printf("OBJECT ID = %lu\n", tmpobj->id); */

                staffobj = (Staff_t *)
                        g_list_nth_data(score->Staff_list, 
                                        get_staff_selected(score));

                if ( ! staffobj ) return FALSE;
                
                if (   is_note(tmpobj->type) ||
                       is_rest(tmpobj->type) ||
                       is_barline(tmpobj->type)) {

                        staffobj->current_x = object_get_x(staffobj, tmpobj) + 15;

                        refresh(area);
                }

                break;
        case GDK_Right:
                tmpobj = (Object_t *) 
                        object_get_right((Staff_t *)
                                         g_list_nth_data(score->Staff_list, 
                                                         get_staff_selected(score)), 
                                         cursor->x_returned);

                if ( ! tmpobj ) { 
                        printf("There's no note on the right!\n");
                        return FALSE;
                }

                staffobj = (Staff_t *)g_list_nth_data(score->Staff_list, 
						      get_staff_selected(score));
                if ( ! staffobj ) return FALSE;
                
		/* Must find an accurate way */
/*                 staffobj->current_x += Spacings.NotesRests.sa_quarter / 2; */
                staffobj->current_x = object_get_x(staffobj, tmpobj) + 15;

                refresh(area);

/*                 printf("right id = %lu\n", tmpobj->id); */

                break;
        case GDK_Page_Down:
                switch ( Selection.object_type ) {
                case DOUBLEWHOLE:
                case WHOLE:
                case HALF:
                case QUARTER:
                case EIGHTH:
                case SIXTEENTH:
                        if ( Selection.object_type != SIXTEENTH )
                                Selection.object_type++;

                }
                break;
        case GDK_Page_Up:
                switch ( Selection.object_type ) {
                case DOUBLEWHOLE:
                case WHOLE:
                case HALF:
                case QUARTER:
                case EIGHTH:
                case SIXTEENTH:
                        if ( Selection.object_type != DOUBLEWHOLE )
                                Selection.object_type--;
                }
                break;
        case GDK_Tab:           /* The TAB Key simply adds the single barline */
                add_object(score, get_staff_selected(score), 
                           BARLINE_SINGLE, 0, 0, 0, 0, 0, 0, 0, 0, 0, cursor->position, 0, FALSE);
                
                gtk_widget_set_size_request(GTK_WIDGET(area), score->width, score->height);

                refresh(area);
                
                break;
        case GDK_d:
                if ( debug_mode == TRUE ) {
                        debug_mode = FALSE;
                        printf("** Debug mode desactivated, press 'd' to turn it on\n");
                } else {
                        debug_mode = TRUE;
                        printf("** Debug mode activated, press 'd' to turn it off and 'p' to print debug messages\n");
                }
                break;
        case GDK_p:
                if ( debug_mode == TRUE ) {
                        printf("** Staves properties:\n");
                        print_staves_list(score);
                        printf("** Objects on staff selected:\n");
                        print_objects_staff(score, get_staff_selected(score));
                        printf("\n");
                } else {
                }
                break;
        case GDK_l:
                tmpobj = (Object_t *)
                        object_get_left((Staff_t *)
                                        g_list_nth_data(score->Staff_list, 
                                                        get_staff_selected(score)), 
                                        cursor->x_returned);

                if ( ! tmpobj ) { 
                        printf("There's no note on the left!\n");
                        return FALSE;
                }

                print_object(tmpobj);
                break;
        case GDK_r:
                tmpobj = (Object_t *)
                        object_get_right((Staff_t *)
                                         g_list_nth_data(score->Staff_list, 
                                                         get_staff_selected(score)), 
                                         cursor->x_returned);
                
                if ( ! tmpobj ) { 
                        printf("There's no note on the right!\n");
                        return FALSE;
                }

                print_object(tmpobj);
                break;

                
        }


        return FALSE;
}

extern gboolean
key_event(GtkWidget *widget, GdkEventKey *event)
{

/*         printf("KEY:%d\n", event->keyval); */

        
/* /\*  switch (event->type) *\/ */
/* /\*  { *\/ */

/* /\*  case GDK_KEY_PRESS: *\/ */
                
/*              debug_msg(g_strdup_printf("keyval = %x\n", event->keyval)); */
/*              debug_msg(g_strdup_printf("keyval name = %s\n",gdk_keyval_name(event->keyval))); */
        
/*      switch (event->keyval) */
/*      { */
                
/*      case GDK_space: */
/* /\*          if (Selection.note = QUARTER) *\/ */
/* /\*          { *\/ */
/* /\* */
/*              g_print("Selection.note = %s\n", Selection.note); */
/*  *\/ */

/*              note_append(Selection.object_type); */

/* /\*                  Score.object_id++; *\/ */
/* /\*                  add_note(get_staff_selected(), Selection.note, *\/ */
/* /\*                           cursor->x_returned, cursor->y_returned, 0); *\/ */
                        
/* /\*                  update_staffs_size(40); *\/ */
                        
/* /\*                  score_new_size(); *\/ */
/* /\*          } *\/ */
/* /\*          if (action_type & APPEND_NOTE); *\/ */
/* /\*          { *\/ */


/* /\*                  enum selected_e selection; *\/ */
/* /\*                  enum note_e     note; *\/ */
                        
/* /\*                  g_message("caca\n"); *\/ */
                        
/* /\*                  Score.object_id++; *\/ */
                        
/* /\*                  selection = get_selection(); *\/ */
/* /\*                  note      = get_selection_note(); *\/ */
                        
/* /\*                  if (note & N_QUARTER) *\/ */
/* /\*                  { *\/ */
/* /\*                          add_note(get_staff_selected(), QUARTER, *\/ */
/* /\*                                   cursor->x_returned, cursor->y_returned, 0); *\/ */
                                
/* /\*                          update_staffs_size(40); *\/ */
                                
/* /\*                          score_new_size(); *\/ */
/* /\*                  } *\/ */
                        
/* /\*          } *\/ */

/* /\*  } *\/ */

/* /\*          if (Selection.object != CURSOR) *\/ */
/* /\*          { *\/ */
/* /\*                  Score.object_id++; *\/ */
                        
/* /\*                  add_note(get_staff_selected(), Selection.note,  *\/ */
/* /\*                           cursor->x_returned, cursor->y_returned, 0); *\/ */
                        
/* /\*                  update_staffs_size(40); *\/ */
                        
/* /\*                  score_new_size(); *\/ */
/* /\*          } *\/ */

/* /\*                g_print("Selection note: %d\n", Selection.note); *\/ */
/*              break; */
/*      case GDK_Return: */
/*              /\* Insert the rest matching the note selected *\/ */


/* /\*          g_message("Sexy Return\n"); *\/ */
/* /\*          if (action_type & ADD_BARLINE) *\/ */
/* /\*          { *\/ */
/* /\*                  printf("Insert Bar line\n"); *\/ */
/* /\*          } *\/ */
                
/* /\*          switch(Selection.note) *\/ */
/* /\*          { *\/ */
/* /\*          case DOUBLEWHOLE: *\/ */
/* /\* /\\*                     Selection.note = DOUBLEWHOLEREST; *\\/ *\/ */

/* /\*                  break; *\/ */
/* /\*          case WHOLE: *\/ */
/* /\*                  Selection.note = WHOLEREST; *\/ */
/* /\*                  break; *\/ */
/* /\*          case HALF: *\/ */
/* /\*                  Selection.note = HALFREST; *\/ */
/* /\*                  break; *\/ */
/* /\*          case QUARTER: *\/ */
/* /\* /\\*                     Selection.note = QUARTERREST; *\\/ *\/ */
/* /\*                  action_type = ADD_BARLINE; *\/ */
/* /\*                  break; *\/ */
/* /\*          case EIGHTH: *\/ */
/* /\*                  Selection.note = EIGHTHREST; *\/ */
/* /\*                  break; *\/ */
/* /\*          case SIXTEENTH: *\/ */
/* /\*                  Selection.note = SIXTEENTHREST; *\/ */
/* /\*                  break; *\/ */
/* /\*          } *\/ */
                
/* /\*          if (Selection.object != CURSOR) *\/ */
/* /\*          { *\/ */
/* /\*                  Score.object_id++; *\/ */
                        
/* /\*                  add_note(get_staff_selected(), Selection.note,  *\/ */
/* /\*                           cursor->x_returned, cursor->y_returned, 0); *\/ */
                        
/* /\*                  update_staffs_size(40); *\/ */
                        
/* /\*                  score_new_size(); *\/ */
/* /\*          } *\/ */
                
/* /\*          switch(Selection.note) *\/ */
/* /\*          { *\/ */
/* /\*          case DOUBLEWHOLEREST: *\/ */
/* /\*                  Selection.note = DOUBLEWHOLE; *\/ */
/* /\*                  break; *\/ */
/* /\*          case WHOLEREST: *\/ */
/* /\*                  Selection.note = WHOLE; *\/ */
/* /\*                  break; *\/ */
/* /\*          case HALFREST: *\/ */
/* /\*                  Selection.note = HALF; *\/ */
/* /\*                  break; *\/ */
/* /\*          case QUARTERREST: *\/ */
/* /\*                  Selection.note = QUARTER; *\/ */
/* /\*                  break; *\/ */
/* /\*          case EIGHTHREST: *\/ */
/* /\*                  Selection.note = EIGHTH; *\/ */
/* /\*                  break; *\/ */
/* /\*          case SIXTEENTHREST: *\/ */
/* /\*                  Selection.note = SIXTEENTH; *\/ */
/* /\*                  break; *\/ */
/* /\*          } *\/ */
                
/*              break; */
/*      case GDK_Up: */
/*              cursor->position++; */
/*              refresh(); */
/*              break; */
/*      case GDK_Down: */
/*              cursor->position--; */
/*              refresh(); */
/*              break; */
/*      case GDK_Page_Down: */
                
/*              break; */
/*      case GDK_Page_Up: */
                
/*              break; */
/* /\*  } *\/ */
        
/* /\*           switch(event->keyval) *\/ */
/* /\*           { *\/ */
/* /\*           case 0177525:  *\/ */
/* /\*                g_print("cocorico\n"); *\/ */
/* /\*                break; *\/ */
/* /\*           case 0177526: *\/ */
/* /\*                g_print("ciciroci\n"); *\/ */
/* /\*                break; *\/ */
/* /\*           } *\/ */

/* /\*  break; *\/ */
                
/*      default: */
/*              return FALSE; */
/*      } */
        
        return FALSE;

}
