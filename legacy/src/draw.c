/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * draw.c
 * gscore - a musical score editor
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

#ifdef _WIN32
#include <windows.h>
#endif

#include "gscore.h"
#include "constants.h"
#include "common.h"
#include "macros.h"
#include "debug.h"
#include "key_cursor.h"
#include "score.h"
#include "draw.h"

gint 
draw_cursor(GtkWidget *area, const gchar *xbm, const gchar *mask_xbm, gint width, gint height)
{
	/* Workaround for the bug in gtk for windows */
#ifdef WIN32
        /* SetCursor ? */
#else
        GdkCursor *cursor;

        GdkBitmap *pix_cursor, *mask_cursor;
        GdkColor bg={0,0,0,0};
        GdkColor fg={0,255,255,255};
     
     
        pix_cursor  = gdk_bitmap_create_from_data(NULL, xbm, width, height);
        mask_cursor = gdk_bitmap_create_from_data(NULL, mask_xbm, width, height);
     
        cursor = gdk_cursor_new_from_pixmap(GDK_PIXMAP(pix_cursor), GDK_PIXMAP(mask_cursor), &fg, &bg, 0, 0);
     
        gdk_window_set_cursor(area->window, cursor);

        g_free(pix_cursor);
        g_free(mask_cursor);

#endif
     
        return 0;
  
}

gint x_space(GList *list, gint index)
{
	/* /\* */
	/*  * Gives back the x position where to place an object. */
	/*  * It looks to the value gived by the next object (space_before) */
	/*  *\/ */
	/*      GList    * tmp_list = NULL; */

	/*      gint length = 0; */

	/*      Object_t *object; */

	/*      g_message("x_space"); */

	/*      length = g_list_length(list); */

	/*      tmp_list = g_list_nth(list,index); */
     
	/*      if (index < length) { */
	/*           object = tmp_list->data; */
	/* /\*           return object->x - object->space_before; *\/ */
	/* 	  return object->x - get_spacing_for_object(object->type); */
	/*      } else { */
	/*      /\* else error message here because there's no object after *\/ */
	/*           g_message("Error, you cannot put anything after a note\n"); */
	/*           return -1; */
	/*      } */
     
	return 0;

}

gint draw_note(GtkWidget *area, gchar *file, gboolean selected, gint x, gint y, gint line_x1, gint line_y1,
               gint line_x2, gint line_y2)
{
	/***
	 *** Originaly, there were a piece of DindinX code here,
	 *** but gtk 2.2 became cooler dealing with pixmaps and
	 *** his piece of code has been droped :P
	 ***/

        GtkWidget *scrolled_window;
        GtkAdjustment *adj;
	GladeXML *xml = glade_get_widget_tree(area);

	GdkPixbuf *pixbuf;

        scrolled_window = glade_xml_get_widget(xml, "sw_score_sw");
        adj = gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(scrolled_window));

        if (x < adj->value - 50) return -1; /* -50 is a dummy stuff */
        if (x > adj->value + 2000) return -1; /* +2000 too */

	pixbuf = gdk_pixbuf_new_from_file(get_file_from_data_dir(g_strdup(file)), NULL);


	if (selected == TRUE)
		gdk_pixbuf_fill(pixbuf, 0xff000fff);

	if (pixbuf)
		{
			GdkGC *gc;
			GdkBitmap *mask = NULL;

			gc = gdk_gc_new(area->window);

			gdk_draw_line(area->window, gc,
				      line_x1,line_y1, line_x2,line_y2);
          
			gdk_gc_set_clip_mask(gc, mask);
			gdk_gc_set_clip_origin(gc, x,y);

			gdk_draw_pixbuf(area->window, NULL,
					pixbuf,
					0, 0, x, y, -1, -1, GDK_RGB_DITHER_NONE, 0, 0);

			g_free(gc);
			g_free(mask);
		}

	/*      g_free(pixbuf); */
     
	return 0;
}

gint draw_pixmap(GdkWindow *window, gchar *file, gboolean is_selected, gdouble x, gdouble y)
{
	GdkPixbuf *pixbuf;
	GdkBitmap *mask = NULL;
	GdkGC *gc;


	pixbuf = gdk_pixbuf_new_from_file(get_file_from_data_dir(g_strdup(file)), NULL);

	/* Start of _OLD_ :P DindinX' code */
     
	gc = gdk_gc_new(window);

	gdk_gc_set_clip_mask(gc, mask);
	/* x and y must be int but the prototype gets double from the drawing area */
	/* There might be a bug here someday: try setting gint x, gint y, instead of gdouble */
	gdk_gc_set_clip_origin(gc, x, y);

        if ( is_selected )
                gdk_pixbuf_fill(pixbuf, 0xff000fff);

	gdk_draw_pixbuf(window, NULL,
			pixbuf,
			0, 0, x, y, -1, -1,
			GDK_RGB_DITHER_NONE, 0, 0);
     
	g_free(gc);
	g_free(mask);
	/* End of _OLD_ :P DindinX' code */

	return 0;
}


void draw_red_rectangle(GtkWidget *area, gint x1, gint y1, gint x2, gint y2)
{
        GdkGC    * gc;
        GdkColor color;


        gc = gdk_gc_new(area->window);

        color.red   = 65535;
        color.green = 0;
        color.blue  = 0;
        gdk_colormap_alloc_color(gtk_widget_get_colormap(area), &color, TRUE, TRUE);
        gdk_gc_set_background(gc, &color);

        gdk_draw_rectangle(area->window, gc,
                           TRUE, x1, y1, x2, y2);

}


void draw_eighth_up(GtkWidget *area, gboolean selected, gint x, gint y)
{
     draw_note(area, QUARTER_HEAD, selected, x, y+21, x+8, y+21, x+8, y);
     draw_note(area, EIGHTH_UP, FALSE, x+8, y, 0, 0, 0, 0);
}

void draw_eighth_down(GtkWidget *area, gboolean selected, gint x, gint y)
{
     draw_note(area, QUARTER_HEAD, selected, x, y + 21, x, y + 46, x, y + 25);
     draw_note(area, EIGHTH_DOWN, FALSE, x, y + 30, 0, 0, 0, 0);
}

void draw_sixteenth_up(GtkWidget *area, gboolean selected, gint x, gint y)
{
     draw_note(area, QUARTER_HEAD, selected, x,y+21, x+8,y+21, x+8,y);
     draw_note(area, SIXTEENTH_UP, FALSE, x+8, y, 0,0, 0,0);
}

void draw_sixteenth_down(GtkWidget *area, gboolean selected, gint x, gint y)
{
     draw_note(area, QUARTER_HEAD, selected, x,y+21, x,y+46, x,y+25);
     draw_note(area, SIXTEENTH_DOWN, FALSE, x, y+30, 0,0, 0,0);
}

gint average(gint staff)
{
/*
 * This functions returns TRUE if the note is lower than the average of the score
 */

/*      gint average = 0; */
/*      gint sum = 0; */

/*      average = (Score.Staff[staff].extremity_end_y - Score.Staff[staff].extremity_begin_y)/2; */
/*      sum = average+Score.Staff[staff].extremity_begin_y; */

/*      return sum; */
	return 0;

}

gint index_object(gint staff, gint x)
{
/*
 * This functions returns the position of an element into the score.
 * You must provide the staff number where to look for, and the x position of the object
 */

/* 	GList * tmp_list; */

/* 	gint counter = 1; */
	
/* 	for ( tmp_list = Score.Staff[staff].Object_list ; tmp_list ; tmp_list = g_list_next(tmp_list)) */
/* 	{ */
/* 		Object_t *object; */
		
/* 		object = (Object_t *)tmp_list->data; */
		
/* 		if (object->x <= x) { */
/* 			counter++; */
/* 		} */
		
/* 	}                */
	
/* 	return counter; */
	return 0;
}

extern
gint draw_staff(GtkWidget *area, guint16 nb_lines, gdouble space_btwn_line, gdouble x1, gdouble y1, gdouble x2,
                gboolean line, gboolean selected)
{
	/*
	 *   nb_line is the number of lines for your score, it's usualy 5 and may be 6 for a tablature
	 *   space_btwn_line is the space (in pixels) to leave between each line
	 *   x1 is the x coordonate where to start
	 *   y1 is the y coordonate where to start
	 *   x2 is the x coordonate where to stop
	 *   line: if true, draw a line a the beginning of the staff
	 *   selected: if true, draw a blue staff
	 */

	GdkGC *gc;
	GdkColor color;
	gdouble counter = 0; /* set to add this value to the x1 position */
	gint16 i = 0;
	Score_t *score = score_get_from_widget(area);

	gc = gdk_gc_new(area->window);

/*         x2 = 300; */

/* 	g_print("|draw.c| draw_staff(nb_lines = %d, space_btwn_line = %f, x1 = %f, y1 = %f, x2 = %f, line = %d, selected = %d)\n", nb_lines, space_btwn_line, x1, y1, x2, line, selected); */
/* 	debug_msg(g_strdup_printf("|draw.c| draw_staff(nb_lines = %d, space_btwn_line = %f, x1 = %f, y1 = %f, x2 = %f, line = %d, selected = %d)\n", nb_lines, space_btwn_line, x1, y1, x2, line, selected)); */

	if ( selected ) {
		color.red   = 0;
		color.green = 0;
		color.blue  = 65535;
		gdk_colormap_alloc_color(gtk_widget_get_colormap(area), &color, TRUE, TRUE);
		gdk_gc_set_foreground(gc, &color);
	}

	while ( i < nb_lines ) {
		gdk_draw_line(area->window, gc,
			      x1, y1+counter, x2, y1+counter);
		counter = counter + space_btwn_line + 1;
		++i;

		score->width = x2 + 300;
	}

	if ( line ) {
		gdk_draw_line(area->window, gc,
			      x1, y1,
			      x1, y1+(nb_lines-1)+(nb_lines-1)*space_btwn_line);
	}

	return 0;
}

gint draw_point(GtkWidget *area, gint x, gint y)
{

	GdkGC *gc;

	gc = gdk_gc_new(area->window);

	gdk_draw_point(area->window, gc, x, y);
	gdk_draw_point(area->window, gc, x + 1, y);
	gdk_draw_point(area->window, gc, x, y + 1);
	gdk_draw_point(area->window, gc, x + 1, y + 1);

	return 0;
}

void draw_tie(gint x, gint y, gint width, gint heigth)
{
/*      GdkGC *gc; */

/*      gc = gdk_gc_new(Score.area->window); */

/*      gdk_draw_arc(Score.area->window, gc, */
/*                   FALSE, x, y, width, heigth, 0, 90*128); */

}


/* This function will draw the lines when you have your object above or below the staff */
void draw_lines_for_object(gint staff, gboolean above, gint nb_lines, gint x)
{
     /* staff: is the staff number (usualy given by the function get_staff_selected())
        above: will draw the lines above if TRUE of below if FALSE of the staff
        nb_lines: the number of lines to draw
        x: the x position of the object
     */

/*      GdkGC * gc; */
/*      gint i; */

/*      if (nb_lines < 1) */
/*           g_message("\ndraw_lines_for_objects(): nb_lines < 1\n"); */

/*      gc = gdk_gc_new(Score.area->window); */

/*      if (above) { */
/*           for (i = 1; i <= nb_lines; i++) */
/*           { */
/*                gdk_draw_line(Score.area->window, gc, */
/*                              x-4, Score.Staff[staff].extremity_begin_y - i*Score.Staff[staff].space_btw_lines - 1*i, */
/*                              x+12, Score.Staff[staff].extremity_begin_y - i*Score.Staff[staff].space_btw_lines - 1*i); */
/*           }  */
/*      }   */

/*      if (!above) { */
/*           for (i = 1; i <= nb_lines; i++) */
/*           { */
/*                gdk_draw_line(Score.area->window, gc, */
/*                              x-4, Score.Staff[staff].extremity_end_y + i*Score.Staff[staff].space_btw_lines + 1*i, */
/*                              x+15, Score.Staff[staff].extremity_end_y + i*Score.Staff[staff].space_btw_lines + 1*i); */
/*           }  */
/*      } */
     
}

gint draw_placement(gint x, gint y)
{
/*      GdkGC *gc; */

/*      gc = gdk_gc_new(Score.area->window); */

/*      gdk_draw_line(Score.area->window, gc, */
/*                    x-5, y-4, x-5, y+4); */

/*      gdk_draw_line(Score.area->window, gc, */
/*                    x+10, y-4, x+10, y+4); */

     return 0;

}

void draw_line(GtkWidget *area, gint red, gint green, gint blue, gint x1, gint y1, gint x2, gint y2)
{
	GdkGC *gc;
	GdkColor color;

	gc = gdk_gc_new(area->window);

	color.red = red;
	color.green = green;
	color.blue = blue;
	gdk_colormap_alloc_color(gtk_widget_get_colormap(area), &color, TRUE, TRUE);
	gdk_gc_set_foreground(gc, &color);

	gdk_draw_line(area->window, gc, x1, y1, x2, y2);

}

/* Just to have that beautiful white color on the drawing area for the Score */
/* gboolean white_score(GtkWidget *drawingarea) */
/* { */
/*         GdkColor  gdk_color; */
/*         GdkColormap * colormap; */
        
/*         colormap = gdk_drawable_get_colormap(drawingarea->window); */
/*         gdk_color.red = 65535; */
/*         gdk_color.green = 65535; */
/*         gdk_color.blue = 65535; */

/*         gdk_colormap_alloc_color(colormap, &gdk_color, TRUE, TRUE); */
        
/*         gdk_window_set_background(drawingarea->window, &gdk_color); */

/*         return TRUE; */
/* } */

gboolean colorize_drawingarea(GtkWidget *drawingarea, guint red, guint green, guint blue)
{
        GdkColor  gdk_color;
        GdkColormap * colormap;

        colormap = gdk_drawable_get_colormap(drawingarea->window);
        gdk_color.red = red;
        gdk_color.green = green;
        gdk_color.blue = blue;

        gdk_colormap_alloc_color(colormap, &gdk_color, TRUE, TRUE);
        
        gdk_window_set_background(drawingarea->window, &gdk_color);

        return TRUE;
}

void draw_page_limit(GtkWidget *area, gint x)
{
	GdkGC *gc;
	GdkColor color;
	Score_t *score = score_get_from_widget(area);
	
	gc = gdk_gc_new(area->window);

	color.red   = 55000;
	color.green = 55000;
	color.blue  = 55000;
	gdk_colormap_alloc_color(gtk_widget_get_colormap(area), &color, TRUE, TRUE);
	gdk_gc_set_foreground(gc, &color);


	gdk_draw_line(area->window, gc,
		      x, 0,
		      x, score->height);
}

/* That will draw the extension needed for notes on staff */
/* It's usefull when you need to see where the note is placed out from the staff */
void draw_staff_extension(GtkWidget *area,Staff_t *staff, gint position, gdouble x)
{
	GdkGC *gc;
	GdkColor color;

	gint draw_dash_helper = 0;
	gint i = 0;

	gdouble extremity_end_y = 0;
	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;


	gc = gdk_gc_new(area->window);

	color.red   = 00000;
	color.green = 00000;
	color.blue  = 00000;

	gdk_colormap_alloc_color(gtk_widget_get_colormap(area), &color, TRUE, TRUE);
	gdk_gc_set_foreground(gc, &color);

	switch(staff->key) {
	case NO_KEY:
	case TREBLE_KEY:
		if (position > 7) {
			draw_dash_helper = position - 6;

			for ( i = 1; i <= draw_dash_helper / 2; i ++ ) {
				gdk_draw_line(area->window, gc,
					      x - 4,
					      staff->extremity_begin_y - (i * staff->space_btw_lines) - i,
					      x + 13,
					      staff->extremity_begin_y - (i * staff->space_btw_lines) - i);
			}
		}

		if (position < -3) {

			draw_dash_helper = position + 2;

			draw_dash_helper = -draw_dash_helper;

			for ( i = 1; i <= draw_dash_helper / 2; i++ ) {
				gdk_draw_line(area->window, gc,
					      x - 4,
					      extremity_end_y + (i * staff->space_btw_lines) + i,
					      x + 13,
					      extremity_end_y + (i * staff->space_btw_lines) + i);
			}
		}



		break;
	}
}
