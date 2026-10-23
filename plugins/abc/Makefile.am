DEFS = -I$(top_srcdir) -I$(top_srcdir)/src/include $(LIBGLADE2_CFLAGS)

abc_la_SOURCES = abc.c export-abc.c import-abc.c
abc_la_LDFLAGS = -module -avoid-version $(LIBGLADE2_LFLAGS) -labcp
abcdir = $(plugins_dir)
abc_LTLIBRARIES = abc.la
