/*
 * spacings.h
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

#ifndef _SPACINGS_H_
#define _SPACINGS_H_

#include <glib.h>


gint get_key_signature_spacing(gint key_signature);
void set_spacings_init(void);
void set_values_spacings_spinbuttons(void);
void update_spacings_from_spinbuttons(void);

/* Structure for spacing settings */

typedef struct Accidentals_t
{
	gint sbaan; 		/* Space between accidental and note */
	gint sbaon; 		/* Space between accidentals on note */
} Accidentals_t;

typedef struct AugmentationDots_t
{
	gint sbdan;		/* Space between dot and note */
	gint sbd;		/* Space between dots */
	gint vaod;		/* Vertical adjustment of dot */
} AugmentationDots_t;

typedef struct Barlines_t
{
	gint hlt;		/* Heavy line thickness */
	gint tlt;		/* Thin line thickness */
	gint sbhatl;		/* Space between heavy and thin line */
	gint swdb;		/* Space within double barlines */
	gint sblad;		/* Space between line and dot */
	gint sbd;		/* Space between dots */
	gint sab;		/* Space after barline */
} Barlines_t;

typedef struct Beams_t
{
	gint bt;		/* Beam thickness */
} Beams_t;

typedef struct Clefs_t
{
	gint sb;		/* Space before */
	gint sa;		/* Space after */
} Clefs_t;

typedef struct Colors_t
{
	gint clefs;
	gint barlines;
	gint staves;
} Colors_t;

typedef struct KeySignatures_t
{
	gint sbksa;		/* Space between key signature accidentals */
	gint saks;		/* Space after key signature */
} KeySignatures_t;

typedef struct Measures_t
{
        gdouble xpsfm;             /* X position start for the first measure */
        gdouble ypsfm;             /* Y position start for the first measure */
        gint slbm;              /* Space left between measures */
} Measures_t;

typedef struct NotesRests_t
{
	gint sa_doublewhole;	/* Space after doublewhole */
	gint sa_doublewholerest;
	gint sa_whole;
	gint sa_wholerest;
	gint sa_half;
	gint sa_halfrest;
	gint sa_quarter;
	gint sa_quarterrest;
	gint sa_eighth;
	gint sa_eighthrest;
	gint sa_sixteenth;
	gint sa_sixteenthrest;
	gint sa_thirtysecond;
	gint sa_thirtysecondrest;
	gint sa_sixtyfourth;
	gint sa_sixtyfourthrest;
} NotesRests_t;

typedef struct Tempo_t
{
        gdouble xpfm;              /* X position from measure */
        gdouble ypfm;              /* Y position from measure */
} Tempo_t;

typedef struct TimeSignatures_t
{
	gdouble sats;		/* Space after time signature */
} TimeSignatures_t;

typedef struct Spacings_t
{
	struct Accidentals_t      Accidentals;
	struct AugmentationDots_t AugmentationDots;
	struct Barlines_t         Barlines;
	struct Beams_t            Beams;
	struct Clefs_t            Clefs;
	struct Colors_t           Colors;
	struct KeySignatures_t    KeySignatures;
        struct Measures_t         Measures;
	struct NotesRests_t       NotesRests;
        struct Tempo_t            Tempo;
	struct TimeSignatures_t   TimeSignatures;
} Spacings_t;


extern Spacings_t Spacings;

#endif
