#include <glib.h>
#include <glib/gprintf.h>

#include <staff.h>
#include <objects.h>
#include <common.h>
#include <gscore.h>

#include "import-abc.h"

#include "abcp.h"

static Score_t *theScore;
static gint numStaff;
static gint numTitle;
static gulong group_id;
static object_e decor;
static const char *pitch_string = "CDEFGABcdefgab',";

typedef struct 
{
  gint nomin;
  gint denom;
} duration_t;

static duration_t base_note;

typedef struct 
{
  gint pitch;
  duration_t duration;
  accidentals_e accident;
} note_t;

static note_t *parse_note(const char *string)
{
  static note_t the_note;
  int pitch;
  int i;
  char *pos;
  int nomin=0;
  int denom=0;

  /* determine accidentals */
  i=1;
  switch(string[0])
    {
    case '=':
      the_note.accident = A_NATURAL;
      break;
    case '^':
      the_note.accident = A_SHARP;
      break;
    case '_':
      the_note.accident = A_FLAT;
      break;
    default:
      the_note.accident = A_NONE;
      i = 0;
    }  

  /* Determine the note pitch */

  pitch = strchr(pitch_string, string[i]) - pitch_string -4;
  for(i=1; string[i] == '\''; ++i){
    pitch+=8;
  }
  for(i=1; string[i]==','; ++i) {
    pitch-=8;
  }
  /* Determine the note duration */
  for(pos= (char*) string;
      (pos[0] &&
       strchr(pitch_string, pos[0])) ;
      ++pos);
/*   g_printf("Duration : '%s'\n", pos); */

  nomin = atoi(pos);
/*   g_printf("Nominator : %d\n", nomin); */
  pos = strchr(pos, '/');
  if(pos) {
    denom = atoi(pos+1);
    if(denom==0) denom = 2;
/*     g_printf("Denominator : %d\n", denom); */
  }

  /* Return result */
  the_note.pitch = pitch;
  if(nomin) {
    the_note.duration.nomin = nomin;
  } else {
    the_note.duration.nomin = 1;
  }

  if(denom) {
    the_note.duration.denom = denom;
  } else {
    the_note.duration.denom = 1;
  }

  return &the_note;
}

static int event_decor(const char *s)
{
  g_printf("Decor : '%s'\n", s);
  decor = O_AUTOSTEM;

  if(strchr(s, '.')) decor |= O_STACCATO;
  if(strchr(s, 'v')) decor |= O_MARCATODOWN;
  if(strchr(s, 'u')) decor |= O_MARCATOUP;

  return 0;
}


static int event_note(const char *s) 
{
  note_t *the_note;
  int ticks;			/* Duration expressed in ticks
				   (1 thick = 1/64th whole) */
  int num0 = 0;
  int num1 = 0;
  int type;
  gboolean is_rest = FALSE;
  
  g_print("Note: %s\n", s);
  the_note = parse_note(s);
  is_rest = (gboolean) strchr("Zz", s[0]);

  /* Convert nomin/denom to WHOLE/HALF... */
  ticks = the_note->duration.nomin * base_note.nomin * 64;
  ticks /= (the_note->duration.denom * base_note.denom);
/*   g_printf("Ticks : %d\n", ticks); */
  while(ticks) {
    if(is_even(ticks)) {
      ++num1;
    } else {
      ++num0;
    }
    ticks = ticks >> 1;
  }

  /* !!! This code supposes that constants are well ordered */
  type = SIXTYFOURTH + 1 -num0 -num1;
  if(is_rest) {
    type += (DOUBLEWHOLEREST-DOUBLEWHOLE);
  }

/*   g_printf("Type : %d\n", type); */
  
  /* TODO: Handle dotted notes */
  
  add_object(theScore, numStaff,
	     type,
	     the_note->accident, decor, 0,0,0,0,0,0,0,
	     the_note->pitch,0,FALSE);
  
  return 0;
}

static int event_field(char x, const char *s, int state)
{
  g_print("Field : %c '%s' %d\n", x, s, state);
  
  switch(x)
    {
    case 'X':
      numTitle = 0;
      numStaff = -1;
      break;
    case 'T':
      switch(numTitle) 
	{
	case 0:
	  theScore->Identity->title = g_string_new((gchar*) s);
	  ++numTitle;
	  theScore->windows_title = g_strdup((gchar*)s);
	    
/* 	g_printf("Title : %s\n", s); */
	  break;
	case 1:
	  theScore->Identity->subtitle = g_string_new((gchar*) s);
	  ++numTitle;
/* 	g_printf("Subtitle %s\n", s); */
	  break;
	default:
	  theScore->Identity->subtitle = 
	    g_string_append(theScore->Identity->subtitle,"\n");
	  theScore->Identity->subtitle = 
	    g_string_append(theScore->Identity->subtitle,s);
	  
	}
      break;
    case 'C':

      theScore->Identity->composer = g_string_new((gchar *) s);
/*     g_printf("Composer : %s\n", s); */
      break;
      
    case 'V':
      create_staff(theScore, 5, 8, 35,
		   staff_get_y_for_next(theScore));

      ++numStaff;
      theScore->staff_startx = 50;
      staff_set_key(theScore, numStaff, TREBLE_KEY);
      score_set_staff_extremity_end_x(theScore, 300);
      staff_set_key_signature(theScore, numStaff, KEY_SIGNATURE_TREBLE_EMPTY);
/*     g_printf("Voice: %s\n", s); */
      break;
    case 'Q':
/*       theScore->tempo_text= g_string_new(s); */
      theScore->tempo = atoi(s);
      break;
    case 'L':
      base_note.nomin = atoi(s);
      base_note.denom = atoi(strchr(s,'/')+1);
      break;
    case 'K':
      if(numStaff==-1) {	/* No voice has been declared */
	create_staff(theScore, 5, 8, 35,
		     staff_get_y_for_next(theScore));

	++numStaff;
        theScore->staff_startx = 50;
	staff_set_key(theScore, numStaff, TREBLE_KEY);
        score_set_staff_extremity_end_x(theScore, 300);
        staff_set_key_signature(theScore, numStaff, KEY_SIGNATURE_TREBLE_EMPTY);
      }
      
    default:
      g_printf("Other field : %c\n",x);
      break;
    }

  return 0;
}

static int event_bar(const char *s)
{
  int type;
  
/*   g_printf("Bar : '%s'\n", s); */

  type = BARLINE;
  
  if(strcmp(s, "|")==0) {
    type = BARLINE_SINGLE;
  } else if(strcmp(s, "||")==0) {
    type = BARLINE_DOUBLE;
  } else if(strcmp(s, "|:")==0) {
    type = BARLINE_OPENREPEAT;
  } else if(strcmp(s, ":|") ==0) {
    type = BARLINE_CLOSEREPEAT;
  } else if(strcmp(s, "::")==0) {
    type = BARLINE_OPENCLOSEREPEAT;
  } else if(strcmp(s, "[|") == 0) {
    type = BARLINE_OPEN;
  } else if(strcmp(s, "|]") == 0) {
    type = BARLINE_CLOSE;
  }
  add_object(theScore, numStaff,
	     type,
	     0, 0, group_id, 0,0,0,0,0,0,
	     0, 0,FALSE);
  
  return 0;
}


/** Function called whenever a token is found */
int my_handler(abcHandle h)
{
  abcScanner *scan = (abcScanner *) h;
  int t;
  char *s;
/*   char *f; */
/*   USHORT l; */
/*   USHORT c; */
  char *x;
/*   char*y; */
  
  t=abcToken(scan);
  s= (char*) abcString(scan);
/*   f=abcFilename(scan); */
/*   l=abcLine(scan); */
/*   c=abcColumn(scan); */
  x= (char*) abcStateName(abcState(scan));

  switch(t)
    {
    case T_FIELDB:
      return event_field(x[strlen(x)-1],s,abcState(scan));
      break;
      
    case T_NOTE:
      return event_note(s);
      
/*       break; */
    case T_BAR:
      return event_bar(s);
    case T_DECOR:
      return event_decor(s);
    default:
      g_printf("%s ", abcTokenName(t));
      break;
    }

  return 0;
  
}


extern gboolean abc_load_file(const gchar *filename,
			  Score_t *score)
{
/*   g_printf("Calling abc_load_file('%s')\n", filename); */
  

  theScore = score;
  base_note.nomin = 1;
  base_note.denom = 8;
  
/*   g_printf("Before abcScanFile\n"); */
  theScore->Identity->filename = g_string_new((gchar *) filename);
  theScore->Staff_list = NULL;
  theScore->tempo_text = g_string_new("");
  theScore->nb_staves = 0;
  
  abcScanFile((char *) filename, my_handler);
/*   g_printf("After abcScanFile\n"); */
  set_staff_selected(theScore, 0);

  
  return TRUE;
}

