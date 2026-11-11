/*
 * soundfont.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

// Where the recorded instruments come from.
//
// A soundfont is a big file of somebody else's recordings and gscore does not
// carry one: the program is a few megabytes and a good orchestra is thirty,
// which is a strange thing to make every user download in order to write a
// tune they will play themselves. So it is fetched once, when somebody asks
// for a recording, and kept.
//
// Three ways in, because people arrive with different things already true of
// them: a file they already have, a link they were given, or nothing at all
// and a list to choose from.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/synth"
)

// source is a soundfont gscore knows how to go and get.
type source struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	About   string `json:"about"`
	License string `json:"license"`
	// Browser says a page may fetch it.
	//
	// Not a matter of the file but of the host: a browser will not hand a
	// page bytes from a site that has not said other sites may read it, and
	// most sites have never thought about it. gscore running on a machine
	// has no such rule and can fetch any of them. Offering a download in
	// the page that is certain to be refused is worse than not offering it.
	Browser bool `json:"browser,omitempty"`
}

// catalogue is the list offered to somebody who has no soundfont at all.
//
// Short on purpose. The point is that one click produces a good orchestra,
// not that gscore becomes a directory of soundfonts: anything else can be
// given by its link or picked off the disk, and both of those paths are the
// same code as this one.
//
// It is also short because the field is. A soundfont offered here has to be
// a complete General MIDI set, licensed so that anybody can use it for
// anything, and published somewhere that will still answer next year —
// which between them rule out nearly everything. The hundreds of others are
// a click away in the collection, where their provenance is their own
// business rather than gscore's recommendation.
//
// Lightest first, because that is the choice being made: all three are good
// and the question is how long you are willing to wait. Only the first is
// shipped with the application — see BUNDLE_FONTS in the Makefile — and it
// is the one that is small enough to be worth carrying.
var catalogue = []source{{
	ID:   "generaluser-gs",
	Name: "GeneralUser GS",
	URL:  "https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/main/GeneralUser-GS.sf2",
	Size: 32319396,
	About: "A complete General MIDI set in 32 MB, by S. Christian Collins. " +
		"The usual recommendation for a first soundfont: every program is " +
		"there and none of them is embarrassing.",
	License: "Free to use for any purpose, including commercial work.",
	Browser: true,
}, {
	ID:   "fluidr3-gm",
	Name: "FluidR3 GM",
	URL:  zanderjazBase + "packs/FluidR3%20GM.SF2",
	Size: 148358590,
	About: "The bank FluidSynth ships with, by Frank Wen: 148 MB of General " +
		"MIDI, bigger and fuller than GeneralUser GS and the reason a Linux " +
		"machine playing a MIDI file sounds the way it does.",
	License: "MIT.",
}, {
	ID:   "musescore-general",
	Name: "MuseScore General",
	URL: "https://ftp.osuosl.org/pub/musescore/soundfont/MuseScore_General/" +
		"MuseScore_General.sf2",
	Size: 215614036,
	About: "The bank MuseScore plays through: FluidR3 rebuilt and extended " +
		"by S. Christian Collins over twenty years, and the fullest of " +
		"these three. All 128 programs have a sound of their own and none " +
		"of them is a stand-in for another.",
	// MIT, and the notice travels with the file. Nothing is redistributed
	// here — this is a link to the mirror the MuseScore project publishes
	// it on, where the licence sits in the folder beside it.
	License: "MIT, by Frank Wen, Michael Cowgill and S. Christian Collins.",
	// The mirror sends no header letting a page read it, so in a browser
	// this is one to fetch yourself. Running on a machine, gscore fetches it.
}}

// Where downloaded soundfonts are kept: soundfontDir, and the folder they
// used to be kept in, which is still read. Both are in soundfontdisk.go,
// because both look at a filesystem and the browser build has none — see
// soundfontnodisk.go for what happens there instead.

// bundledSoundfontDirs are the ones shipped with this copy of gscore.
//
// The macOS application carries a General MIDI bank in its Resources so that
// it can make a sound the first time it is opened, with nothing to fetch and
// nothing to choose. Found by where the program itself is rather than by an
// installed path, so that a bundle dragged anywhere — another folder, another
// machine, a disk image somebody was sent — still finds its own instruments.
func bundledSoundfontDirs() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	return []string{
		// Gscore.app/Contents/MacOS/gscore → Contents/Resources/soundfonts
		filepath.Join(dir, "..", "Resources", "soundfonts"),
		// And beside the binary, which is where the other platforms put
		// anything that travels with it.
		filepath.Join(dir, "soundfonts"),
	}
}

// installed is one soundfont found on this machine.
type installed struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Where string `json:"where"`
}

// listSoundfonts is every soundfont gscore can reach, the ones loaded into
// this process first and then whatever is on the disk.
func listSoundfonts() []installed {
	memoryFonts.Lock()
	out := append([]installed(nil), memoryFonts.list...)
	memoryFonts.Unlock()
	return append(out, diskSoundfonts()...)
}

/*
 * Keeping them loaded
 */

// fonts holds the soundfonts already read, because reading one is thirty
// megabytes of samples and the dialog asks about them every time it is
// opened.
var fonts struct {
	sync.Mutex
	byPath map[string]*synth.Font
}

// memoryFonts are the soundfonts handed over as bytes rather than found on
// a disk, which is how the browser build gets any: it has no filesystem, so
// the page fetches the file and gives it to the core.
var memoryFonts struct {
	sync.Mutex
	list []installed
}

// memoryPrefix marks a font that lives only in this process. It is used as a
// path everywhere a path is used, so that nothing downstream has to know the
// difference.
const memoryPrefix = "memory:"

func loadFont(path string) (*synth.Font, error) {
	fonts.Lock()
	defer fonts.Unlock()
	if f, ok := fonts.byPath[path]; ok {
		return f, nil
	}
	if strings.HasPrefix(path, memoryPrefix) {
		// It was never on a disk, so if it is not already loaded it is gone:
		// a reloaded page has to be given it again.
		return nil, errors.New("those instruments are not loaded any more")
	}
	f, err := synth.LoadFont(path)
	if err != nil {
		return nil, err
	}
	if fonts.byPath == nil {
		fonts.byPath = map[string]*synth.Font{}
	}
	fonts.byPath[path] = f
	return f, nil
}

/*
 * What the score needs
 */

// voice is one staff of the score and what will be heard playing it.
type voice struct {
	Part       int    `json:"part"`
	Name       string `json:"name"`
	Instrument string `json:"instrument"`
	Program    int    `json:"program"`
	Drum       bool   `json:"drum"`
	// Font is the soundfont this part is set to play through, empty for
	// whichever one is the default.
	Font string `json:"font,omitempty"`
	// Preset is what that soundfont will actually sound.
	//
	// Substitute says it is not the instrument asked for: the font has
	// nothing at this program and its first preset is being used instead,
	// which is right for a one-instrument soundfont and wrong for a staff
	// pointed at the wrong file. Only the person looking can tell which,
	// so they are shown.
	Preset     string `json:"preset,omitempty"`
	Substitute bool   `json:"substitute"`
	Missing    bool   `json:"missing"`
}

// voices lists the parts of a score with the instrument each one sounds as.
//
// This is the review: what gscore thinks the piece is scored for, before
// anybody spends a minute rendering it and finds out that the staff they
// meant as a clarinet has been a piano all along.
func (a *App) voices(assign map[int]string, def string) []voice {
	var out []voice
	// What each staff really plays through, which is the dialog's own
	// choices and then whatever has been said about the instruments.
	chosen := a.partFonts(assign, def)
	// Hidden staves included: hiding one is a decision about the page, and
	// it still sounds.
	for i, p := range a.score.Parts {
		v := voice{
			Part:       i,
			Name:       partName(p, i),
			Instrument: p.Instrument.String(),
			Program:    int(p.Instrument),
			Drum:       p.Clef == model.Percussion,
			Font:       chosen[i],
		}
		if v.Drum {
			v.Instrument = "percussion"
		}
		path := v.Font
		if path == "" {
			path = def
		}
		if path != "" {
			if f, err := loadFont(path); err == nil {
				preset, exact := f.Lookup(v.Program, v.Drum)
				switch {
				case preset.Name == "":
					v.Missing = true
				case !exact:
					v.Preset, v.Substitute = strings.TrimSpace(preset.Name), true
				default:
					v.Preset = strings.TrimSpace(preset.Name)
				}
			}
		}
		out = append(out, v)
	}
	return out
}

// partName is what to call a staff in a list of them.
func partName(p *model.Part, i int) string {
	if n := strings.TrimSpace(p.Name); n != "" {
		return n
	}
	return "Staff " + strconv.Itoa(i+1)
}

/*
 * The API
 */

// isCollection says an entry in the list is not one soundfont but hundreds,
// which are browsed rather than downloaded whole.
const zanderjazID = "zanderjaz"

// handleSoundfonts answers with everything the recording dialog shows: the
// soundfonts on this machine, the ones that can be fetched, and what the
// score is scored for.
func (a *App) handleSoundfonts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	def := q.Get("font")
	assign := parseAssignment(q.Get("assign"))

	a.mu.RLock()
	parts := a.voices(assign, def)
	a.mu.RUnlock()

	dir, _ := soundfontDir()
	_, zcount := zanderjazCategories()
	writeJSON(w, map[string]any{
		"installed": listSoundfonts(),
		"catalogue": catalogue,
		// The collection is served from a site that does not let pages read
		// it, so in the browser it can only be reached by gscore running on
		// a machine, or opened from a file already downloaded.
		"collection":  map[string]any{"id": zanderjazID, "count": zcount, "browser": false},
		"dir":         dir,
		"canDownload": a.local,
		"voices":      parts,
	})
}

// partFonts says which soundfont each staff of the score plays through.
//
// Three answers, and the nearest one wins:
//
//   - what this recording was told about this staff, which is the dialog's
//     own list and belongs to the export being made;
//   - what has been said about the instrument the staff is playing, which
//     is a standing answer kept in the settings — a real violin for every
//     violin, in this score and the next one;
//   - and failing both, whichever soundfont was chosen for the whole thing.
//
// The middle one is checked against the disk. A library chosen a year ago
// and since moved would otherwise take a staff silent, and the staff would
// be silent in a way nothing in the dialog explains: the fallback is the
// chosen soundfont, which at least plays something.
//
// A part not in the answer plays through the chosen one, which is nearly
// every part of nearly every score.
func (a *App) partFonts(assign map[int]string, def string) map[int]string {
	byInstrument := loadSettings().InstrumentFonts
	out := map[int]string{}
	for i, p := range a.score.Parts {
		if path := assign[i]; path != "" {
			out[i] = path
			continue
		}
		if path := byInstrument[p.Sound]; path != "" && path != def {
			if fileExists(path) {
				out[i] = path
			}
		}
	}
	return out
}

// parseAssignment reads "0=/a.sf2,2=/b.sf2": which part plays through which
// soundfont, for everything that is not on the default one.
func parseAssignment(s string) map[int]string {
	out := map[int]string{}
	for _, pair := range strings.Split(s, ",") {
		key, path, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if i, err := strconv.Atoi(strings.TrimSpace(key)); err == nil && path != "" {
			out[i] = path
		}
	}
	return out
}

// handleSoundfontFetch downloads a soundfont and keeps it.
//
// Either one from the list, by id, or any link at all. What arrives is read
// as a soundfont before it is kept: a link that has rotted into a web page,
// or a download that stopped halfway, is a file that would otherwise sit
// there looking installed and playing nothing.
func (a *App) handleSoundfontFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !a.local {
		http.Error(w, "soundfonts can only be downloaded on this machine",
			http.StatusForbidden)
		return
	}
	q := r.URL.Query()
	url, name := q.Get("url"), q.Get("name")
	if id := q.Get("id"); id != "" {
		found := false
		for _, s := range catalogue {
			if s.ID == id {
				url, name, found = s.URL, s.Name, true
			}
		}
		if !found {
			http.Error(w, "no such soundfont", http.StatusBadRequest)
			return
		}
	}
	if !strings.HasPrefix(url, "https://") {
		http.Error(w, "a soundfont is fetched over https", http.StatusBadRequest)
		return
	}

	// Counted as it arrives, where the caller gave a job to count it under.
	// The request still answers when the download is finished and with what
	// it always answered with: this is a second way of asking, not a
	// different shape of the same one.
	prog := watchDownload(q.Get("job"))
	path, err := fetchSoundfont(url, name, prog)
	if err != nil {
		prog.finish(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	f, err := loadFont(path)
	if err != nil {
		os.Remove(path)
		err = fmt.Errorf("what arrived is not a soundfont: %w", err)
		prog.finish(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	prog.finish(nil)
	size := int64(0)
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}
	writeJSON(w, map[string]any{
		"status": "ok",
		"font": installed{
			Path: path, Name: f.Name, Size: size, Where: "downloaded",
		},
	})
}

/*
 * Saying how far a download has got
 */

// download is one soundfont on its way in.
//
// A soundfont is thirty to two hundred megabytes, which on an ordinary line
// is somewhere between ten seconds and five minutes. For all of that the
// editor used to say "Downloading the soundfont…" and nothing else, and a
// sentence that has not changed in four minutes is indistinguishable from a
// program that has stopped. So the bytes are counted as they arrive and
// anybody who asks is told.
type download struct {
	mu       sync.Mutex
	received int64
	total    int64 // 0 where the server would not say
	done     bool
	failed   string
	started  time.Time
	// ended is when it stopped, so that the rate is measured over the
	// download and not over how long ago it happened: left running, the
	// speed shown beside a finished download falls away towards nothing.
	ended time.Time
}

func (d *download) add(n int) {
	d.mu.Lock()
	d.received += int64(n)
	d.mu.Unlock()
}

func (d *download) finish(err error) {
	d.mu.Lock()
	d.done = true
	d.ended = time.Now()
	if err != nil {
		d.failed = err.Error()
	}
	d.mu.Unlock()
}

// over says the download finished longer ago than this, which is what
// makes it one to forget. Asked of the moment it ended rather than of how
// long it took: a download that took half a second is not one to keep for
// ever.
func (d *download) over(age time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.done && !d.ended.IsZero() && time.Since(d.ended) > age
}

// state is a copy, so that a reader never holds the lock a writer wants.
func (d *download) state() (received, total int64, done bool, failed string, since time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	took := time.Since(d.started)
	if !d.ended.IsZero() {
		took = d.ended.Sub(d.started)
	}
	return d.received, d.total, d.done, d.failed, took
}

var downloads struct {
	sync.Mutex
	byJob map[string]*download
}

// downloadKept is how long a finished download is still answered for.
//
// The page asks a few times a second and stops when it is told the download
// is done; this is for the one it was in the middle of asking when that
// happened, and for anybody who looks a moment later.
const downloadKept = 2 * time.Minute

// watchDownload starts counting for a job, and forgets the old ones.
func watchDownload(job string) *download {
	if job == "" {
		return &download{started: time.Now()}
	}
	downloads.Lock()
	defer downloads.Unlock()
	if downloads.byJob == nil {
		downloads.byJob = map[string]*download{}
	}
	for id, d := range downloads.byJob {
		if d.over(downloadKept) {
			delete(downloads.byJob, id)
		}
	}
	d := &download{started: time.Now()}
	downloads.byJob[job] = d
	return d
}

// handleSoundfontProgress says how far a download has got.
//
// A job nobody started answers as one that has not begun rather than as an
// error: the page may well ask before the download has been registered, and
// a refusal there would be a progress bar that flashes a failure on its way
// up.
func (a *App) handleSoundfontProgress(w http.ResponseWriter, r *http.Request) {
	job := r.URL.Query().Get("job")
	downloads.Lock()
	d := downloads.byJob[job]
	downloads.Unlock()
	if d == nil {
		writeJSON(w, map[string]any{"received": 0, "total": 0, "done": false})
		return
	}
	received, total, done, failed, since := d.state()
	out := map[string]any{
		"received": received,
		"total":    total,
		"done":     done,
		"seconds":  since.Seconds(),
	}
	if failed != "" {
		out["error"] = failed
	}
	// Bytes a second, measured over the whole download rather than the last
	// moment: an average that settles is more use than a number that jumps
	// about, and the only thing anybody does with it is estimate the wait.
	if s := since.Seconds(); s > 0.25 && received > 0 {
		out["bytesPerSecond"] = float64(received) / s
		if total > received {
			out["secondsLeft"] = float64(total-received) / (float64(received) / s)
		}
	}
	writeJSON(w, out)
}

// counter passes bytes through and counts them.
type counter struct {
	r io.Reader
	d *download
}

func (c counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.d.add(n)
	}
	return n, err
}

// fetchSoundfont downloads to a temporary name beside the destination and
// moves it into place when it is whole, so that an interrupted download is
// never mistaken for a soundfont.
func fetchSoundfont(url, name string, prog *download) (string, error) {
	dir, err := soundfontDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(url), filepath.Ext(url))
	}
	path := filepath.Join(dir, safeFileName(name)+".sf2")

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the download answered %s", resp.Status)
	}
	// How big it is, where the other end says. A server that does not say
	// leaves this at zero, and the bar then shows what has arrived rather
	// than how much of it is left — which is all anybody can be told.
	var body io.Reader = resp.Body
	if prog != nil {
		prog.mu.Lock()
		prog.total = resp.ContentLength
		if prog.total < 0 {
			prog.total = 0
		}
		prog.mu.Unlock()
		body = counter{r: resp.Body, d: prog}
	}

	tmp, err := os.CreateTemp(dir, "download-*.part")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(tmp, body)
	tmp.Close()
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}

func safeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "soundfont"
	}
	return out
}

// handleSoundfontLoad takes a soundfont as the body of a request and keeps
// it in memory.
//
// For the browser, which has no disk to point at: the page fetches the file
// — from wherever, and out of its own cache the second time — and hands the
// bytes over. Reading it here rather than in JavaScript means the same code
// plays it, answers what is in it and records with it as everywhere else.
func (a *App) handleSoundfontLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "soundfont"
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f, err := synth.LoadFontFrom(bytes.NewReader(data), name)
	if err != nil {
		http.Error(w, "that is not a soundfont: "+err.Error(),
			http.StatusBadRequest)
		return
	}
	key := memoryPrefix + name

	fonts.Lock()
	if fonts.byPath == nil {
		fonts.byPath = map[string]*synth.Font{}
	}
	fonts.byPath[key] = f
	fonts.Unlock()

	entry := installed{
		Path: key, Name: f.Name, Size: int64(len(data)), Where: "loaded here",
	}
	memoryFonts.Lock()
	kept := memoryFonts.list[:0]
	for _, e := range memoryFonts.list {
		if e.Path != key {
			kept = append(kept, e)
		}
	}
	memoryFonts.list = append(kept, entry)
	memoryFonts.Unlock()

	writeJSON(w, map[string]any{"status": "ok", "font": entry})
}

// handleSoundfontPick opens the system's file panel on a soundfont.
func (a *App) handleSoundfontPick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !a.local {
		http.Error(w, "only on this machine", http.StatusForbidden)
		return
	}
	path, err := pickFile()
	if err != nil {
		writeJSON(w, map[string]any{"unavailable": true})
		return
	}
	if path == "" {
		writeJSON(w, map[string]any{"cancelled": true})
		return
	}
	f, err := loadFont(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	size := int64(0)
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}
	writeJSON(w, map[string]any{
		"status": "ok",
		"font":   installed{Path: path, Name: f.Name, Size: size, Where: "chosen"},
	})
}

// presetsJSON is a soundfont's whole contents, for somebody who wants to see
// what is in one.
func (a *App) handleSoundfontPresets(w http.ResponseWriter, r *http.Request) {
	f, err := loadFont(r.URL.Query().Get("font"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, _ := json.Marshal(f.Presets())
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// canSound says whether gscore may play through a soundfont here.
//
// On the machine it is running on, or inside the page when the core is the
// page. Not over a network: the soundfonts are files on somebody's disk and
// rendering an orchestra is work they did not offer to do for strangers.
func (a *App) canSound() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.local || a.inBrowser
}

var errNoSoundfont = errors.New(
	"there is no soundfont to play this with: choose or download one first")

// soundfontDisplayName is what to call a soundfont in front of somebody.
//
// The name the lists use, found by the path it was chosen by, so that
// everything naming it names it the same: the dialog it was picked in, the
// status line while it plays, and the sentence that says a recording was
// made with it.
//
// A soundfont carries a name of its own inside the file and it is often not
// the name anybody knows it by — the bank inside NAProGM.sf2 calls itself
// "NAProPercussionFull". Where the path is not one of the installed ones —
// a font loaded into this process from a page, say — the file's own name is
// all there is, and it is used.
func soundfontDisplayName(path, inside string) string {
	if path != "" {
		for _, f := range listSoundfonts() {
			if f.Path == path {
				return f.Name
			}
		}
	}
	if inside != "" {
		return inside
	}
	return "the chosen soundfont"
}
