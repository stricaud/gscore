/*
 * scanengine.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * Reading photographed and scanned pages: which program does it, and the
 * request that hands it pages.
 *
 * The program is homr, which runs as a process of its own — see
 * internal/format/scanned. Where it is found, in order:
 *
 *   1. GSCORE_HOMR, a whole command line, for whoever has their own copy.
 *   2. The command typed in Settings, where that is what was chosen there.
 *   3. A copy shipped beside the gscore binary, in the package that carries
 *      one.
 *   4. uv, where it is installed: uvx fetches homr the first time and keeps
 *      it. Settings chooses between the processor and an NVIDIA card here,
 *      which is a matter of which build of the runtime uv fetches; homr
 *      picks the device itself once it runs.
 *   5. homr on the PATH.
 */

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/stricaud/gscore/internal/format/scanned"
	"github.com/stricaud/gscore/internal/model"
)

// scanEngineEnv names the command that reads pages, for whoever has one.
const scanEngineEnv = "GSCORE_HOMR"

// scanKeepalive is how often the reply to a page being read says it is
// still coming. The native window gives up on a reply that has gone quiet
// for a minute, and a page takes longer than that on a processor.
const scanKeepalive = 5 * time.Second

// pageLimit is the most a page may weigh: a photograph from a phone is a
// few megabytes, a 600 dpi scan twenty.
const pageLimit = 64 << 20

func init() { scanned.Find = scanEngine }

var errNoScanEngine = fmt.Errorf("%w: install uv (https://docs.astral.sh/uv/) and gscore "+
	"will run homr through it, or set %s to the command that runs homr", scanned.ErrNoEngine, scanEngineEnv)

// scanEngine is the page reader to run, found as the file comment says.
func scanEngine() (scanned.Engine, error) {
	if v := strings.TrimSpace(os.Getenv(scanEngineEnv)); v != "" {
		argv := splitCommand(v)
		if len(argv) == 0 {
			return scanned.Engine{}, fmt.Errorf("%s is set but holds no command", scanEngineEnv)
		}
		return scanned.Engine{Command: argv, Source: "the " + scanEngineEnv + " variable"}, nil
	}
	s := loadSettings()
	if s.ScanReader == "custom" {
		argv := splitCommand(s.ScanCommand)
		if len(argv) == 0 {
			return scanned.Engine{}, errors.New("the page reader's command in Settings is empty")
		}
		return scanned.Engine{Command: argv, Source: "the command in Settings"}, nil
	}
	if p := scanEngineBeside(); p != "" {
		return scanned.Engine{Command: []string{p}, Source: "the copy beside gscore"}, nil
	}
	if uvx, err := exec.LookPath("uvx"); err == nil {
		extra := "cpu"
		if s.ScanReader == "cuda" {
			extra = "cuda"
		}
		return scanned.Engine{
			Command: []string{uvx, "--from", "homr[" + extra + "]", "homr"},
			Source:  "uv on this machine",
		}, nil
	}
	if p, err := exec.LookPath("homr"); err == nil {
		return scanned.Engine{Command: []string{p}, Source: "the PATH"}, nil
	}
	return scanned.Engine{}, errNoScanEngine
}

// scanEngineBeside is a homr shipped beside the binary, as
// packaging/homr/build.sh lays it out: a folder named homr, or homr-arm64
// and the like where a package carries one build of it for a universal
// binary, next to the executable or in a macOS bundle's Resources. Empty
// where there is none.
func scanEngineBeside() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return scanEngineIn(filepath.Dir(exe), runtime.GOARCH)
}

// scanEngineIn looks for the shipped homr from the folder the binary is
// in, for this architecture.
func scanEngineIn(dir, arch string) string {
	for _, folder := range []string{"homr-" + arch, "homr"} {
		for _, rel := range []string{
			folder + "/bin/homr", "../Resources/" + folder + "/bin/homr",
			folder + "/homr.cmd", folder + "/Scripts/homr.exe",
		} {
			p := filepath.Join(dir, rel)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
	}
	return ""
}

// splitCommand cuts a command line into its words the way a shell would
// for the simple cases: on spaces, except inside single or double quotes.
// uvx --from 'homr[cpu]' homr is four words, the third without its quotes.
func splitCommand(s string) []string {
	var out []string
	var cur strings.Builder
	in, quote := false, rune(0)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, in = r, true
		case unicode.IsSpace(r):
			if in {
				out = append(out, cur.String())
				cur.Reset()
				in = false
			}
		default:
			cur.WriteRune(r)
			in = true
		}
	}
	if in {
		out = append(out, cur.String())
	}
	return out
}

// handleOpenScan reads pictures of pages as one score: the files sent as
// the request's "page" parts, in their order; or the addresses url= names,
// fetched, and the files path= names, read, in their order. Fetching and
// reading by path are for the machine the editor runs on, as opening by
// path is: a server reached from elsewhere does not go and get things for
// whoever asks.
//
// The reply is held open while the reader works — see slowreply.go — and
// is then the answer: {"status":"ok",...} or {"error":"..."}.
func (a *App) handleOpenScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	eng, err := scanned.Find()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dir, err := os.MkdirTemp("", "gscore-pages-")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(dir)

	var paths, names []string
	keep := func(name string, data []byte) error {
		if scanned.Kind(data) == "" {
			return fmt.Errorf("%s is neither a PNG nor a JPEG", name)
		}
		p := filepath.Join(dir, fmt.Sprintf("%02d%s", len(paths)+1, scanned.Kind(data)))
		if err := os.WriteFile(p, data, 0o600); err != nil {
			return err
		}
		paths, names = append(paths, p), append(names, name)
		return nil
	}
	if urls, files := r.URL.Query()["url"], r.URL.Query()["path"]; len(urls) > 0 || len(files) > 0 {
		if !a.local {
			http.Error(w, "fetching pages from the web, or reading them by path, "+
				"is for the machine the editor runs on", http.StatusForbidden)
			return
		}
		for _, u := range urls {
			name, data, err := fetchPage(r.Context(), u)
			if err == nil {
				err = keep(name, data)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err == nil {
				err = keep(filepath.Base(f), data)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	} else {
		mr, err := r.MultipartReader()
		if err != nil {
			http.Error(w, "send the pages as parts named page, or url= addresses to fetch",
				http.StatusBadRequest)
			return
		}
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if part.FormName() != "page" {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(part, pageLimit))
			if err == nil {
				err = keep(filepath.Base(part.FileName()), data)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	if len(paths) == 0 {
		http.Error(w, "no pages were sent", http.StatusBadRequest)
		return
	}

	first := names[0]
	slowly(w, func() (any, error) {
		return a.openPictures(r.Context(), eng, paths, first)
	}, badRequest)
}

// openPictures has the page reader read pictures of pages as one score and
// makes it the document, named after the first page, which is what
// somebody will look for when they save it.
func (a *App) openPictures(ctx context.Context, eng scanned.Engine, paths []string, first string) (map[string]any, error) {
	a.scan.begin()
	s, err := scanned.ImportFilesWith(ctx, eng, paths, a.scan.report)
	a.scan.end()
	if s == nil {
		if err == nil {
			err = errors.New("the pages could not be read")
		}
		return nil, err
	}
	name := strings.TrimSuffix(first, filepath.Ext(first)) + ".gscore"
	a.lock()
	a.forgetPDF()
	a.setBook([]*model.Score{s}, name, err)
	a.mu.Unlock()
	out := map[string]any{
		"status": "ok", "filename": name, "tunes": 1,
		"pages": len(paths), "engine": eng.String(),
	}
	if err != nil {
		out["partial"] = err.Error()
	}
	return out, nil
}

// fetchPage gets one page from the web, and names it after the end of its
// address.
func fetchPage(ctx context.Context, raw string) (string, []byte, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", nil, fmt.Errorf("%q is not a web address", raw)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "gscore")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("%s could not be fetched: %w", u.Host, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("%s answered %s", u.Host, res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, pageLimit))
	if err != nil {
		return "", nil, err
	}
	name := path.Base(u.Path)
	if name == "" || name == "/" || name == "." {
		name = "page"
	}
	return name, data, nil
}

// scanEngineReport is the page reader as Settings shows it: the command
// and where it was found, or why there is none.
func scanEngineReport() map[string]string {
	eng, err := scanEngine()
	if err != nil {
		return map[string]string{"error": err.Error()}
	}
	return map[string]string{"command": eng.String(), "source": eng.Source}
}

// scanReaderChoice is the setting as the dialog shows it: "cpu" where
// nothing was chosen.
func scanReaderChoice(s settings) string {
	if s.ScanReader == "" {
		return "cpu"
	}
	return s.ScanReader
}
