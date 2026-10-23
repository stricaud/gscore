AC_INIT([Gscore], [0.0.9],
        [Sebastien Tricaud <toady@gscore.org>],
        [gscore])
AM_INIT_AUTOMAKE

dnl Libraries requirements
GLIB2_VERSION=2.6.0
GTK2_VERSION=2.6.0
LIBGLADE2_VERSION=2.4.0

AC_SUBST(LIBGLADE2_VERSION)
AC_SUBST(GLIB2_VERSION)
AC_SUBST(GTK2_VERSION)


AM_MAINTAINER_MODE

dnl Put the preprocessor macros in a header instead of command line 
AM_CONFIG_HEADER(config.h)

dnl Checks for headers
AC_HEADER_STDC
AC_HEADER_DIRENT

dnl Checks for typedefs, structures, and compiler characteristics.
AC_C_CONST
AC_TYPE_PID_T
AC_TYPE_SIZE_T
AC_HEADER_TIME

dnl Checks for library functions.
AC_TYPE_SIGNAL
AC_FUNC_VPRINTF
AC_CHECK_FUNCS(gettimeofday select socket strdup strerror strstr)
7
dnl Checks for programs
AM_PROG_LIBTOOL

if test "x$prefix" = "xNONE"; then
	prefix="/usr/local"
fi

if test "x$localstatedir" = "x\${prefix}/var"; then
	localstatedir="$prefix/var"
fi

if test "x$sysconfdir" = "x\${prefix}/etc"; then
    sysconfdir="$prefix/etc"
fi

AC_SUBST(localstatedir)

plugins_dir=$libdir/gscore

AC_SUBST(plugins_dir)

dnl **************************************************
dnl * Check for Gtkdoc.                              *
dnl **************************************************
AC_CHECK_PROG(GTKDOC, gtkdoc-mkdb, true, false)
AM_CONDITIONAL(HAVE_GTK_DOC, test -n $GTKDOC)
AC_SUBST(HAVE_GTK_DOC)

AC_ARG_ENABLE(gtk-doc, [  --enable-gtk-doc  Use gtk-doc to build documentation [default=no]], enable_gtk_doc="$enableval", enable_gtk_doc=no)
AM_CONDITIONAL(ENABLE_GTK_DOC, test x$enable_gtk_doc = xyes)

dnl ********************************************************
dnl languages
dnl ********************************************************
ALL_LINGUAS="cs de en_CA nl pt_BR sv zh_CN"

dnl ********************************************************
dnl triggers gettext
dnl ********************************************************
AM_GLIB_GNU_GETTEXT
GETTEXT_PACKAGE=gscore
AC_SUBST(GETTEXT_PACKAGE)
AC_DEFINE_UNQUOTED(GETTEXT_PACKAGE,"$GETTEXT_PACKAGE",[fix])

dnl *******************************
dnl checking for glib
dnl *******************************
PKG_CHECK_MODULES(GLIB2,[glib-2.0 >= $GLIB2_VERSION])

dnl *******************************
dnl checking for gtk+
dnl *******************************
PKG_CHECK_MODULES(GKT2,[gtk+-2.0 >= $GTK2_VERSION])

dnl *******************************
dnl checking for libglade
dnl *******************************
PKG_CHECK_MODULES(LIBGLADE2,[libglade-2.0 >= $LIBGLADE2_VERSION])

dnl *******************************
dnl checking for libabcp
dnl *******************************
AC_CHECK_LIB(abcp, abcScanFile, [], [
        echo "*****************************************************"
        echo "Error! You need to have libabcp installed."
	echo "You can download it from http://abcp.sourceforge.net/"
        echo "*****************************************************"
        exit -1
        ])

AC_PROG_INSTALL

dnl CFLAGS="$CFLAGS -Wall -Wstrict-prototypes -Wmissing-declarations \
dnl -Wbad-function-cast -Wcast-qual -Wcast-align -Wnested-externs -Wunused"

CFLAGS="$CFLAGS -Wall"

AC_SUBST(LDFLAGS)
AC_SUBST(CFLAGS)
AC_SUBST(LIBGLADE2_CFLAGS)
AC_SUBST(LIBGLADE2_LIBS)

AC_OUTPUT([

Makefile

plugins/Makefile
plugins/abc/Makefile

pixmaps/Makefile

src/Makefile

glade/Makefile

po/Makefile.in

])


echo
echo "*** Dumping configuration ***"
echo "    - Compiler               : ${CC}"
echo "    - Generate documentation : $enable_gtk_doc"
echo "    - Prefix                 : $prefix"
echo "    - Exec prefix            : $exec_prefix"
echo "    - Data dir               : $datadir"
echo "    - Plugins dir            : $plugins_dir"
echo
echo "    You can now run 'make' to compile gscore"
echo
