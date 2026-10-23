/*
 * abc.c
 * plugin to read/write abc music
 *
 * (C) Copyright 2003 Jerome Marrec
 * e-mail : j.marrec@10pouces.homelinux.net
 * URL    : http://10pouces.homelinux.net
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
 *
 * ChangeLog:
 * Sat May 29 2004: Sebastien Tricaud <toady@gscore.org>
 *                  - switched to the new plugin API
 *
*/


#include <glib.h>

#include "plugin.h"
#include "gscore.h"

#include "export-abc.h"
#include "import-abc.h"

#define API_VERSION 1
#define PLUGIN_TYPE GSCORE_PLUGIN_FILTER

#define PLUGIN_NAME "ABC"
#define PLUGIN_VERSION "1"
#define PLUGIN_SUMMARY "ABC"
#define PLUGIN_DESCRIPTION "Import/Export ABC Music"
#define PLUGIN_AUTHOR "Jerome Marrec <j.marrec@10pouces.org>"
#define PLUGIN_HOMEPAGE GSCORE_HOMEPAGE


/* Extensions for the filter */
const gchar *extensions[] = {"abc", NULL};


/* Filter related functions */
gboolean abc_filter_export (Score_t *score,
                            const gchar *filename,
                            void *user_data);
gboolean abc_filter_import (Score_t **score,
                            const gchar *filename,
                            void *user_data);
/* Plugin multi-purpose functions */
void init(void);		/* When gscore application initialy starts up */
void quit(void);		/* When gscore application exits */
void run (void);		/* To run anything everytime the plugin is called */


/* The main function */
GscorePlugin *
gscore_plugin_init(void)
{
	GscorePlugin *plugin;


	/* Some memory allocations */
	GSCORE_PLUGIN_INIT(plugin);

	plugin->info->api_version = API_VERSION;
	plugin->info->type        = PLUGIN_TYPE;
	plugin->info->priority    = GSCORE_PRIORITY_DEFAULT;
	plugin->info->name        = PLUGIN_NAME;
	plugin->info->version     = PLUGIN_VERSION;
	plugin->info->summary     = PLUGIN_SUMMARY;
	plugin->info->description = PLUGIN_DESCRIPTION;
	plugin->info->author      = PLUGIN_AUTHOR;
	plugin->info->homepage    = PLUGIN_HOMEPAGE;

	plugin->init_proc   = init;
	plugin->quit_proc   = quit;
	plugin->run_proc    = run;

	plugin->filter->extensions = extensions;
	plugin->filter->import     = abc_filter_import;
	plugin->filter->export     = abc_filter_export;

	return plugin;
}


/* Functions */
void init(void)
{
	g_print("Initializing %s\n", PLUGIN_NAME);
}

void quit(void)
{
	g_print("Quitting %s\n", PLUGIN_NAME);
}

void run(void)
{
	g_print("Running %s\n", PLUGIN_NAME);
}

/* Filter functions */
gboolean abc_filter_export (Score_t *score,
                            const gchar *filename,
                            void *user_data)
{

	save_file(filename, score);
	return TRUE;
}

gboolean abc_filter_import (Score_t **score,
                            const gchar *filename,
                            void *user_data)
{
/*         g_print("Import function called for %s\n", PLUGIN_NAME); */
/* 	g_print("Loading %s\n", filename); */

        GSCORE_PLUGIN_STRUCT_INIT(*score);

	return abc_load_file(filename,*score);
}
