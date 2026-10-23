/* #include <glib.h> */

/* typedef struct _Object               Object; */
/* typedef struct _Note                 Note; */
/* typedef struct _Tab                  Tab; */
/* typedef struct _Staff                Staff; */
/* typedef struct _Indentity            Indentity; */
/* typedef struct _ScorePluginInterface ScorePI; */

/* /\* What we should display on the score *\/ */
/* /\* typedef enum p_display_e *\/ */
/* /\* { *\/ */

/* /\* 	PD_MEASURE_NUMBER    = 1 << 1, *\/ */
/* /\* 	PD_INSTRUMENT        = 1 << 2, *\/ */
/* /\* 	PD_CLEFS             = 1 << 3, *\/ */
/* /\* 	PD_SCORE_EXPRESSIONS = 1 << 4, *\/ */
/* /\* 	PD_BARLINES          = 1 << 5, *\/ */
/* /\* 	PD_ENDINGS           = 1 << 6, *\/ */
/* /\* 	PD_KEY_SIG           = 1 << 7, *\/ */
/* /\* 	PD_TIME_SIG          = 1 << 8, *\/ */
/* /\* 	PD_STAFF_NAME        = 1 << 9, *\/ */

/* /\* } p_display_e; *\/ */

/* typedef enum p_accidentals_e */
/* { */

/* 	PA_SHARP       = 1 << 1, */
/* 	PA_DOUBLESHARP = 1 << 2, */
/* 	PA_FLAT        = 1 << 3, */
/* 	PA_DOUBLEFLAT  = 1 << 4, */
/* 	PA_NATURAL     = 1 << 5, */
/* 	PA_PARENTHESIS = 1 << 6, */

/* } p_accidentals_e; */

/* typedef enum p_object_e */
/* { */

/* 	PO_BEAMED          = 1 << 1, */
/*         PO_LAST_BEAMED     = 1 << 2, */
/* 	PO_TIE_START       = 1 << 3, */
/* 	PO_AUTOSTEM        = 1 << 4, */
/* 	PO_STEMUP          = 1 << 5, */
/* 	PO_STEMDOWN        = 1 << 6, */
/* 	PO_AUGMENTATIONDOT = 1 << 7, */
/* 	PO_STACCATO        = 1 << 8, */
/* 	PO_GRACE           = 1 << 9, */

/* } p_object_e; */

/* struct _Object */
/* { */

/*         gulong           id;      /\* Unique ID *\/ */

/*         gint             type; /\* key_signature, note (quarter, rest, ...), tab *\/ */
/*         p_object_e       nature;  /\* Stem Up/Down, Staccato, Grace... ? *\/ */
/* 	p_accidentals_e  accidentals; */

/*         gint             x; */
/*         gint             y; */
/*         gint             x2; /\* we need supplementary coordinates for *\/ */
/*         gint             y2; /\* crescendo etc..     *\/ */
/*         gint             x3; /\* and this is used for slurs, tied notes *\/ */
/*         gint             y3; */

/*         gint             pitch;        */

/*         gint             tab_number; */

/*         gboolean         is_selected;  /\* Is the object selected ? if so, change its color *\/ */


/* }; */

/* struct _Staff */
/* { */

/* /\* 	p_display_e  display; /\\* What do we display ? (default: everything) *\\/ *\/ */

/*         gchar      nb_lines;    /\* How many lines for the staff (usually 5) *\/ */
/*         gchar      space_btw_lines; /\* How far are the lines with each other *\/ */

/*         gboolean   is_selected; /\* TRUE if the staff is selected *\/ */

/*         gint       key;         /\* The key for the staff *\/ */
/*         gint       key_signature; /\* The key signature for the staff*\/ */

/*         /\* time_signature[0] = TIME_SIGNATURE_NORMAL | TIME_SIGNATURE_COMMON_TIME | TIME_SIGNATURE_ALLA_BREVE *\/ */
/*         /\* time_signature[1] = Number of beats *\/ */
/*         /\* time_signature[2] = Beat duration *\/ */
/*         guint8     time_signature[2]; /\* The time signature for the staff *\/ */

/*         gint       measure_number; /\* The number of the measure on which we are working *\/ */
/*         gint       total_measures; /\* How many measures in total *\/ */

/*         /\* Useful positions *\/ */
/*         glong      extremity_begin_x; /\* Position X where the staff begins *\/ */
/*         glong      extremity_begin_y; /\* Position Y where the staff begins *\/ */
/*         gint       current_x;   /\* The current position *\/ */

/*         /\* The Instrument for the staff (MIDI) *\/ */
/*         gint       midi_instrument;  */

/*         GList    * Object_list; */
/*         struct _Object  * Object; */

/* }; */

/* struct _Identity */
/* { */
        
/*         gchar *title; */
/*         gchar *subtitle; */
/*         gchar *composer; */
/*         gchar *copyright; */
/*         gchar *instrument; */
/*         gchar *filename; */
        
/* }; */

/* /\* ScorePluginInterface *\/ */
/* struct _ScorePluginInterface	 */
/* { */

/*         gint          tempo;             /\* Tempo of the score *\/ */
/*         gulong        object_id;    /\* Every object in the score has a *unique* ID *\/ */
/*         gint          nb_staves;      /\* How many staves in the score *\/ */
/*         gint          width;        /\* How wide is the drawing area (score) widget ? *\/ */
/*         gint          height;       /\* How high is the drawing area (score) widget ? *\/ */
/*         gint          staff_selected; /\* The number of the staff selected *\/ */
/*         glong         staff_extremity_end_x; /\* Position X where the staff ends *\/ */
/*         gint          staff_startx; /\* Where we start to put the notes *\/ */

/*         struct _Identity    * Identity; /\* Useful informations *\/ */

/*         /\* Staves tracked in a list, to have many staves as your memory allow you to have *\/ */
/*         GList       * Staff_list; */
/*         Staff       * Staff; */

/* }; */


