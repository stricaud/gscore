/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * mouse_event.c
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

#include <gtk/gtk.h>

#include "gscore.h"
#include "constants.h"
#include "staff.h"
#include "position.h"
#include "draw.h"
#include "selection.h"
#include "key_cursor.h"
#include "common.h"
#include "score.h"
#include "mouse_event.h"

gint max_x = 0;
gint max_y = 0;
gint min_x = 0;
gint min_y = 0;

gboolean set_min_x = FALSE;
gboolean set_min_y = FALSE;
gboolean set_max_x = FALSE;
gboolean set_max_y = FALSE;

static gint staff_working_with = 0;

gboolean 
mouse_button_release_event(GtkWidget *widget, GdkEventButton *event, gpointer user_data)
{
        Score_selection_t *selection = score_get_selection_from_widget(widget);
        Score_t *score = score_get_from_widget(widget);
        GtkWidget *area = score_get_area_from_widget(widget);
  
        /* 	printf("selection->x_origin = %f, selection->y_origin = %f, selection->x = %f, selection->y = %f\n", */
        /* 	       selection->x_origin, selection->y_origin, selection->x, selection->y); */
        
	if ( Selection.object_type == CURSOR ) {

		highlight_selection(score, selection->x_origin, selection->y_origin, 
				    selection->x, selection->y);

                set_staff_unselect(score, get_staff_selected(score));
                set_staff_selected(score, get_staff_id(score, get_staff_extremity_end_y(score, staff_working_with) - 5));
/*                 set_staff_selected(score, get_staff_id(score, event->y)); */

	}

        selection->x_origin = 0;
        selection->y_origin = 0;

        max_x = 0;
        max_y = 0;
        min_x = 0;
        min_y = 0;

        set_min_x = FALSE;
        set_min_y = FALSE;
        set_max_x = FALSE;
        set_max_y = FALSE;

        refresh(area);

	return FALSE;
}


gboolean 
mouse_button_press_event(GtkWidget *widget, GdkEventButton *event, gpointer user_data)
{
        Score_t *score = score_get_from_widget(widget);
/*         Staff_t *staff = staff_selected_get_from_score(score); */

/*         show_object_tooltip(NULL); */
  
        if ( Selection.object_type == CURSOR ) {

                undo_selection(score);

                staff_working_with = get_staff_id(score, event->y);

                set_selection_origin(widget, (GdkEventButton *)event);
        }

	return FALSE;
}


gint redraw_till_x1_then_from_x2(gdouble x1, gdouble x2, gint staff, gint coeff)
{
  /*
   * x1: the position till redraw
   * x2: the position from where to redraw 
   * staff: the staff redrawed
   * coeff: the coefficient, how long above and below the staff ?
   */

/*   gtk_widget_queue_draw_area(Score.area, 0, */
/*                              Score.Staff[staff].extremity_begin_y - coeff, */
/*                              x1,  */
/*                              Score.Staff[staff].extremity_end_y + coeff); */
/*   gtk_widget_queue_draw_area(Score.area, x2, */
/*                              Score.Staff[staff].extremity_begin_y - coeff, */
/*                              Score.width,  */
/*                              Score.Staff[staff].extremity_end_y + coeff); */
  return 0;
}

gboolean mouse_motion_event(GtkWidget *widget, GdkEventMotion *event, gpointer user_data)
{

        gdouble x;
        gdouble y;

/* 	g_print("mouse_motion_event : state = %d\n", event->state); */
	
	if(! (event->state & GDK_BUTTON1_MASK)) return TRUE;
/* 	g_print("mme : class = %s %s\n", G_OBJECT_CLASS_NAME(widget), G_OBJECT_CLASS_NAME(event)); */
	
	Score_selection_t *selection = score_get_selection_from_widget(widget);
	GtkWidget *area = score_get_area_from_widget(widget);
	
        x = event->x;
        y = event->y;

        selection->x = x;
        selection->y = y;

        if ((selection->x_origin != 0) && (selection->y_origin != 0))
                {
                        draw_line(area, 65535, 0, 0, selection->x_origin, selection->y_origin, selection->x_origin, selection->y);
                        draw_line(area, 65535, 0, 0, selection->x_origin, selection->y_origin, selection->x, selection->y_origin);
                        draw_line(area, 65535, 0, 0, selection->x_origin, selection->y, selection->x, selection->y);
                        draw_line(area, 65535, 0, 0, selection->x, selection->y_origin, selection->x, selection->y);

                        

                        if ((selection->x >= selection->x_origin) && (selection->y >= selection->y_origin))
                                {
                                        /*    |    */
                                        /* ------- */
                                        /*    | X  */

                                        if (selection->x > max_x)
                                                {
                                                        max_x = selection->x;
                                                }
                                        
                                        if (selection->y > max_y)
                                                {
                                                        max_y = selection->y;
                                                }

					/*                                         printf("bottom/right\n"); */

                                        /* Clean inside the square */
                                        gtk_widget_queue_draw_area(area, selection->x_origin + 1, selection->y_origin + 1,
                                                                   selection->x - selection->x_origin - 1,
                                                                   selection->y - selection->y_origin - 1);
                                        
                                        /* Clean around */
                                        /* (Right) */
                                        gtk_widget_queue_draw_area(area, selection->x + 1, selection->y_origin,
                                                                   max_x - selection->x_origin, max_y - selection->y_origin + 1);
                                        /* (Bottom) */
                                        gtk_widget_queue_draw_area(area, selection->x_origin, selection->y + 1,
                                                                   max_x - selection->x_origin, max_y - selection->y + 1);

                                }

                        if ((selection->x < selection->x_origin) && (selection->y < selection->y_origin))
                                {
                                        /*  X |    */
                                        /* ------- */
                                        /*    |    */

					/*                                         printf("top/left\n"); */

                                        if (!set_min_y) {
						min_y = selection->y;
						set_min_y = TRUE;
					}
					
                                        if (selection->y < min_y) {
						min_y = selection->y;
					}
					
                                        if (!set_min_x) {
						min_x = selection->x;
						set_min_x = TRUE;
					}
                                        
                                        if (selection->x < min_x) {
						min_x = selection->x;
					}

                                        /* Clean inside the square */
                                        gtk_widget_queue_draw_area(area, selection->x + 1, selection->y + 1,
                                                                   selection->x_origin - selection->x - 1,
                                                                   selection->y_origin - selection->y - 1);

                                        /* Clean around */
                                        if (min_x < 0) min_x = 0;
                                        if (min_y < 0) min_y = 0;
                                        /* (Left) */
                                        gtk_widget_queue_draw_area(area, min_x, min_y,
                                                                   selection->x - min_x,
                                                                   selection->y_origin + 1);
                                        /* (Top) */
                                        gtk_widget_queue_draw_area(area, min_x, min_y,
                                                                   selection->x_origin - min_x + 1, selection->y - min_y);

                                }

                        if ((selection->x <= selection->x_origin) && (selection->y > selection->y_origin))
                                {
                                        /*    |    */
                                        /* ------- */
                                        /*  X |    */

					/*                                         printf("bottom/left\n"); */


                                        if (!set_min_x)
                                                {
                                                        min_x = selection->x;
                                                        set_min_x = TRUE;
                                                }
                                        
                                        if (selection->x < min_x)
                                                {
                                                        min_x = selection->x;
                                                }

                                        if (selection->y > max_y)
                                                {
                                                        max_y = selection->y;
                                                }


                                        /* Clean inside the square */
                                        gtk_widget_queue_draw_area(area, selection->x + 1, selection->y_origin + 1,
                                                                   selection->x_origin - selection->x - 1,
                                                                   selection->y - selection->y_origin - 1);


                                        /* Clean around */
                                        if (min_x < 0) min_x = 0;
                                        /* (Left) */
                                        gtk_widget_queue_draw_area(area, min_x, selection->y_origin,
                                                                   selection->x - min_x,
                                                                   max_y - selection->y_origin + 1);
                                        /* (Bottom) */
                                        gtk_widget_queue_draw_area(area, selection->x, selection->y + 1,
                                                                   selection->x_origin - min_x,
                                                                   max_y - selection->y);
                                }
                        
                }

	if ((selection->x > selection->x_origin) && (selection->y <= selection->y_origin))
		{
			/*    | X  */
			/* ------- */
			/*    |    */

			/*                                         printf("top/right\n"); */

			if (!set_min_y)
				{
					min_y = selection->y;
					set_min_y = TRUE;
				}

			if (selection->y < min_y)
				{
					min_y = selection->y;
				}

			if (selection->x > max_x)
				{
					max_x = selection->x;
				}


			/* Clean inside the square */
			gtk_widget_queue_draw_area(area, selection->x_origin + 1, selection->y + 1,
						   selection->x - selection->x_origin - 1,
						   selection->y_origin - selection->y - 1);


			/* Clean around */
			if (min_y < 0) min_y = 0;
			/* (Right) */
			gtk_widget_queue_draw_area(area, selection->x + 1, min_y,
						   max_x - selection->x + 1,
						   selection->y_origin - min_y + 1);
			/* (Top) */
			gtk_widget_queue_draw_area(area, selection->x_origin, min_y,
						   max_x - selection->x_origin,
						   selection->y - min_y);
		}

	return FALSE;
}


/*         printf("Mouse moving\n"); */

/*      GdkGC    * gc; */
/*      gdouble x, y; */
/*      gint position; */

/*      gint offset = 0; */

/*      x = event->x; */
/*      y = event->y; */

/*      gc = gdk_gc_new(Score.area->window); */

/* /\*      if (DEBUG == 1) *\/ */
/* /\*        g_print("GDK_EVENT_MOTION\n"); *\/ */

/*      /\*  */
/*       * When the mouse moves upon the staff, you should redraw absolutely everything */
/*       *\/ */

/*      if ((Selection.x_origin >= 0) && (Selection.y_origin >= 0)) */
/*        { */
/*          if (DEBUG == 1) */
/*            g_print("X = %f, Y = %f\n", x, y); */

/*          do_selection(Selection.x_origin, Selection.y_origin, x, y); */
/* /\*          g_print("you are selecting something\n"); *\/ */
/*        } else { */

/*          gtk_widget_queue_draw_area(GTK_WIDGET(Score.area),  */
/*                                     KeyCursor.x_returned - 50, KeyCursor.y_returned - 50 ,  */
/*                                     KeyCursor.x_returned + 50, KeyCursor.y_returned + 50); */
         
/*          switch (Selection.object_type) */
/* 	 { */
/* 	 case DOUBLEWHOLE: */
/* 	 case WHOLE: */
/* 		 offset = 0; */
/* 		 break; */
/* 	 case HALF: */
/* 	 case QUARTER: */
/* 	 case EIGHTH: */
/* 	 case SIXTEENTH: */
/* 		 offset = 21; */
/* 		 break; */
/* 	 default: */
/* 		 offset = 0; */
/* 	 } */


/*          KeyCursor.position = get_position(TREBLE_KEY, get_staff_selected(), y + offset); */

/*        } */
/* /\*          switch (Selection.note) *\/ */
/* /\*            { *\/ */

/* /\*            case DOUBLEWHOLE: *\/ */
/* /\*            case WHOLE: *\/ */
/* /\* /\\*              position = get_position(TREBLE_KEY, get_staff_selected(), y+21); *\\/ *\/ */
          
/* /\* /\\*              switch(position) *\\/ *\/ */
/* /\* /\\*                { *\\/ *\/ */
/* /\* /\\*                case 12: *\\/ *\/ */
/* /\* /\\*                case 13: *\\/ *\/ */
/* /\* /\\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\\/ *\/ */
/* /\* /\\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y); *\\/ *\/ */
/* /\* /\\*                  draw_lines_for_object(get_staff_selected(), TRUE,  *\\/ *\/ */
/* /\* /\\*                                        3, x); *\\/ *\/ */
/* /\* /\\*                  break; *\\/ *\/ */
/* /\* /\\*                case 10: *\\/ *\/ */
/* /\* /\\*                case 11: *\\/ *\/ */
/* /\* /\\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\\/ *\/ */
/* /\* /\\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y); *\\/ *\/ */
/* /\* /\\*                  draw_lines_for_object(get_staff_selected(), TRUE,  *\\/ *\/ */
/* /\* /\\*                                        2, x); *\\/ *\/ */
/* /\* /\\*                  break; *\\/ *\/ */
/* /\* /\\*                case 8: *\\/ *\/ */
/* /\* /\\*                case 9: *\\/ *\/ */
/* /\* /\\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\\/ *\/ */
/* /\* /\\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y); *\\/ *\/ */
/* /\* /\\*                  draw_lines_for_object(get_staff_selected(), TRUE,  *\\/ *\/ */
/* /\* /\\*                                        1, x); *\\/ *\/ */
/* /\* /\\*                  break; *\\/ *\/ */
/* /\* /\\*                case 7: *\\/ *\/ */
/* /\* /\\*                case 6: *\\/ *\/ */
/* /\* /\\*           case 5: *\\/ *\/ */
/* /\* /\\*                case 4: *\\/ *\/ */
/* /\* /\\*                case 3: *\\/ *\/ */
/* /\* /\\*                case 2: *\\/ *\/ */
/* /\* /\\*                case 1: *\\/ *\/ */
/* /\* /\\*                case 0: *\\/ *\/ */
/* /\* /\\*                case -1: *\\/ *\/ */
/* /\* /\\*                case -2: *\\/ *\/ */
/* /\* /\\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y + 21); *\\/ *\/ */
/* /\* /\\*                  break; *\\/ *\/ */
/* /\* /\\*                case -4: *\\/ *\/ */
/* /\* /\\*                case -5: *\\/ *\/ */
/* /\* /\\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\\/ *\/ */
/* /\* /\\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, y + 50 , x + 20, y + 8); *\\/ *\/ */
/* /\* /\\*                  draw_lines_for_object(get_staff_selected(), FALSE,  *\\/ *\/ */
/* /\* /\\*                                        1, x); *\\/ *\/ */
/* /\* /\\*                case -6: *\\/ *\/ */
/* /\* /\\*                case -7: *\\/ *\/ */
/* /\* /\\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\\/ *\/ */
/* /\* /\\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, y + 50 , x + 20, y + 8); *\\/ *\/ */
/* /\* /\\*                  draw_lines_for_object(get_staff_selected(), FALSE,  *\\/ *\/ */
/* /\* /\\*                                        2, x); *\\/ *\/ */
/* /\* /\\*                  break; *\\/ *\/ */
/* /\* /\\*                } *\\/ *\/ */
         
/* /\*              break; *\/ */
/* /\*            case HALF: *\/ */
/* /\*            case QUARTER: *\/ */
/* /\*            case EIGHTH: *\/ */
/* /\*            case SIXTEENTH: *\/ */
/* /\*              position = get_position(TREBLE_KEY, get_staff_selected(), y+21); *\/ */
          
/* /\*              switch(position) *\/ */
/* /\*                { *\/ */
/* /\*                case 12: *\/ */
/* /\*                case 13: *\/ */
/* /\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\/ */
/* /\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y + 21); *\/ */
/* /\*                  draw_lines_for_object(get_staff_selected(), TRUE,  *\/ */
/* /\*                                        3, x); *\/ */
/* /\*                  break; *\/ */
/* /\*                case 10: *\/ */
/* /\*                case 11: *\/ */
/* /\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\/ */
/* /\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y + 21); *\/ */
/* /\*                  draw_lines_for_object(get_staff_selected(), TRUE,  *\/ */
/* /\*                                        2, x); *\/ */
/* /\*                  break; *\/ */
/* /\*                case 8: *\/ */
/* /\*                case 9: *\/ */
/* /\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\/ */
/* /\*                  /\\*             gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x-10, y-10, x+20, y+20); *\\/ *\/ */
/* /\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y + 21); *\/ */
/* /\*             draw_lines_for_object(get_staff_selected(), TRUE,  *\/ */
/* /\*                                   1, x); *\/ */
/* /\*             break; *\/ */
/* /\*                case 7: *\/ */
/* /\*                case 6: *\/ */
/* /\*           case 5: *\/ */
/* /\*                case 4: *\/ */
/* /\*                case 3: *\/ */
/* /\*                case 2: *\/ */
/* /\*                case 1: *\/ */
/* /\*                case 0: *\/ */
/* /\*                case -1: *\/ */
/* /\*                case -2: *\/ */
/* /\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, 0 , x + 20, y + 21); *\/ */
/* /\*                  break; *\/ */
                 
/* /\* /\\*           case 6: *\\/ *\/ */
/* /\* /\\*           case 7: *\\/ *\/ */
/* /\* /\\*             redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\\/ *\/ */
/* /\* /\\*             break; *\\/ *\/ */
/* /\* /\\*           case 5: *\\/ *\/ */
/* /\* /\\*                  draw_placement(x+2, y+25); *\\/ *\/ */
/* /\*                  break; *\/ */
/* /\*                case -4: *\/ */
/* /\*                case -5: *\/ */
/* /\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\/ */
/* /\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, y + 50 , x + 20, y + 8); *\/ */
/* /\*                  draw_lines_for_object(get_staff_selected(), FALSE,  *\/ */
/* /\*                                        1, x); *\/ */
/* /\*                case -6: *\/ */
/* /\*                case -7: *\/ */
/* /\*                  redraw_till_x1_then_from_x2(x-4, x+12, get_staff_selected(), 50); *\/ */
/* /\*                  gtk_widget_queue_draw_area(GTK_WIDGET(Score.area), x - 20, y + 50 , x + 20, y + 8); *\/ */
/* /\*                  draw_lines_for_object(get_staff_selected(), FALSE,  *\/ */
/* /\*                                        2, x); *\/ */
/* /\*                  break; *\/ */
/* /\*                } *\/ */
/* /\*              break; *\/ */
/* /\*            } *\/ */
/* /\*        } *\/ */

/*      return FALSE; */
/* } */

