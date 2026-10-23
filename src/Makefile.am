SUBDIRS = 

DEFS = -I$(top_srcdir)/ -I$(srcdir)/include/	\
	-DPLUGINS_DIR=\"$(plugins_dir)\"	\
	-DDATA_DIR=\"$(datadir)\"		\
	$(LIBGLADE2_CFLAGS)

bin_PROGRAMS = gscore
gscore_LDFLAGS = -export-dynamic
gscore_LDADD = $(LIBGLADE2_LIBS)

gscore_SOURCES =				\
	articulation.c				\
	beam.c					\
	common.c				\
	chords.c				\
	debug.c					\
	display.c				\
	draw_barline.c				\
	draw.c					\
	edit.c					\
	error.c					\
	file.c					\
	help.c					\
	include					\
	insert.c				\
	key_cursor.c				\
	key_event.c				\
	libglade-garbage.c			\
	main.c					\
	Makefile.am				\
	mode.c					\
	mouse_event.c				\
	notes.c					\
	objects.c				\
	objects_callbacks.c			\
	plugin.c				\
	position.c				\
	realize_area.c				\
	rests.c					\
	score.c					\
	score_callbacks.c			\
	selection.c				\
	spacings.c				\
	splash.c				\
	staff.c					\
	tab_transposition.c			\
	text.c					\
	tie.c					\
	view.c


