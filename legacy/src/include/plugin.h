/* BEWARE: new plugin interface, a la GAIM */

#ifndef _GSCORE_PLUGIN_H_
#define _GSCORE_PLUGIN_H_ 

#include <glib.h>
#include <gmodule.h>
#include "gscore.h"

#define GSCORE_HOMEPAGE "http://www.gscore.org"

typedef struct _GscorePlugin           GscorePlugin;
typedef struct _GscorePluginInfo       GscorePluginInfo;
typedef struct _GscorePluginFilter     GscorePluginFilter;


typedef int GscorePluginPriority; 


#include "plugin-structure.h"
#include "constants.h"

typedef struct _plugin_pnf             plugin_pnf;

/* Plugin path and filename structure */
/* It is used by the linked list to store the filename only */
struct _plugin_pnf {
	gchar *path;
	gchar *filename;
};


/**
 * Plugin types.
 */
typedef enum
{
	GSCORE_PLUGIN_UNKNOWN  = -1,  /**< Unknown type.  */
	GSCORE_PLUGIN_FILTER = 0,     /**< Filter plugin. */
	GSCORE_PLUGIN_ACTION = 1,     /**< Action plugin. */

} GscorePluginType;

#define GSCORE_PRIORITY_DEFAULT     0
#define GSCORE_PRIORITY_HIGHEST  9999
#define GSCORE_PRIORITY_LOWEST  -9999

#define GSCORE_PLUGIN_API_VERSION 1

/* returns false on error loading score */
typedef gboolean (* GscoreImportFunction) (Score_t **score, 
					   const gchar *filename,
					   void *user_data);

/* returns false on error saving score */
typedef gboolean (* GscoreExportFunction) (Score_t *score,
					   const gchar *filename,
					   void *user_data);

struct _GscorePluginFilter
{
	const gchar **extensions;

	GscoreImportFunction import;
	GscoreExportFunction export;
};

/**
 * Detailed information about a plugin.
 */
typedef void (* GscoreInitProc) (void);
typedef void (* GscoreQuitProc) (void);
typedef void (* GscoreRunProc) (void);

struct _GscorePluginInfo
{
	unsigned int api_version;
	GscorePluginType type;
	GscorePluginPriority priority;

	char *id;
	char *name;
	char *version;
	char *summary;
	char *description;
	char *author;
	char *homepage;

};

/**
 * A plugin handle.
 */
struct _GscorePlugin
{
	gboolean loaded;                       /**< The loaded state.         */
	GModule *handle;                       /**< The module handle.        */
	char *path;                            /**< The path to the plugin.   */
	char *filename;
	GscorePluginInfo *info;                /**< The plugin information.   */
	GscorePluginFilter *filter;            /**< The plugin filter  */
	char *error;
	void *extra;                           /**< Plugin-specific data.     */

	/* Called when the gscore application initialy starts up */
	GscoreInitProc init_proc;
	/* Called when the gscore application exits */
	GscoreQuitProc quit_proc;
	/* Called to run a miscellaneous thing everytime the plugin is called */
	GscoreRunProc  run_proc;
};

#define GSCORE_PLUGIN_INIT(plugin)					\
           {								\
                   plugin = g_new0(GscorePlugin, 1);			\
                   plugin->info = g_new0(GscorePluginInfo, 1);		\
                   plugin->filter = g_new0(GscorePluginFilter, 1);	\
	   } 

#define GSCORE_PLUGIN_STRUCT_INIT(plugin)		\
           {						\
	     plugin = g_malloc(sizeof(Score_t));	\
             (plugin)->Identity = g_malloc(sizeof(Identity_t)); \
           }

#ifdef __cplusplus
extern "C" {
#endif

gint gscore_plugins_list(GList *plugins_list);

/**
 * Probes a plugin, retrieving the information on it and adding it to the
 * list of available plugins.
 *
 * @param filename The plugin's filename.
 *
 * @return The plugin handle.
 *
 * @see gscore_plugin_load()
 * @see gscore_plugin_destroy()
 */
GscorePlugin *gscore_plugin_get(const char *filename);

/* /\** */
/*  * Registers a plugin and prepares it for loading. */
/*  * */
/*  * This shouldn't be called by anything but the internal module code. */
/*  * */
/*  * @param plugin The plugin to register. */
/*  *\/ */
/* gboolean gscore_plugin_register(GscorePlugin *plugin); */

/* Get all plugins from DATA_DIR/gscore/plugins and ~/.gscore/plugins */
/* and load them in the plugin_list */
gint gscore_plugins_load(GList **plugins_list);

/* gboolean gscore_plugin_register(GscorePlugin *plugin); */

GList *gscore_plugins_get_list_loaded(void);

GscorePlugin *gscore_plugin_get_from_extension(const gchar *extension);

/* gboolean gscore_plugins_register_all(GList **plugins_list); */

/* gboolean gscore_plugins_probe_and_register_from_dir(const char *dir); */

/* /\** */
/*  * Attempts to load a previously probed plugin. */
/*  * */
/*  * @param plugin The plugin to load. */
/*  * */
/*  * @return @c TRUE if successful, or @c FALSE otherwise. */
/*  * */
/*  * @see gscore_plugin_reload() */
/*  * @see gscore_plugin_unload() */
/*  *\/ */
/* gboolean gscore_plugin_load(GscorePlugin *plugin); */

/* /\** */
/*  * Unloads the specified plugin. */
/*  * */
/*  * @param plugin The plugin handle. */
/*  * */
/*  * @return @c TRUE if successful, or @c FALSE otherwise. */
/*  * */
/*  * @see gscore_plugin_load() */
/*  * @see gscore_plugin_reload() */
/*  *\/ */
/* gboolean gscore_plugin_unload(GscorePlugin *plugin); */

/* /\** */
/*  * Reloads a plugin. */
/*  * */
/*  * @param plugin The old plugin handle. */
/*  * */
/*  * @return @c TRUE if successful, or @c FALSE otherwise. */
/*  * */
/*  * @see gscore_plugin_load() */
/*  * @see gscore_plugin_unload() */
/*  *\/ */
/* gboolean gscore_plugin_reload(GscorePlugin *plugin); */

/* /\** */
/*  * Unloads a plugin and destroys the structure from memory. */
/*  * */
/*  * @param plugin The plugin handle. */
/*  *\/ */
/* void gscore_plugin_destroy(GscorePlugin *plugin); */

/* /\** */
/*  * Returns whether or not a plugin is currently loaded. */
/*  * */
/*  * @param plugin The plugin. */
/*  * */
/*  * @return TRUE if loaded, or FALSE otherwise. */
/*  *\/ */
/* gboolean gscore_plugin_is_loaded(const GscorePlugin *plugin); */

/* /\*@}*\/ */

/* /\**************************************************************************\/ */
/* /\** @name Plugins API                                                     *\/ */
/* /\**************************************************************************\/ */
/* /\*@{*\/ */

/* /\** */
/*  * Sets the search paths for plugins. */
/*  * */
/*  * @param count The number of search paths. */
/*  * @param paths The search paths. */
/*  *\/ */
/* void gscore_plugins_set_search_paths(size_t count, char **paths); */

/* /\** */
/*  * Unloads all loaded plugins. */
/*  *\/ */
/* void gscore_plugins_unload_all(void); */

/* /\** */
/*  * Destroys all registered plugins. */
/*  *\/ */
/* void gscore_plugins_destroy_all(void); */

/* /\** */
/*  * Attempts to load all the plugins in the specified preference key */
/*  * that were loaded when gscore last quit. */
/*  * */
/*  * @param key The preference key containing the list of plugins. */
/*  *\/ */
/* void gscore_plugins_load_saved(const char *key); */

/* /\** */
/*  * Probes for plugins in the registered module paths. */
/*  * */
/*  * @param ext The extension type to probe for, or @c NULL for all. */
/*  * */
/*  * @see gscore_plugin_set_probe_path() */
/*  *\/ */
/* void gscore_plugins_probe(const char *ext); */

/* /\** */
/*  * Returns whether or not plugin support is enabled. */
/*  * */
/*  * @return TRUE if plugin support is enabled, or FALSE otherwise. */
/*  *\/ */
/* gboolean gscore_plugins_enabled(void); */


/* /\** */
/*  * Finds a plugin with the specified name. */
/*  * */
/*  * @param name The plugin name. */
/*  * */
/*  * @return The plugin if found, or @c NULL if not found. */
/*  *\/ */
/* GscorePlugin *gscore_plugins_find_with_name(const char *name); */

/* /\** */
/*  * Finds a plugin with the specified filename (filename with a path). */
/*  * */
/*  * @param filename The plugin filename. */
/*  * */
/*  * @return The plugin if found, or @c NULL if not found. */
/*  *\/ */
/* GscorePlugin *gscore_plugins_find_with_filename(const char *filename); */

/* /\** */
/*  * Finds a plugin with the specified basename (filename without a path). */
/*  * */
/*  * @param basename The plugin basename. */
/*  * */
/*  * @return The plugin if found, or @c NULL if not found. */
/*  *\/ */
/* GscorePlugin *gscore_plugins_find_with_basename(const char *basename); */

/* /\** */
/*  * Finds a plugin with the specified plugin ID. */
/*  * */
/*  * @param id The plugin ID. */
/*  * */
/*  * @return The plugin if found, or @c NULL if not found. */
/*  *\/ */
/* GscorePlugin *gscore_plugins_find_with_id(const char *id); */

/* /\** */
/*  * Returns a list of all loaded plugins. */
/*  * */
/*  * @return A list of all loaded plugins. */
/*  *\/ */
/* GList *gscore_plugins_get_loaded(void); */

/* /\** */
/*  * Returns a list of all plugins, whether loaded or not. */
/*  * */
/*  * @return A list of all plugins. */
/*  *\/ */
/* GList *gscore_plugins_get_all(void); */

/* /\*@}*\/ */

/* #ifdef __cplusplus */
/* } */
/* #endif */


#endif /* _GSCORE_PLUGIN_H */

