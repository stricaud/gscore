/*
 * i18n.js
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The interface in somebody's own language.
 *
 * The key is the English, not an invented name. Five hundred keys of the
 * shape "menu.edit.pitch.up" are five hundred things to keep in step with
 * the words they stand for, and the first one that drifts is a menu entry
 * reading MENU.EDIT.PITCH.UP. With the English as the key there is nothing
 * to keep in step: a string with no translation comes back as itself, so a
 * half-finished language is an interface half in French rather than a page
 * of broken keys.
 *
 * The markup is not marked up. Every visible word is already in index.html,
 * and putting a data-i18n on each of them would be the same five hundred
 * names by another road. Instead the page is walked once when it loads and
 * every piece of text in it is looked up — which also means a word added to
 * a menu is translatable the moment somebody writes the French for it, with
 * nothing else to remember.
 */

// LANGUAGES is what can be chosen, by code, with the name each is called in
// itself. A language is named in its own tongue wherever it is offered, so
// that somebody looking for their own finds it without reading English.
const LANGUAGES = [
  { code: "en", name: "English" },
  { code: "fr", name: "Français" },
];

// MESSAGES is the English of the interface against its translations.
//
// Written as one table per language rather than one entry per string: the
// person writing a translation works down a list in their own language,
// which is how a translation is actually done.
// The French. Written against the English it replaces, in one table, so
// that whoever corrects it reads down a list in their own language.
//
// Anything not here is left in English on purpose: the name of the
// program, the dynamics, the note names, the time signatures, the
// Italian a score is marked up in, and the examples somebody is meant to
// copy. Translating those would be translating the music.
// The French. Written against the English it replaces, in one table, so
// that whoever corrects it reads down a list in their own language.
//
// Anything not here is left in English on purpose: the name of the
// program, the dynamics and the Italian a score is marked up in, the note
// names and the time signatures, and the examples somebody is meant to
// copy. Translating those would be translating the music.
const MESSAGES = {
  fr: {
  ", or drop one here.": ", ou déposez-en un ici.",
  "15ma — two octaves up": "15ma — deux octaves plus haut",
  "15mb — two octaves down": "15mb — deux octaves plus bas",
  "16th": "Double croche",
  "16th rest": "Quart de soupir",
  "32nd": "Triple croche",
  "32nd rest": "Huitième de soupir",
  "44.1 kHz — CD": "44,1 kHz — CD",
  "48 kHz — video": "48 kHz — vidéo",
  "64th": "Quadruple croche",
  "8 (as the originals)": "8 (comme les originaux)",
  "8 px/space": "8 px/interligne",
  "8va — an octave up": "8va — une octave plus haut",
  "8vb — an octave down": "8vb — une octave plus bas",
  "96 kHz": "96 kHz",
  "= 116 becomes (": "= 116 devient (",
  "A": "Un",
  "A dozen words, and JavaScript for the rest.": "Une douzaine de mots, et JavaScript pour le reste.",
  "A musical score editor by Sébastien Tricaud.": "Un éditeur de partitions par Sébastien Tricaud.",
  "A new score.": "Une nouvelle partition.",
  "A note is two facts — how long, and which pitch — and this is both of them in the order a musician says them.": "Une note, ce sont deux faits — sa durée et sa hauteur — et voici les deux dans l’ordre où un musicien les dit.",
  "A score file written by hand can hold a mistyped character, a wrapped line or a construction Gscore does not know. The music around it was read; these are the lines that were passed over, so that the file can be corrected at the source.": "Un fichier de partition écrit à la main peut contenir une faute de frappe, une ligne repliée ou une construction que Gscore ne connaît pas. La musique autour a été lue ; voici les lignes qui ont été passées, pour que le fichier puisse être corrigé à la source.",
  "About Gscore": "À propos de Gscore",
  "Accent": "Accent",
  "Accidental": "Altération",
  "Accidentals": "Altérations",
  "Actual size": "Taille réelle",
  "Add a bar at the end": "Ajouter une mesure à la fin",
  "Add a grand staff": "Ajouter une double portée",
  "Add a staff…": "Ajouter une portée…",
  "Add another notehead to the note just written": "Ajouter une tête de note à la note qui vient d’être écrite",
  "Alto": "Clé d’ut 3e",
  "Analysis": "Analyse",
  "and press the keys you would rather use. Escape while it is waiting leaves the binding alone.": "et appuyez sur les touches que vous préférez. Échap pendant l’attente laisse le raccourci tel quel.",
  "Apply": "Appliquer",
  "Articulation": "Articulation",
  "Articulations": "Articulations",
  "Audio…": "Audio…",
  "Augmentation dot": "Point d’augmentation",
  "Automatic": "Automatique",
  "Automatic (your browser’s)": "Automatique (celle du navigateur)",
  "Automatic stem": "Hampe automatique",
  "Back one note — or move the selection back": "Une note en arrière — ou reculer la sélection",
  "Back to one note": "Revenir à une seule note",
  "Back to the usual size": "Revenir à la taille normale",
  "Barline": "Barre de mesure",
  "Barline here": "Barre de mesure ici",
  "Barlines": "Barres de mesure",
  "Bass": "Clé de fa",
  "Beam": "Ligature",
  "Beam the selected notes together — on one note, break the beam before it": "Ligaturer les notes sélectionnées — sur une seule note, rompre la ligature avant elle",
  "Beam together": "Ligaturer ensemble",
  "Bigger": "Plus grand",
  "Brace — one instrument": "Accolade — un seul instrument",
  "Bracket — a family": "Crochet — une famille",
  "Break the beam": "Rompre la ligature",
  "Bring the level up": "Monter le niveau",
  "Buzz roll": "Roulement",
  "C major or A minor": "Do majeur ou la mineur",
  "Cancel": "Annuler",
  "Change": "Modifier",
  "Choose a file…": "Choisir un fichier…",
  "Choose what to print. Paper size, margins and “Save as PDF” are in your system’s print dialog, which opens next.": "Choisissez quoi imprimer. Le format, les marges et « Enregistrer en PDF » sont dans la fenêtre d’impression du système, qui s’ouvre ensuite.",
  "Chord": "Accord",
  "Chord builder…": "Constructeur d’accords…",
  "Chord symbol": "Chiffrage d’accord",
  "Chord symbol…": "Chiffrage d’accord…",
  "Clear": "Effacer",
  "Clefs": "Clés",
  "Click a pad, or press its letter while the cursor is on a drum staff, to write it": "Cliquez un pad, ou appuyez sur sa lettre pendant que le curseur est sur une portée de batterie, pour l’écrire",
  "Click or drag to draw; hold Alt to erase. The dotted line is the staff line the symbol sits on.": "Cliquez ou glissez pour dessiner ; maintenez Alt pour effacer. La ligne pointillée est la ligne de portée sur laquelle le symbole repose.",
  "Close": "Fermer",
  "Close repeat": "Fin de reprise",
  "Coda — where the music goes on after To Coda": "Coda — où la musique reprend après To Coda",
  "Come back here and open it with the button below. It is kept after that, so this is once.": "Revenez ici et ouvrez-le avec le bouton ci-dessous. Il est conservé ensuite, donc c’est une fois pour toutes.",
  "Common": "Mesure à quatre temps",
  "Continuous strip": "Bande continue",
  "Copy": "Copier",
  "Crescendo": "Crescendo",
  "Crescendo over what is picked up": "Crescendo sur ce qui est sélectionné",
  "Cut": "Couper",
  "D.C. al Coda — back to the beginning, then the coda": "D.C. al Coda — retour au début, puis la coda",
  "D.C. al Fine — back to the beginning, stop at Fine": "D.C. al Fine — retour au début, s’arrêter à Fine",
  "D.S. al Coda — back to the sign, then the coda": "D.S. al Coda — retour au signe, puis la coda",
  "D.S. al Fine — back to the sign, stop at Fine": "D.S. al Fine — retour au signe, s’arrêter à Fine",
  "Da capo — back to the beginning": "Da capo — retour au début",
  "Dal segno — back to the sign": "Dal segno — retour au signe",
  "Delete": "Supprimer",
  "Delete what is picked up": "Supprimer ce qui est sélectionné",
  "Diagram": "Diagramme",
  "Diminuendo": "Diminuendo",
  "Diminuendo over what is picked up": "Diminuendo sur ce qui est sélectionné",
  "Done": "Terminé",
  "Dot what is picked up — again for two dots, three, then none": "Pointer ce qui est sélectionné — encore pour deux points, trois, puis aucun",
  "Dotted": "Pointée",
  "Dotted half": "Blanche pointée",
  "Dotted quarter": "Noire pointée",
  "Double": "Double",
  "Double flat": "Double bémol",
  "Double sharp": "Double dièse",
  "Down": "Vers le bas",
  "Down a step": "Un degré plus bas",
  "Down a step — or move the selected notes down": "Un degré plus bas — ou descendre les notes sélectionnées",
  "Down an octave": "Une octave plus bas",
  "Down bow": "Tiré",
  "Download it yourself": "Le télécharger vous-même",
  "Download one…": "En télécharger une…",
  "Download the file:": "Téléchargez le fichier :",
  "Draw a new one…": "En dessiner un nouveau…",
  "Draw a symbol": "Dessiner un symbole",
  "Drum kit": "Batterie",
  "Drum pads": "Pads de batterie",
  "Duplet — on a selection, those notes become the group": "Duolet — sur une sélection, ces notes forment le groupe",
  "Dynamic": "Nuance",
  "Dynamics": "Nuances",
  "Each note carries a series of partials above it. Where two notes' partials land together they reinforce; where they land close but not together they beat, and that is roughness — measured here in semitones, so 1 is as rough as two notes a semitone apart. The cents say how far equal temperament has bent each interval from the whole-number ratio it is named after.": "Chaque note porte au-dessus d’elle une série de partiels. Là où les partiels de deux notes coïncident, ils se renforcent ; là où ils tombent près sans coïncider, ils battent, et c’est la rugosité — mesurée ici en demi-tons, donc 1 est aussi rugueux que deux notes à un demi-ton. Les cents disent de combien le tempérament égal a écarté chaque intervalle du rapport en nombres entiers dont il porte le nom.",
  "Each one starting on its own page.": "Chacun commençant sur sa propre page.",
  "Edit": "Édition",
  "Editing": "Édition",
  "Eighth": "Croche",
  "Eighth note": "Croche",
  "Eighth rest": "Demi-soupir",
  "End the bar here": "Terminer la mesure ici",
  "Every tune in the file": "Tous les airs du fichier",
  "Export": "Exporter",
  "Export as a web page": "Exporter en page web",
  "Export as audio": "Exporter en audio",
  "Export as PDF": "Exporter en PDF",
  "Export as SVG": "Exporter en SVG",
  "Fermata": "Point d’orgue",
  "File": "Fichier",
  "files": "enregistrés",
  "Fill the screen": "Remplir l’écran",
  "Filled in from the name for the usual chords. One character a string, lowest first: a fret number, or x for a string that is not played. Type your own and it stays yours; clear it to be offered one again.": "Rempli d’après le nom pour les accords usuels. Un caractère par corde, la plus grave d’abord : un numéro de case, ou x pour une corde qui n’est pas jouée. Saisissez le vôtre et il reste le vôtre ; effacez-le pour qu’on vous en propose un à nouveau.",
  "Final": "Finale",
  "Fine — where the piece ends on the way back": "Fine — où le morceau s’arrête au retour",
  "Fingering": "Doigté",
  "First and second time": "Première et deuxième fois",
  "First time": "Première fois",
  "Flat": "Bémol",
  "Flatten — again for a double flat, again for none": "Bémoliser — encore pour un double bémol, encore pour rien",
  "Four slashes": "Quatre barres",
  "From bar": "À partir de la mesure",
  "Full screen": "Plein écran",
  "Full screen (F)": "Plein écran (F)",
  "Go to a bar": "Aller à une mesure",
  "Go to measure…": "Aller à la mesure…",
  "Go to the end of a staff": "Aller à la fin d’une portée",
  "Grid": "Grille",
  "Half": "Blanche",
  "Half note": "Blanche",
  "Half rest": "Demi-pause",
  "Harmonics": "Harmoniques",
  "Harmonics…": "Harmoniques…",
  "Help": "Aide",
  "Hide this staff": "Masquer cette portée",
  "How far each staff reaches, and whether its instrument can go there.": "Jusqu’où va chaque portée, et si son instrument peut s’y rendre.",
  "How low the bottom of the harmony is written.": "Jusqu’où le bas de l’harmonie est écrit grave.",
  "How much of the kit to offer": "Quelle part du kit proposer",
  "How the lines move against each other.": "Comment les lignes se meuvent l’une contre l’autre.",
  "How to type music": "Comment saisir la musique",
  "Import a picture…": "Importer une image…",
  "Import from a .gscore…": "Importer depuis un .gscore…",
  "In brackets": "Entre parenthèses",
  "In parentheses": "Entre parenthèses",
  "Insert": "Insérer",
  "Insert a symbol": "Insérer un symbole",
  "Insert a symbol…": "Insérer un symbole…",
  "Instruments": "Instruments",
  "Instruments in this score": "Instruments de cette partition",
  "is a container. With this on it holds a PDF of the score as well as the music, so the same file renamed": "est un conteneur. Avec ceci activé, il contient un PDF de la partition en plus de la musique, si bien que le même fichier renommé",
  "Join staves": "Réunir les portées",
  "Keep bars to the meter": "Garder les mesures à la mesure",
  "Keep with the score": "Garder avec la partition",
  "Key signature, flats to the left and sharps to the right": "Armature, bémols à gauche et dièses à droite",
  "Key signature…": "Armature…",
  "Keyboard shortcuts": "Raccourcis clavier",
  "Keys and modulations": "Tonalités et modulations",
  "Keys and modulations…": "Tonalités et modulations…",
  "Kit": "Kit",
  "Language": "Langue",
  "Line": "Lignes",
  "Lines and endings": "Lignes et fins de reprise",
  "Lines that could not be read": "Lignes qui n’ont pas pu être lues",
  "Lyrics": "Paroles",
  "Lyrics…": "Paroles…",
  "Marcato": "Marcato",
  "Marcato down": "Marcato vers le bas",
  "Marcato up": "Marcato vers le haut",
  "Marks": "Signes",
  "Measure": "Mesure",
  "Measure numbers": "Numéros de mesure",
  "Melodic shape": "Profil mélodique",
  "Melodic shape…": "Profil mélodique…",
  "MIT licence. The music font is Bravura, under the SIL Open Font License.": "Licence MIT. La police musicale est Bravura, sous licence SIL Open Font.",
  "Move staff down": "Descendre la portée",
  "Move staff up": "Monter la portée",
  "Move the note down a step": "Descendre la note d’un degré",
  "Move the note up a step": "Monter la note d’un degré",
  "Move the notes themselves earlier": "Déplacer les notes elles-mêmes plus tôt",
  "Move the notes themselves later": "Déplacer les notes elles-mêmes plus tard",
  "MusicXML, compressed…": "MusicXML, compressé…",
  "Name": "Nom",
  "name it": "nommez-le",
  "Natural": "Bécarre",
  "Natural — or tie, with nothing picked up": "Bécarre — ou liaison, si rien n’est sélectionné",
  "New score": "Nouvelle partition",
  "New score…": "Nouvelle partition…",
  "No direction here": "Aucun renvoi ici",
  "No dynamic": "Aucune nuance",
  "No ending": "Aucune fin numérotée",
  "No hairpin": "Aucun soufflet",
  "No octave line": "Aucune ligne d’octave",
  "No sign here": "Aucun signe ici",
  "No slur": "Aucune liaison",
  "No tremolo": "Aucun trémolo",
  "No tuplet — the group back to one note": "Aucun n-olet — le groupe redevient des notes ordinaires",
  "None": "Aucune",
  "Nonuplet": "Nonolet",
  "Nonuplet — on a selection, those notes become the group": "Nonolet — sur une sélection, ces notes forment le groupe",
  "Note": "Note",
  "Note lengths": "Valeurs de note",
  "Nothing": "Rien",
  "Nothing drawn yet. Use Insert → Draw a symbol… first.": "Rien de dessiné pour l’instant. Utilisez Insérer → Dessiner un symbole… d’abord.",
  "nowhere yet": "nulle part encore",
  "Number the pages": "Numéroter les pages",
  "Octuplet": "Octolet",
  "Octuplet — on a selection, those notes become the group": "Octolet — sur une sélection, ces notes forment le groupe",
  "Off": "Désactivé",
  "OK": "OK",
  "On one note — or move the selection on": "Une note en avant — ou avancer la sélection",
  "One slash": "Une barre",
  "Open": "Ouvrir",
  "Open an ABC file from the File menu, press": "Ouvrez un fichier ABC depuis le menu Fichier, appuyez sur",
  "Open recent": "Documents récents",
  "Open repeat": "Début de reprise",
  "Open score…": "Ouvrir une partition…",
  "Open the file…": "Ouvrir le fichier…",
  "Opening…": "Ouverture…",
  "opens in anything that reads one — and anybody you send it to can read the score without gscore. With it off the file holds the music alone, and saving is quicker on a long piece.": "s’ouvre dans tout ce qui en lit un — et quiconque le reçoit peut lire la partition sans gscore. Désactivé, le fichier ne contient que la musique, et l’enregistrement est plus rapide sur un long morceau.",
  "Optional text expression": "Indication en toutes lettres (facultative)",
  "Options": "Options",
  "Originally written in C and GTK+ between 2001 and 2005, the last release being 0.0.9 “harder, better, stronger, faster” in April 2005. Rewritten in Go in 2026, engraved with SMuFL and Bravura.": "Écrit à l’origine en C et GTK+ entre 2001 et 2005, la dernière version étant la 0.0.9 « harder, better, stronger, faster » en avril 2005. Réécrit en Go en 2026, gravé avec SMuFL et Bravura.",
  "Other number…": "Autre nombre…",
  "Page Setup": "Mise en page",
  "Paginated": "Paginé",
  "Parameters…": "Paramètres…",
  "Paste": "Coller",
  "Percussion": "Percussion",
  "Pick a note, or play, and this follows it.": "Choisissez une note, ou lancez la lecture, et ceci la suit.",
  "Pick the one you want to see on the staff.": "Choisissez celui que vous voulez voir sur la portée.",
  "Pick the signature you want to see on the staff.": "Choisissez l’armature que vous voulez voir sur la portée.",
  "Pixels per space": "Pixels par interligne",
  "Play": "Lecture",
  "Play / Pause": "Lecture / Pause",
  "Play / Pause (Shift+Space)": "Lecture / Pause (Maj+Espace)",
  "Play from the beginning": "Lire depuis le début",
  "Play, pause, and carry on from where it paused": "Lire, mettre en pause, et repartir d’où l’on s’est arrêté",
  "Playback sound…": "Son de lecture…",
  "Played through a soundfont — recorded instruments — and written as FLAC, which is lossless.": "Joué au travers d’une banque de sons — des instruments enregistrés — et écrit en FLAC, qui est sans perte.",
  "Playing": "Lecture",
  "Print": "Imprimer",
  "Print Preview…": "Aperçu avant impression…",
  "Print…": "Imprimer…",
  "Quadruplet": "Quartolet",
  "Quadruplet — on a selection, those notes become the group": "Quartolet — sur une sélection, ces notes forment le groupe",
  "Quarter": "Noire",
  "Quarter note": "Noire",
  "Quarter rest": "Soupir",
  "Quintuplet": "Quintolet",
  "Quintuplet — on a selection, those notes become the group": "Quintolet — sur une sélection, ces notes forment le groupe",
  "Range": "Tessiture",
  "Range…": "Tessiture…",
  "Read from the notes, not from the key signature.": "Lu dans les notes, non dans l’armature.",
  "Ready": "Prêt",
  "Redo": "Rétablir",
  "Rehearsal mark…": "Repère de répétition…",
  "Remove": "Retirer",
  "Remove the slur": "Retirer la liaison de phrasé",
  "Remove the tie": "Retirer la liaison",
  "Remove this change": "Supprimer ce changement",
  "Remove this staff": "Supprimer cette portée",
  "Repeat ending": "Fin de reprise numérotée",
  "Repeat the bar before": "Répéter la mesure précédente",
  "Repeat the two bars before": "Répéter les deux mesures précédentes",
  "Repeats and jumps": "Reprises et renvois",
  "Report a bug": "Signaler une anomalie",
  "reset": "réinitialiser",
  "Reset all": "Tout réinitialiser",
  "Rest": "Silence",
  "Rests": "Silences",
  "Reverb and chorus": "Réverbération et chorus",
  "Rhythm and metre": "Rythme et métrique",
  "Rhythm and metre…": "Rythme et métrique…",
  "Room at the end": "Résonance à la fin",
  "Run": "Exécuter",
  "Sample rate": "Fréquence d’échantillonnage",
  "Save": "Enregistrer",
  "Save as…": "Enregistrer sous…",
  "Save symbol": "Enregistrer le symbole",
  "Score": "Partition",
  "Score and parts": "Conducteur et parties",
  "Score options…": "Options de partition…",
  "Script console": "Console de script",
  "Script console…": "Console de script…",
  "Second time": "Deuxième fois",
  "seconds, for the last chord to ring": "secondes, pour laisser sonner le dernier accord",
  "Segno — the sign a D.S. goes back to": "Segno — le signe auquel un D.S. renvoie",
  "Selecting and moving": "Sélection et déplacement",
  "Septuplet": "Septolet",
  "Septuplet — on a selection, those notes become the group": "Septolet — sur une sélection, ces notes forment le groupe",
  "Set instrument…": "Choisir l’instrument…",
  "Set key signature": "Définir l’armature",
  "Set Tempo": "Définir le tempo",
  "Set tempo…": "Définir le tempo…",
  "Settings": "Réglages",
  "Settings…": "Réglages…",
  "Sextuplet": "Sextolet",
  "Sextuplet — on a selection, those notes become the group": "Sextolet — sur une sélection, ces notes forment le groupe",
  "Shape of the piece": "Forme du morceau",
  "Shape of the piece…": "Forme du morceau…",
  "Share mine as a .gscore…": "Partager les miens en .gscore…",
  "Sharp": "Dièse",
  "Sharpen — again for a double sharp, again for none": "Diéser — encore pour un double dièse, encore pour rien",
  "Show all at once": "Tout afficher à la fois",
  "Show every staff": "Afficher toutes les portées",
  "Show every tune": "Afficher tous les airs",
  "Show fingering": "Afficher le doigté",
  "Show the fingering": "Afficher le doigté",
  "Single": "Simple",
  "Sixteenth note": "Double croche",
  "Slur": "Liaison de phrasé",
  "Slur and tie": "Liaisons",
  "Slur the passage": "Lier le passage",
  "Slur — the same as S": "Liaison de phrasé — comme S",
  "Smaller": "Plus petit",
  "Soundfont": "Banque de sons",
  "Spacing": "Espacement",
  "Spacing…": "Espacement…",
  "Staccatissimo": "Staccatissimo",
  "Staccato": "Staccato",
  "Staff": "Portée",
  "Staff names": "Noms des portées",
  "Staves": "Portées",
  "Stem": "Hampe",
  "Stem down": "Hampe vers le bas",
  "Stem up": "Hampe vers le haut",
  "Stems": "Hampes",
  "Stop": "Arrêter",
  "Stop playing, or let go of the selection": "Arrêter la lecture, ou lâcher la sélection",
  "Stop, back to the beginning (Esc)": "Arrêter, retour au début (Échap)",
  "Symbol": "Symbole",
  "Tab": "Tablature",
  "Tab transposition…": "Transposition de tablature…",
  "Take the selection one note further back": "Étendre la sélection d’une note en arrière",
  "Take the selection one note further on": "Étendre la sélection d’une note en avant",
  "Taken by": "Pris par",
  "Tenor": "Clé d’ut 4e",
  "Tenuto": "Tenuto",
  "Text…": "Texte…",
  "The file": "Le fichier",
  "The full score, then each staff alone, for the players.": "Le conducteur entier, puis chaque portée seule, pour les musiciens.",
  "The language the menus and panels are written in.": "La langue dans laquelle les menus et les panneaux sont écrits.",
  "The loudest moment just under full scale, so it is as loud as anything else you would play.": "Le moment le plus fort juste sous le maximum, pour que ce soit aussi fort que tout ce que vous écouteriez par ailleurs.",
  "The room the soundfont was voiced to be heard in.": "La salle pour laquelle la banque de sons a été réglée.",
  "The script": "Le script",
  "The speed of the piece.": "La vitesse du morceau.",
  "The window": "La fenêtre",
  "Third time": "Troisième fois",
  "Thirty-second note": "Triple croche",
  "This tune": "Cet air",
  "Three slashes": "Trois barres",
  "Tie": "Liaison de prolongation",
  "Tie the notes": "Lier les notes",
  "Tie this notehead only": "Lier cette tête de note seule",
  "Tie — the same as T": "Liaison de prolongation — comme T",
  "Time signature": "Chiffrage de mesure",
  "Time signature from here…": "Chiffrage de mesure à partir d’ici…",
  "Time signatures": "Chiffrages de mesure",
  "Title and composer…": "Titre et compositeur…",
  "To Coda — leave here on the way back": "To Coda — quitter ici au retour",
  "Tools": "Outils",
  "Treble": "Clé de sol",
  "Tremolo": "Trémolo",
  "Triplet": "Triolet",
  "Triplet — on a selection, those notes become the group": "Triolet — sur une sélection, ces notes forment le groupe",
  "Tunes": "Airs",
  "Tuplet": "N-olet",
  "Tuplets": "N-olets",
  "Turn the paper on its side": "Mettre la feuille en paysage",
  "Two notes — one beam": "Deux notes — une barre",
  "Two notes — three beams": "Deux notes — trois barres",
  "Two notes — two beams": "Deux notes — deux barres",
  "Two slashes": "Deux barres",
  "Type a meter…": "Saisir une mesure…",
  "Type music here": "Saisissez la musique ici",
  "Type the music": "Saisir la musique",
  "Type the music in, at the cursor": "Saisir la musique, au curseur",
  "Type the verse as you would write it. The words are handed out to the notes in order, one syllable each. Break a word across notes with hyphens —": "Saisissez le couplet comme vous l’écririez. Les mots sont distribués aux notes dans l’ordre, une syllabe chacune. Coupez un mot entre les notes par des traits d’union —",
  "Type to search…": "Saisir pour rechercher…",
  "Undo": "Annuler",
  "Up": "Vers le haut",
  "Up a step": "Un degré plus haut",
  "Up a step — or move the selected notes up": "Un degré plus haut — ou monter les notes sélectionnées",
  "Up an octave": "Une octave plus haut",
  "Up bow": "Poussé",
  "Value": "Valeur",
  "Verse": "Couplet",
  "View": "Affichage",
  "Voice leading": "Conduite des voix",
  "Voice leading…": "Conduite des voix…",
  "Web page…": "Page web…",
  "What each line is made of.": "De quoi chaque ligne est faite.",
  "What each staff is scored for, and what the soundfont will play it with.": "Pour quoi chaque portée est écrite, et ce avec quoi la banque de sons la jouera.",
  "What is on screen now.": "Ce qui est à l’écran en ce moment.",
  "What to call this script": "Comment nommer ce script",
  "Where the notes fall against the beat.": "Où les notes tombent par rapport au temps.",
  "Who is playing when, and what repeats what.": "Qui joue quand, et ce qui répète quoi.",
  "Whole": "Ronde",
  "Whole note": "Ronde",
  "Whole rest": "Pause",
  "Wider lines, and fewer staves down the page — a score too tall for the sheet is divided between staves and continued overleaf. Parts are always printed upright.": "Des lignes plus larges, et moins de portées sur la page — un conducteur trop haut pour la feuille est divisé entre les portées et continué au verso. Les parties sont toujours imprimées en portrait.",
  "Write a rest of the same length": "Écrire un silence de même valeur",
  "Write a word over the music": "Écrire un mot au-dessus de la musique",
  "Write with the mouse, rather than pointing with it": "Écrire à la souris, plutôt que de pointer avec elle",
  "Write it": "L’écrire",
  "Write the bar out": "Écrire la mesure en toutes notes",
  "Write the note the palette is showing": "Écrire la note affichée dans la palette",
  "Write with the mouse": "Écrire à la souris",
  "Writing": "Écriture",
  "Your symbols": "Vos symboles",
  "Zoom": "Zoom",
  "Zoom in": "Agrandir",
  "Zoom out": "Réduire",
  "“2/5” along the foot. Counted within each part, so a player can tell they have all of theirs.": "« 2/5 » en bas de page. Compté dans chaque partie, pour qu’un musicien voie qu’il a bien toute la sienne.",
  },
};

// lang is the language in force. "en" is not a translation but the absence
// of one, so it has no table.
let lang = "en";

// t is one string in the language in force.
//
// Anything with no translation comes back as it went in. That is the whole
// fallback: an interface in two languages at once is ugly and perfectly
// usable, and it is what lets a translation be written a hundred words at a
// time instead of all at once.
function t(s) {
  if (typeof s !== "string" || !s) return s;
  const table = MESSAGES[lang];
  if (!table) return s;
  const got = table[tidy(s)];
  return got === undefined ? s : got;
}

// tidy is a string as the dictionary holds it: one space wherever the markup
// has a run of them.
//
// A sentence in HTML is written across several lines and indented to suit
// the markup, so the text the page actually holds is full of newlines and
// leading spaces that nobody put there on purpose. Keyed as it comes, a
// paragraph would have to be translated once for every way it might be
// wrapped — and re-translated the first time anyone re-indented the file.
function tidy(s) {
  return String(s).replace(/\s+/g, " ").trim();
}

// currentLanguage is the code in force, and languageChoice what was asked
// for — "auto" where nobody has asked for anything and the browser decides.
function currentLanguage() { return lang; }
let chosenLanguage = "auto";
function languageChoice() { return chosenLanguage; }

// browserLanguage is the language the browser asks for, where it is one
// gscore has.
//
// The first of the list it understands, because navigator.languages is in
// the order somebody put their languages in and the first match is the one
// they would rather read. "fr-CA" is French: the region says which French
// and gscore has only the one.
function browserLanguage(nav) {
  const list = (nav && (nav.languages || (nav.language ? [nav.language] : []))) || [];
  for (const want of list) {
    const code = String(want).toLowerCase().split("-")[0];
    if (LANGUAGES.some((l) => l.code === code)) return code;
  }
  return "en";
}

// setLanguage puts a language in force. "auto" means the browser's.
//
// It returns the code that ended up in force, which is not always the one
// asked for: a language gscore does not have falls back to English rather
// than leaving the interface in whatever was there before.
function setLanguage(choice, nav) {
  chosenLanguage = choice || "auto";
  const want = chosenLanguage === "auto"
    ? browserLanguage(nav || (typeof navigator === "undefined" ? null : navigator))
    : String(choice).toLowerCase().split("-")[0];
  lang = LANGUAGES.some((l) => l.code === want) ? want : "en";
  return lang;
}

// SKIP are the elements whose text is not prose.
//
// A keystroke is a keystroke in every language — ⌘Z is ⌘Z — and the score
// itself is music, which is the one thing that needs no translating at all.
const SKIP = new Set(["SCRIPT", "STYLE", "KBD", "SVG", "TEXTAREA", "CODE"]);

// TRANSLATED_ATTRS are the attributes a reader sees.
const TRANSLATED_ATTRS = ["placeholder", "title", "aria-label", "label"];

// translateDOM puts the page into the language in force.
//
// Every text node under the root, and the attributes a reader sees. It is
// run once when the page loads and again when the language is changed; the
// original English is kept on the node the first time, so changing language
// twice does not translate a translation.
function translateDOM(root, doc) {
  const d = doc || (typeof document === "undefined" ? null : document);
  if (!root || !d) return;

  const walk = (node) => {
    if (node.nodeType === 3) { // a piece of text
      const was = node.gscoreEnglish !== undefined
        ? node.gscoreEnglish : node.nodeValue;
      const key = tidy(was);
      if (!key) return;
      node.gscoreEnglish = was;
      const said = t(key);
      if (said === key) {
        node.nodeValue = was; // nothing to say; leave the spacing alone
        return;
      }
      // The space around the words is the page's layout and stays as it is;
      // only what is between it is replaced.
      const lead = String(was).match(/^\s*/)[0];
      const tail = String(was).match(/\s*$/)[0];
      node.nodeValue = lead + said + tail;
      return;
    }
    if (node.nodeType !== 1) return;
    // Upper-cased: an HTML element's tagName comes back shouting and an
    // SVG one does not, and the score is drawn in SVG. Matched as it
    // comes, every notehead on the page was looked up in the dictionary.
    if (SKIP.has(String(node.tagName).toUpperCase()) ||
        node.hasAttribute("data-no-i18n")) {
      return;
    }
    for (const name of TRANSLATED_ATTRS) {
      if (!node.hasAttribute(name)) continue;
      const key = "gscoreEnglish:" + name;
      const was = node[key] !== undefined ? node[key] : node.getAttribute(name);
      node[key] = was;
      node.setAttribute(name, t(was));
    }
    for (const child of Array.from(node.childNodes)) walk(child);
  };
  walk(root);
}
