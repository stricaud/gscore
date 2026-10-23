/*
 * constants.h
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
#ifndef _CONSTANT_H_
#define _CONSTANT_H_

#define APP_VERSION "0.0.9"

/***
 *** Pixmaps location
 ***/
#define LOGO_GILBERT "pixmaps/gilbert.xpm"

#define DOUBLEWHOLE_HEAD "pixmaps/noteheads-1.xpm"
#define WHOLE_HEAD "pixmaps/noteheads-0.xpm"
#define HALF_HEAD "pixmaps/noteheads-1bis.xpm"
#define QUARTER_HEAD "pixmaps/noteheads-2.xpm"

#define NATURAL_ACCIDENTAL "pixmaps/accidentals0.xpm"
#define SHARP_ACCIDENTAL "pixmaps/accidentals-1.xpm"
#define FLAT_ACCIDENTAL "pixmaps/accidentals-1bis.xpm"

#define EIGHTH_UP "pixmaps/flags-u3.xpm"
#define EIGHTH_DOWN "pixmaps/flags-d3.xpm"
#define SIXTEENTH_UP "pixmaps/flags-u4.xpm"
#define SIXTEENTH_DOWN "pixmaps/flags-d4.xpm"

#define DOUBLEWHOLE_REST "pixmaps/rests-0.xpm" 
#define WHOLE_REST "pixmaps/rests-Oo.xpm" 
#define HALF_REST "pixmaps/rests-1o.xpm" 
#define QUARTER_REST "pixmaps/rests-2bis.xpm"
#define EIGHTH_REST "pixmaps/rests-3bis.xpm" 
#define SIXTEENTH_REST "pixmaps/rests-4.xpm" 

/* tabs */
#define ONE "pixmaps/one.xpm"
#define TWO "pixmaps/two.xpm"
#define THREE "pixmaps/three.xpm"
#define FOUR "pixmaps/four.xpm"
#define FIVE "pixmaps/five.xpm"
#define SIX "pixmaps/six.xpm"
#define SEVEN "pixmaps/seven.xpm"
#define EIGHT "pixmaps/eight.xpm"
#define NINE "pixmaps/nine.xpm"

/* keys */
#define CLEF_C "pixmaps/clefs-C.xpm"
#define CLEF_C_CHANGE "pixmaps/clefs-C_change.xpm"
#define CLEF_F "pixmaps/clefs-F.xpm"
#define CLEF_F_CHANGE "pixmaps/clefs-F_change.xpm"
#define CLEF_G "pixmaps/clefs-G.xpm"
#define CLEF_G_CHANGE "pixmaps/clefs-G_change.xpm"
#define CLEF_PERCUSSION "pixmaps/clefs-percussion.xpm"
#define CLEF_PERCUSSION_CHANGE "pixmaps/clefs-percussion_change.xpm"
#define CLEF_TAB "pixmaps/clefs-tab.xpm"
#define CLEF_TAB_CHANGE "pixmaps/clefs-tab_change.xpm"

/* accidentals */
#define ACCIDENTAL_SHARP "pixmaps/accidentals-1.xpm"
#define ACCIDENTAL_FLAT "pixmaps/accidentals-1bis.xpm"
#define ACCIDENTAL_DOUBLEFLAT "pixmaps/accidentals-2bis.xpm"
#define ACCIDENTAL_DOUBLESHARP "pixmaps/accidentals-2bis2.xpm"
#define ACCIDENTAL_LEFTPAREN "pixmaps/accidentals-leftparen.xpm"
#define ACCIDENTAL_RIGHTPAREN "pixmaps/accidentals-rightparent.xpm"
#define ACCIDENTAL_NATURAL "pixmaps/accidentals0.xpm"

/* time signature */
#define TIME_SIG_24 "pixmaps/p_24.xpm"
#define TIME_SIG_68 "pixmaps/p_24.xpm"
#define TIME_SIG_CT "pixmaps/common_time.xpm"
#define TIME_SIG_AB "pixmaps/alla_breve.xpm"

/* CURSOR */
#define CURSOR 0

/* KEYS */
#define NO_KEY 1
#define TREBLE_KEY 2
#define ALTO_KEY 3
#define TENOR_KEY 4
#define BASS_KEY 5
#define PERCUSSION_KEY 6
#define TREBLE_8VB_KEY 7
#define BASS_8VB_KEY 8
#define BARITONE_KEY 9
#define TAB_KEY 10

/* NOTES */
#define DOUBLEWHOLE 11
#define WHOLE 12
#define HALF 13
#define QUARTER 14
#define EIGHTH 15
#define SIXTEENTH 16
#define THIRTYSECOND 17
#define SIXTYFOURTH 18

/* ACCIDENTALS  */
#define NATURAL 30
#define SHARP 31
#define FLAT 32
#define DOUBLEFLAT 33
#define DOUBLESHARP 34
#define COURTESY 35

/* RESTS */
#define DOUBLEWHOLEREST 41
#define WHOLEREST 42
#define HALFREST 43
#define QUARTERREST 44
#define EIGHTHREST 45
#define SIXTEENTHREST 46
#define THIRTYSECONDREST 47
#define SIXTYFOURTHREST 48

#define DECRESCENDO 50
#define CRESCENDO 51

/* BARLINES */
#define BARLINE 60
#define BARLINE_SINGLE 61
#define BARLINE_DOUBLE 62
#define BARLINE_OPENREPEAT 63
#define BARLINE_CLOSEREPEAT 64
#define BARLINE_OPENCLOSEREPEAT 65
#define BARLINE_OPEN 66
#define BARLINE_CLOSE 67

/* TIME SIGNATURES */
#define TIME_SIGNATURE_NORMAL 70
#define TIME_SIGNATURE_COMMON_TIME 71
#define TIME_SIGNATURE_ALLA_BREVE 72

/* KEY SIGNATURES */
#define KEY_SIGNATURE_EMPTY 80

#define KEY_SIGNATURE_TREBLE_EMPTY 81
#define KEY_SIGNATURE_TREBLE_A_SHARP 82
#define KEY_SIGNATURE_TREBLE_B_SHARP 83
#define KEY_SIGNATURE_TREBLE_C_SHARP 84
#define KEY_SIGNATURE_TREBLE_D_SHARP 85
#define KEY_SIGNATURE_TREBLE_E_SHARP 86
#define KEY_SIGNATURE_TREBLE_F_SHARP 87
#define KEY_SIGNATURE_TREBLE_G_SHARP 88
#define KEY_SIGNATURE_TREBLE_A_FLAT 89
#define KEY_SIGNATURE_TREBLE_B_FLAT 90
#define KEY_SIGNATURE_TREBLE_C_FLAT 91
#define KEY_SIGNATURE_TREBLE_D_FLAT 92
#define KEY_SIGNATURE_TREBLE_E_FLAT 93
#define KEY_SIGNATURE_TREBLE_F_FLAT 94
#define KEY_SIGNATURE_TREBLE_G_FLAT 95

/* DYNAMICS */
#define DYNAMIC_PPPP 200
#define DYNAMIC_PPP 201
#define DYNAMIC_PP 202
#define DYNAMIC_P 203
#define DYNAMIC_MP 204
#define DYNAMIC_MF 205
#define DYNAMIC_F 206
#define DYNAMIC_FF 207
#define DYNAMIC_FFF 208

/* Clipboard */
/* When you select quarters and you want to change them into half, this is called */
#define ACTION_CHANGE_OBJECT 300
#define ACTION_CHANGE_STAFF 301	      /* Change Key Signature, etc. */
#define ACTION_ADD_OBJECT 302         /* When you add an object */
#define ACTION_ADD_STAFF 303          /* When you add a staff */
#define ACTION_DEL_OBJECT 304
#define ACTION_DEL_STAFF 305


/* MIDI */
#define MIDI_GRAND_PIANO 1000


/* SPECIAL */
#define NO_ACCIDENTAL -1


/* SPACINGS */
/* The user cannot change that so that all notes start at the same point in the staff */
/* Cleaner for the eyes, not for the code ;) but nobody cares about it and it works! */
#define STANDARD_KEY_SIZE 15    
#define STANDARD_TIME_SIGNATURE_SIZE 15
#define STANDARD_ACCIDENTAL_SIZE 8

#endif
