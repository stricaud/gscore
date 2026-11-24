//go:build !js

/*
 * video.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 *
 * The score as a film: the music going past, a piano along the bottom with
 * its keys going down as the notes sound, and the recording under it — the
 * kind of thing that is put on a video site so that people can hear a piece
 * and follow it at the same time.
 *
 * Two programs do what gscore does not: rsvg-convert draws the engraving as
 * pictures, and ffmpeg makes a film of them with the sound. Neither is
 * carried; both are asked for by name when they are not there. Everything
 * between — where the music is at each moment, which keys are down, the
 * frames themselves — is worked out here, from the same playback the Play
 * button uses, so the film shows what the editor shows.
 */

package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/stricaud/gscore/internal/brand"
	"github.com/stricaud/gscore/internal/engrave"
	"github.com/stricaud/gscore/internal/model"
	"github.com/stricaud/gscore/internal/render/svg"
)

// The film's shape: what the video sites expect, and the frame rate a
// cursor moves smoothly at.
const (
	videoW, videoH = 1920, 1080
	videoFPS       = 30
	// videoKeys is how tall the piano along the bottom is, and videoMargin
	// the space round the music above it.
	videoKeys   = 170
	videoMargin = 40
	// videoLead is how far across the frame the cursor is held, as on the
	// screen: most of the frame is the music ahead.
	videoLead = 0.32
	// videoBand is how wide each picture of the strip is drawn, in pixels:
	// a long piece is too wide to draw as one picture.
	videoBand = 4096
)

// videoTools finds the two programs the film needs, and says which are
// missing.
func videoTools() (ffmpeg, rsvg string, missing []string) {
	ffmpeg, _ = exec.LookPath("ffmpeg")
	rsvg, _ = exec.LookPath("rsvg-convert")
	if ffmpeg == "" {
		missing = append(missing, "ffmpeg")
	}
	if rsvg == "" {
		missing = append(missing, "rsvg-convert")
	}
	return ffmpeg, rsvg, missing
}

// videoInstall is how to get them, for the machine gscore is on.
func videoInstall() string {
	if _, err := exec.LookPath("brew"); err == nil {
		return "brew install ffmpeg librsvg"
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "sudo apt-get install ffmpeg librsvg2-bin"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "sudo dnf install ffmpeg librsvg2-tools"
	}
	return "install ffmpeg and librsvg (for rsvg-convert)"
}

// errVideoTools is the refusal when one of the programs is not there.
func errVideoTools(missing []string) error {
	return fmt.Errorf("a video needs %s, which is not installed: %s",
		strings.Join(missing, " and "), videoInstall())
}

// handleVideoTools says whether a video can be made here, and if not, what
// is missing and how to install it.
//
//	GET /api/videotools  →  {"missing": ["ffmpeg"], "install": "brew install ffmpeg librsvg"}
func (a *App) handleVideoTools(w http.ResponseWriter, r *http.Request) {
	_, _, missing := videoTools()
	if missing == nil {
		missing = []string{}
	}
	writeJSON(w, map[string]any{"missing": missing, "install": videoInstall()})
}

// videoProgress is how far the film being made has got, for the window to
// show while it waits: a film of a few minutes takes a minute or two, and a
// window that says only "making the video" for that long looks stuck.
var videoProgress progress

type progress struct {
	sync.Mutex
	phase       string // sound, pictures, frames, finishing; empty when idle
	done, total int
}

func (p *progress) set(phase string, done, total int) {
	p.Lock()
	p.phase, p.done, p.total = phase, done, total
	p.Unlock()
}

// percent is how much of the film is made: the frames are nearly all of the
// time, so they are nearly all of the count.
func (p *progress) percent() (string, int) {
	p.Lock()
	defer p.Unlock()
	switch p.phase {
	case "frames":
		if p.total > 0 {
			return p.phase, min(99, 100*p.done/p.total)
		}
	case "finishing":
		return p.phase, 99
	}
	return p.phase, 0
}

// handleVideoProgress says how far the film being made has got.
//
//	GET /api/videoprogress  →  {"phase": "frames", "percent": 42}
func (a *App) handleVideoProgress(w http.ResponseWriter, r *http.Request) {
	phase, pct := videoProgress.percent()
	writeJSON(w, map[string]any{"phase": phase, "percent": pct})
}

// recordVideo writes the score as an MP4 film.
func recordVideo(ctx context.Context, out io.Writer, s *model.Score, rec recording) error {
	ffmpeg, rsvg, missing := videoTools()
	if len(missing) > 0 {
		return errVideoTools(missing)
	}
	dir, err := os.MkdirTemp("", "gscore-video")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	videoProgress.set("sound", 0, 1)
	defer videoProgress.set("", 0, 0)
	// The sound, made first: it is what the film is as long as.
	audio := filepath.Join(dir, "sound.flac")
	af, err := os.Create(audio)
	if err != nil {
		return err
	}
	err = recordFLAC(af, s, rec)
	if cerr := af.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("the sound could not be made: %w", err)
	}

	pb := buildPlayback(s)
	videoProgress.set("pictures", 0, 1)
	opt := engrave.DefaultOptions()
	opt.Mode = engrave.Strip
	lay := engrave.Engrave(s, opt)

	f, err := newFilm(ctx, lay, pb, rsvg, dir)
	if err != nil {
		return err
	}

	film := filepath.Join(dir, "film.mp4")
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", videoW, videoH), "-r", fmt.Sprint(videoFPS), "-i", "-",
		"-i", audio,
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "20",
		"-c:a", "aac", "-b:a", "192k", "-shortest", "-movflags", "+faststart",
		film)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// As long as the music, and a moment after it for the last note to
	// ring; -shortest trims the film to the sound.
	frames := int(math.Ceil((pb.Duration + rec.Tail + 1) * videoFPS))
	w := bufio.NewWriterSize(in, 1<<20)
	frame := image.NewRGBA(image.Rect(0, 0, videoW, videoH))
	var werr error
	for i := 0; i < frames && werr == nil; i++ {
		if err := ctx.Err(); err != nil {
			werr = err
			break
		}
		f.draw(frame, float64(i)/videoFPS)
		_, werr = w.Write(frame.Pix)
		videoProgress.set("frames", i+1, frames)
	}
	if werr == nil {
		werr = w.Flush()
	}
	videoProgress.set("finishing", frames, frames)
	in.Close()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg could not make the film: %v %s", err,
			strings.TrimSpace(stderr.String()))
	}
	// ffmpeg stops taking frames once the sound has ended, the film being
	// as long as its shortest part, and closes the pipe on the frames still
	// on their way. That is the film finished, not a failure — so long as
	// ffmpeg says it made it, which is the check above. Stopping is not.
	if werr != nil && !errors.Is(werr, syscall.EPIPE) {
		return werr
	}
	made, err := os.Open(film)
	if err != nil {
		return err
	}
	defer made.Close()
	_, err = io.Copy(out, made)
	return err
}

// film is everything a frame is drawn from.
type film struct {
	ctx   context.Context
	lay   *engrave.Layout
	pb    Playback
	scale float64 // pixels to the staff space
	top   int     // where the music starts down the frame
	tall  int     // how tall the music is, in pixels
	wide  int     // how wide the whole strip is, in pixels
	rsvg  string
	dir   string
	// bands are the pictures of the strip, drawn when first wanted.
	bands map[int]*image.RGBA
	// stamp is gscore's mark in the corner: Gilbert bobbing, as he does
	// while the editor is busy, and the name under him.
	stamp *stamp
	// path is where the cursor is when, from the notes: each note's time
	// and the x of its column.
	path []cursorPoint
}

type cursorPoint struct{ t, x float64 }

func newFilm(ctx context.Context, lay *engrave.Layout, pb Playback, rsvg, dir string) (*film, error) {
	height := lay.Content
	if height <= 0 {
		height = lay.Height
	}
	room := float64(videoH - videoKeys - 2*videoMargin)
	scale := math.Min(room/height, 24)
	f := &film{
		ctx: ctx, lay: lay, pb: pb, scale: scale, rsvg: rsvg, dir: dir,
		tall:  int(math.Ceil(height * scale)),
		wide:  int(math.Ceil(lay.Width * scale)),
		bands: map[int]*image.RGBA{},
	}
	f.top = videoMargin + (int(room)-f.tall)/2
	st, err := newStamp(ctx, rsvg, dir)
	if err != nil {
		return nil, err
	}
	f.stamp = st

	// Where each note is on the strip, by the number the playback and the
	// engraving both give it.
	at := map[int]float64{}
	for _, h := range lay.Hits {
		for _, el := range h.Elements {
			at[el.Ref] = el.X + el.Width/2
		}
	}
	for _, ev := range pb.Events {
		if x, ok := at[ev.Ref]; ok {
			f.path = append(f.path, cursorPoint{ev.Start, x})
		}
	}
	sort.SliceStable(f.path, func(i, j int) bool { return f.path[i].t < f.path[j].t })
	if len(f.path) == 0 {
		return nil, errors.New("there are no notes to make a film of")
	}
	return f, nil
}

// cursorAt is where the music is at time t, in staff spaces along the strip:
// gliding from one note to the next, and jumping where a repeat goes back.
func (f *film) cursorAt(t float64) float64 {
	p := f.path
	i := sort.Search(len(p), func(i int) bool { return p[i].t > t })
	switch {
	case i == 0:
		return p[0].x
	case i == len(p):
		return p[len(p)-1].x
	}
	a, b := p[i-1], p[i]
	if b.x < a.x || b.t <= a.t {
		return a.x // back to the start of a repeat: no glide backwards
	}
	return a.x + (b.x-a.x)*(t-a.t)/(b.t-a.t)
}

// band is the i-th picture of the strip, drawn the first time it is wanted.
// Only the ones near the cursor are kept.
func (f *film) band(i int) (*image.RGBA, error) {
	if b, ok := f.bands[i]; ok {
		return b, nil
	}
	for k := range f.bands {
		if k < i-1 || k > i+1 {
			delete(f.bands, k)
		}
	}
	w := float64(videoBand) / f.scale
	svgPath := filepath.Join(f.dir, fmt.Sprintf("band%04d.svg", i))
	pngPath := filepath.Join(f.dir, fmt.Sprintf("band%04d.png", i))
	out, err := os.Create(svgPath)
	if err != nil {
		return nil, err
	}
	height := float64(f.tall) / f.scale
	err = svg.Render(out, f.lay, svg.Options{
		StaffSpace: f.scale, EmbedFont: true,
		Crop: &svg.Box{X0: float64(i) * w, Y0: 0, X1: float64(i+1) * w, Y1: height},
	})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(f.ctx, f.rsvg, "-b", "white", "-o", pngPath, svgPath)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("rsvg-convert could not draw the score: %v %s", err, msg)
	}
	pf, err := os.Open(pngPath)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(pf)
	pf.Close()
	if err != nil {
		return nil, err
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	f.bands[i] = rgba
	os.Remove(svgPath)
	os.Remove(pngPath)
	return rgba, nil
}

var (
	videoPaper = color.RGBA{255, 255, 255, 255}
	// Seen through: the music under it stays readable. Not premultiplied,
	// so the colour is the colour.
	videoCursor = color.NRGBA{59, 125, 216, 70}
	videoWhite  = color.RGBA{246, 244, 240, 255}
	videoBlack  = color.RGBA{28, 26, 24, 255}
	videoEdge   = color.RGBA{138, 133, 125, 255}
	// The staves' colours, as the keyboard in the editor has them.
	videoHands = []color.RGBA{
		{59, 125, 216, 255}, {224, 137, 58, 255}, {76, 175, 106, 255}, {181, 87, 154, 255},
	}
)

// draw makes the frame for time t.
func (f *film) draw(frame *image.RGBA, t float64) {
	draw.Draw(frame, frame.Bounds(), image.NewUniform(videoPaper), image.Point{}, draw.Src)

	// The music, with the cursor held a third of the way across.
	cx := f.cursorAt(t) * f.scale
	left := int(cx - videoLead*videoW)
	left = max(0, min(left, f.wide-videoW))
	for x := 0; x < videoW; {
		sx := left + x
		bi := sx / videoBand
		b, err := f.band(bi)
		if err != nil {
			break
		}
		bx := sx - bi*videoBand
		n := min(min(videoW-x, videoBand-bx), b.Bounds().Dx()-bx)
		if n <= 0 {
			break
		}
		h := min(f.tall, b.Bounds().Dy())
		draw.Draw(frame, image.Rect(x, f.top, x+n, f.top+h), b, image.Pt(bx, 0), draw.Src)
		x += n
	}
	// The cursor: a band of colour over the music where it is now.
	cur := int(cx) - left
	draw.Draw(frame, image.Rect(cur-3, f.top-videoMargin/2, cur+3, f.top+f.tall+videoMargin/2),
		image.NewUniform(videoCursor), image.Point{}, draw.Over)

	// The piano, with the keys down that are sounding now.
	down := map[int]int{}
	for _, ev := range f.pb.Events {
		if ev.Drum == 0 && t >= ev.Start && t < ev.Start+ev.Dur {
			if _, ok := down[ev.MIDI]; !ok {
				down[ev.MIDI] = ev.Part
			}
		}
	}
	drawKeys(frame, image.Rect(videoMargin, videoH-videoKeys-10, videoW-videoMargin, videoH-10), down)
	f.stamp.draw(frame, t)
}

// drawKeys draws eighty-eight keys in a box, with those in down pressed in
// the colour of the staff they were played from.
func drawKeys(img *image.RGBA, box image.Rectangle, down map[int]int) {
	const low, high = 21, 108
	whites := 0
	for m := low; m <= high; m++ {
		if !blackKey(m) {
			whites++
		}
	}
	ww := float64(box.Dx()) / float64(whites)
	fill := func(r image.Rectangle, c color.RGBA) {
		draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
	}
	keyColour := func(m int, plain color.RGBA) color.RGBA {
		if part, ok := down[m]; ok {
			return videoHands[part%len(videoHands)]
		}
		return plain
	}
	x := float64(box.Min.X)
	var blacks []image.Rectangle
	var blackMidi []int
	for m := low; m <= high; m++ {
		if blackKey(m) {
			bw := ww * 0.6
			blacks = append(blacks, image.Rect(int(x-bw/2), box.Min.Y,
				int(x+bw/2), box.Min.Y+box.Dy()*62/100))
			blackMidi = append(blackMidi, m)
			continue
		}
		r := image.Rect(int(x), box.Min.Y, int(x+ww), box.Max.Y)
		fill(r, videoEdge)
		fill(r.Inset(1), keyColour(m, videoWhite))
		x += ww
	}
	for i, r := range blacks {
		fill(r, videoBlack)
		if c := keyColour(blackMidi[i], videoBlack); c != videoBlack {
			fill(r.Inset(2), c)
		}
	}
}

// blackKey says whether a MIDI note is a black key.
func blackKey(m int) bool {
	switch ((m % 12) + 12) % 12 {
	case 1, 3, 6, 8, 10:
		return true
	}
	return false
}

/*
 * The stamp.
 *
 * Gilbert, in the top right of every frame, bobbing as he does in the
 * editor while it works — up and down by ten pixels and rocking three
 * degrees either way, every 1.4 seconds — with "gscore" under him: so a
 * film that is passed round says where it came from.
 */

const (
	stampSize   = 104 // Gilbert's height, in pixels
	stampMargin = 28
	stampPeriod = 1.4 // seconds for one bob, as the editor's busy-bob
	stampFrames = 42  // one bob at thirty frames a second
	stampRock   = 3.0 // degrees either way
	stampRise   = 10.0
	stampPad    = 12 // room round him for the rocking to turn into
)

type stamp struct {
	poses []*image.NRGBA // one per frame of a bob
	word  *image.NRGBA
}

func newStamp(ctx context.Context, rsvg, dir string) (*stamp, error) {
	img, err := png.Decode(strings.NewReader(string(brand.GilbertPNG)))
	if err != nil {
		return nil, err
	}
	small := scaleTo(img, stampSize)
	st := &stamp{}
	for k := 0; k < stampFrames; k++ {
		// Eased the way the editor's animation is: still at either end,
		// quickest through the middle.
		phase := float64(k) / stampFrames
		angle := -stampRock * math.Cos(2*math.Pi*phase)
		st.poses = append(st.poses, rotated(small, angle*math.Pi/180, stampPad))
	}
	// The name, drawn by the same program the score is: there is no
	// typeface in here to draw it with.
	svgPath := filepath.Join(dir, "stamp.svg")
	pngPath := filepath.Join(dir, "stamp.png")
	word := `<svg xmlns="http://www.w3.org/2000/svg" width="180" height="40">` +
		`<text x="90" y="29" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" ` +
		`font-size="26" font-weight="600" fill="#2b2b2b" letter-spacing="1">gscore</text></svg>`
	if err := os.WriteFile(svgPath, []byte(word), 0o600); err != nil {
		return nil, err
	}
	if msg, err := exec.CommandContext(ctx, rsvg, "-o", pngPath, svgPath).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("rsvg-convert could not draw the stamp: %v %s", err, msg)
	}
	pf, err := os.Open(pngPath)
	if err != nil {
		return nil, err
	}
	defer pf.Close()
	wimg, err := png.Decode(pf)
	if err != nil {
		return nil, err
	}
	st.word = image.NewNRGBA(wimg.Bounds())
	draw.Draw(st.word, st.word.Bounds(), wimg, wimg.Bounds().Min, draw.Src)
	return st, nil
}

// draw puts the stamp in the top right corner of the frame for time t.
func (st *stamp) draw(frame *image.RGBA, t float64) {
	k := int(math.Round(t*videoFPS)) % stampFrames
	phase := float64(k) / stampFrames
	rise := int(math.Round(stampRise * (1 - math.Cos(2*math.Pi*phase)) / 2))
	pose := st.poses[k]
	pw, ph := pose.Bounds().Dx(), pose.Bounds().Dy()
	ww, wh := st.word.Bounds().Dx(), st.word.Bounds().Dy()
	right := videoW - stampMargin
	centre := right - max(pw, ww)/2
	top := stampMargin - stampPad + int(stampRise)
	draw.Draw(frame, image.Rect(centre-pw/2, top-rise, centre-pw/2+pw, top-rise+ph),
		pose, image.Point{}, draw.Over)
	wy := top + ph - stampPad + 2
	draw.Draw(frame, image.Rect(centre-ww/2, wy, centre-ww/2+ww, wy+wh),
		st.word, image.Point{}, draw.Over)
}

// scaleTo makes a picture tall pixels high, averaging what it shrinks so
// that the lines stay smooth.
func scaleTo(src image.Image, tall int) *image.NRGBA {
	b := src.Bounds()
	k := float64(b.Dy()) / float64(tall)
	wide := int(math.Round(float64(b.Dx()) / k))
	out := image.NewNRGBA(image.Rect(0, 0, wide, tall))
	for y := 0; y < tall; y++ {
		for x := 0; x < wide; x++ {
			x0, y0 := float64(x)*k, float64(y)*k
			var r, g, bl, a, n float64
			for sy := int(y0); sy < int(math.Ceil(y0+k)) && sy < b.Dy(); sy++ {
				for sx := int(x0); sx < int(math.Ceil(x0+k)) && sx < b.Dx(); sx++ {
					c := color.NRGBAModel.Convert(src.At(b.Min.X+sx, b.Min.Y+sy)).(color.NRGBA)
					w := float64(c.A) / 255
					r += float64(c.R) * w
					g += float64(c.G) * w
					bl += float64(c.B) * w
					a += float64(c.A)
					n++
				}
			}
			if n == 0 || a == 0 {
				continue
			}
			wsum := a / 255
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / wsum), G: uint8(g / wsum), B: uint8(bl / wsum), A: uint8(a / n),
			})
		}
	}
	return out
}

// rotated turns a picture about its middle by angle radians, on a canvas
// pad pixels bigger all round so the corners have somewhere to go.
func rotated(src *image.NRGBA, angle float64, pad int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx()+2*pad, b.Dy()+2*pad
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	sin, cos := math.Sin(angle), math.Cos(angle)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Where this pixel comes from in the picture as it was.
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			sx := cos*dx + sin*dy + float64(b.Dx())/2
			sy := -sin*dx + cos*dy + float64(b.Dy())/2
			ix, iy := int(math.Floor(sx)), int(math.Floor(sy))
			if ix < 0 || iy < 0 || ix >= b.Dx() || iy >= b.Dy() {
				continue
			}
			out.SetNRGBA(x, y, src.NRGBAAt(b.Min.X+ix, b.Min.Y+iy))
		}
	}
	return out
}
