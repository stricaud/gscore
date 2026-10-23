/* -*- mode:C; tab-width:8; c-default-style:linux; c-basic-offset:8; indent-tabs-mode:nil -*- */
/*
 * draw_barline.c
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

#include "gscore.h"
#include "common.h"
#include "spacings.h"
#include "draw_barline.h"

gint draw_barline_single(GtkWidget *area, Staff_t *staff, gint x)
{
	GdkGC * gc;

	guint8 thickness = 0;
	gdouble extremity_end_y = 0;

	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;

	gc = gdk_gc_new(area->window);

	for ( thickness = 0; thickness < Spacings.Barlines.tlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      x + thickness, 
			      staff->extremity_begin_y, 
			      x + thickness, 
			      extremity_end_y);
	}
     
	return 0;
}

gint draw_barline_double(GtkWidget *area, Staff_t *staff, gint x)
{
	GdkGC * gc;
     
	guint8 thickness = 0;
	gdouble extremity_end_y = 0;

	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;


	gc = gdk_gc_new(area->window);

     
	for ( thickness = 0; thickness < Spacings.Barlines.tlt; thickness++ ) {

		gdk_draw_line(area->window, gc,
			      x + thickness, staff->extremity_begin_y,
			      x + thickness, extremity_end_y);

		gdk_draw_line(area->window, gc,
			      x + Spacings.Barlines.swdb + thickness, staff->extremity_begin_y,
			      x + Spacings.Barlines.swdb + thickness, extremity_end_y);

	}

	return 0;
}

gint draw_barline_openrepeat(GtkWidget *area, Staff_t *staff, gint x)
{
	GdkGC * gc;

	gint average = 0;
     
	guint8 thickness = 0;
	gdouble extremity_end_y = 0;

	gdouble new_x = 0;	/* According to the thickness... the x pos where the dots should be change */

	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;


	gc = gdk_gc_new(area->window);

	for ( thickness = 0; thickness < Spacings.Barlines.hlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      x + thickness, staff->extremity_begin_y,
			      x + thickness, extremity_end_y);
	}

	new_x += x;
	new_x += Spacings.Barlines.hlt;

	for ( thickness = 0; thickness < Spacings.Barlines.tlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      new_x + Spacings.Barlines.sbhatl + thickness, staff->extremity_begin_y,
			      new_x + Spacings.Barlines.sbhatl + thickness, extremity_end_y);

	}

	new_x += Spacings.Barlines.tlt;
	new_x += Spacings.Barlines.sbhatl;
	new_x += Spacings.Barlines.sblad;

	average = (extremity_end_y - staff->extremity_begin_y) / 2;

	gdk_draw_point (area->window, gc, 
			new_x - 1, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, 
			new_x - 1, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, 
			new_x, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, 
			new_x, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));

	gdk_draw_point (area->window, gc, 
			new_x - 1, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, 
			new_x - 1, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, 
			new_x, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, 
			new_x, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));

	return 0;

}

gint draw_barline_closerepeat(GtkWidget *area, Staff_t *staff, gint x)
{
	GdkGC * gc;
	gint average = 0;
     
	guint8 thickness = 0;
	gdouble extremity_end_y = 0;

	gdouble new_x = 0;	/* According to the thickness... the x pos where the dots should be change */

	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;


	gc = gdk_gc_new(area->window);


	average = (extremity_end_y - staff->extremity_begin_y)/2;

	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));

	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));

	new_x += x + 1;

	for ( thickness = 0;  thickness < Spacings.Barlines.tlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      new_x + Spacings.Barlines.sblad + thickness, staff->extremity_begin_y,
			      new_x + Spacings.Barlines.sblad + thickness, extremity_end_y);
	}

	new_x += Spacings.Barlines.tlt;
	new_x += Spacings.Barlines.sblad;

	for ( thickness = 0; thickness < Spacings.Barlines.hlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      new_x + Spacings.Barlines.sbhatl + thickness, staff->extremity_begin_y,
			      new_x + Spacings.Barlines.sbhatl + thickness, extremity_end_y);
	}


	return 0;

}

gint draw_barline_opencloserepeat(GtkWidget *area, Staff_t *staff, gint x)
{
	GdkGC * gc;
	gint average = 0;
     
	guint8 thickness = 0;
	gdouble extremity_end_y = 0;

	gdouble new_x = 0;	/* According to the thickness... the x pos where the dots should be change */

	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1;


	gc = gdk_gc_new(area->window);


	average = (extremity_end_y - staff->extremity_begin_y)/2;

	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));

	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, x + 1, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));

	new_x += x + 1;

	for ( thickness = 0;  thickness < Spacings.Barlines.tlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      new_x + Spacings.Barlines.sblad + thickness, staff->extremity_begin_y,
			      new_x + Spacings.Barlines.sblad + thickness, extremity_end_y);
	}

	new_x += Spacings.Barlines.tlt;
	new_x += Spacings.Barlines.sblad;

	for ( thickness = 0; thickness < Spacings.Barlines.hlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      new_x + Spacings.Barlines.sbhatl + thickness, staff->extremity_begin_y,
			      new_x + Spacings.Barlines.sbhatl + thickness, extremity_end_y);
	}

	new_x += Spacings.Barlines.sbhatl;
	new_x += Spacings.Barlines.hlt;
	new_x += Spacings.Barlines.sbhatl;

	for ( thickness = 0;  thickness < Spacings.Barlines.tlt; thickness++ ) {
		gdk_draw_line(area->window, gc,
			      new_x + thickness, staff->extremity_begin_y,
			      new_x + thickness, extremity_end_y);
	}

	new_x += Spacings.Barlines.tlt;
	new_x += Spacings.Barlines.sblad;

	gdk_draw_point (area->window, gc, new_x, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, new_x, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, new_x + 1, staff->extremity_begin_y + average - Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, new_x + 1, staff->extremity_begin_y + average - (Spacings.Barlines.sbd + 1));

	gdk_draw_point (area->window, gc, new_x, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, new_x, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));
	gdk_draw_point (area->window, gc, new_x + 1, staff->extremity_begin_y + average + Spacings.Barlines.sbd);
	gdk_draw_point (area->window, gc, new_x + 1, staff->extremity_begin_y + average + (Spacings.Barlines.sbd + 1));

	return 0;

}

gint draw_barline_endstaff(GtkWidget *area, Staff_t *staff)
{
/* 	GList * listrunner; */
/* 	GdkGC * gc; */
     
/* 	guint8 thickness = 0; */
/* 	gdouble extremity_end_y = 0; */

/* 	extremity_end_y =  staff->extremity_begin_y +(staff->nb_lines - 1) * staff->space_btw_lines + staff->nb_lines - 1; */



/* 	gc = gdk_gc_new(area->window); */
     

/* 	gdk_draw_line(area->window, gc, */
/* 		      Score.staff_extremity_end_x - 5, staff->extremity_begin_y, */
/* 		      Score.staff_extremity_end_x - 5, extremity_end_y - 1); */
/* 	gdk_draw_line(area->window, gc, */
/* 		      Score.staff_extremity_end_x - 2, staff->extremity_begin_y, */
/* 		      Score.staff_extremity_end_x - 2, extremity_end_y - 1); */
/* 	gdk_draw_line(area->window, gc, */
/* 		      Score.staff_extremity_end_x - 1, staff->extremity_begin_y, */
/* 		      Score.staff_extremity_end_x - 1, extremity_end_y - 1); */
/* 	gdk_draw_line(area->window, gc, */
/* 		      Score.staff_extremity_end_x, staff->extremity_begin_y, */
/* 		      Score.staff_extremity_end_x, extremity_end_y - 1); */


	return 0;
}
