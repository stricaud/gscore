; gscore.nsi - the Windows installer.
;
; A zip of an .exe is not an installation: nothing puts it on the Start
; menu, nothing tells Windows how to open a .gscore file, and nothing can
; remove it again. This builds the real thing - Program Files, a shortcut,
; an entry in Installed apps, an uninstaller - and it carries the soundfont
; with it, so the program can play a note the moment it opens with nothing
; to fetch and nothing to choose.
;
; Built by `make windows`, cross-compiled from any machine with NSIS:
;
;   makensis -DVERSION=0.1.1 -DVERSION_DOTS=0.1.1.0 \
;            -DSRCDIR=build/windows -DICON=assets/icon/gscore.ico \
;            -DOUTFILE=build/gscore-0.1.1-windows-setup.exe \
;            packaging/windows/gscore.nsi
;
; (C) Copyright 2026 Sebastien Tricaud. MIT licence; see LICENSE.

Unicode true

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!ifndef VERSION
	!error "give -DVERSION: it comes from the VERSION file, via the Makefile"
!endif
!ifndef VERSION_DOTS
	!error "give -DVERSION_DOTS: four numbers, as in 0.1.1.0"
!endif
!ifndef SRCDIR
	!error "give -DSRCDIR: the staged folder `make windows` built"
!endif
!ifndef ICON
	!error "give -DICON: the .ico for the installer and the shortcuts"
!endif
!ifndef OUTFILE
	!define OUTFILE "gscore-setup.exe"
!endif

!define UNINST_KEY \
	"Software\Microsoft\Windows\CurrentVersion\Uninstall\Gscore"

Name "Gscore ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$PROGRAMFILES64\Gscore"
; An upgrade goes where the last one went rather than back to the default,
; so somebody who chose another disk is not installed twice.
InstallDirRegKey HKLM "Software\Gscore" "InstallDir"
RequestExecutionLevel admin
; Solid LZMA, because most of the download is one soundfont and the
; dictionary is worth more than the time.
SetCompressor /SOLID lzma

VIProductVersion "${VERSION_DOTS}"
VIAddVersionKey "ProductName"     "Gscore"
VIAddVersionKey "ProductVersion"  "${VERSION}"
VIAddVersionKey "FileVersion"     "${VERSION}"
VIAddVersionKey "FileDescription" "Gscore installer"
VIAddVersionKey "CompanyName"     "Sebastien Tricaud"
VIAddVersionKey "LegalCopyright"  "(C) 2001-2026 Sebastien Tricaud. MIT licence."

!define MUI_ICON   "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\gscore.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Open Gscore"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${SRCDIR}/LICENSE.txt"
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; The program is built for amd64 only, so say so here rather than let
; somebody install a file their machine cannot run.
Function .onInit
	${IfNot} ${RunningX64}
		MessageBox MB_OK|MB_ICONSTOP \
			"Gscore is 64-bit, and this is a 32-bit Windows."
		Abort
	${EndIf}
	SetRegView 64
FunctionEnd

Function un.onInit
	SetRegView 64
FunctionEnd

Section "Gscore" SecMain
	SectionIn RO

	SetOutPath "$INSTDIR"
	File "${SRCDIR}/gscore.exe"
	File "${SRCDIR}/LICENSE.txt"
	; Kept beside the program so the shortcuts and the file association have
	; an icon to point at without reaching inside the .exe.
	File "/oname=gscore.ico" "${ICON}"

	; Where gscore looks for instruments: beside its own executable, so a
	; folder that is copied elsewhere takes them along.
	SetOutPath "$INSTDIR\soundfonts"
	File "${SRCDIR}/soundfonts/*.sf2"

	SetOutPath "$INSTDIR"
	CreateShortCut "$SMPROGRAMS\Gscore.lnk" "$INSTDIR\gscore.exe" "" \
		"$INSTDIR\gscore.ico"

	WriteRegStr HKLM "Software\Gscore" "InstallDir" "$INSTDIR"
	WriteUninstaller "$INSTDIR\uninstall.exe"

	; Installed apps, so this can be removed the way everything else is.
	WriteRegStr HKLM "${UNINST_KEY}" "DisplayName"     "Gscore ${VERSION}"
	WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion"  "${VERSION}"
	WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon"     "$INSTDIR\gscore.ico"
	WriteRegStr HKLM "${UNINST_KEY}" "Publisher"       "Sebastien Tricaud"
	WriteRegStr HKLM "${UNINST_KEY}" "URLInfoAbout" \
		"https://github.com/stricaud/gscore"
	WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
	WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" \
		'"$INSTDIR\uninstall.exe"'
	WriteRegStr HKLM "${UNINST_KEY}" "QuietUninstallString" \
		'"$INSTDIR\uninstall.exe" /S'
	WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
	WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1
	; In kilobytes, which is what the Settings app prints.
	${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
	IntFmt $0 "0x%08X" $0
	WriteRegDWORD HKLM "${UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

Section "Open .gscore files with Gscore" SecAssoc
	WriteRegStr HKCR ".gscore" "" "Gscore.Score"
	WriteRegStr HKCR "Gscore.Score" "" "Gscore score"
	WriteRegStr HKCR "Gscore.Score\DefaultIcon" "" "$INSTDIR\gscore.ico"
	WriteRegStr HKCR "Gscore.Score\shell\open\command" "" \
		'"$INSTDIR\gscore.exe" "%1"'
	; Tell the shell to re-read the associations, or the new icon does not
	; appear until the next time somebody signs in.
	System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, i 0, i 0)'
SectionEnd

Section "Desktop shortcut" SecDesktop
	CreateShortCut "$DESKTOP\Gscore.lnk" "$INSTDIR\gscore.exe" "" \
		"$INSTDIR\gscore.ico"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
	!insertmacro MUI_DESCRIPTION_TEXT ${SecMain} \
		"Gscore, and a General MIDI soundfont to play scores through."
	!insertmacro MUI_DESCRIPTION_TEXT ${SecAssoc} \
		"Double-clicking a .gscore file opens it in Gscore."
	!insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} \
		"A shortcut on the desktop."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
	Delete "$INSTDIR\gscore.exe"
	Delete "$INSTDIR\LICENSE.txt"
	Delete "$INSTDIR\gscore.ico"
	Delete "$INSTDIR\uninstall.exe"
	RMDir /r "$INSTDIR\soundfonts"
	; Not /r on the folder itself: anything else in there is somebody's, and
	; an uninstaller that empties a directory it does not own is how scores
	; get deleted.
	RMDir "$INSTDIR"

	Delete "$SMPROGRAMS\Gscore.lnk"
	Delete "$DESKTOP\Gscore.lnk"

	DeleteRegKey HKLM "${UNINST_KEY}"
	DeleteRegKey HKLM "Software\Gscore"

	; Only if .gscore is still ours: another program may have taken it since.
	ReadRegStr $0 HKCR ".gscore" ""
	${If} $0 == "Gscore.Score"
		DeleteRegKey HKCR ".gscore"
	${EndIf}
	DeleteRegKey HKCR "Gscore.Score"
	System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, i 0, i 0)'
SectionEnd
