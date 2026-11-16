/*
 * assistant.go
 * gscore - a musical score editor
 *
 * (C) Copyright 2026 Sebastien Tricaud
 *
 * Permission is hereby granted, free of charge, under the terms of the MIT
 * licence; see the LICENSE file at the root of this repository.
 */

package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/stricaud/gscore/internal/format/abc"
	"github.com/stricaud/gscore/internal/model"
)

/*
 * An assistant that writes music
 *
 * A chat box beside the score: "write me the next four bars for every
 * instrument", "one swingy bar where the trumpet leads". The words go to a
 * language model along with the score so far, and what comes back is put
 * into the score as bars.
 *
 * The score goes out and comes back as ABC. It is the one notation every
 * model has read a great deal of, it says a whole band in a few lines, and
 * gscore already reads and writes it — so the assistant is asked for ABC
 * and nothing else, and what it answers is read the way any ABC file is.
 *
 * Four kinds of service: OpenAI, Anthropic, Mistral, and anything running
 * on this machine that speaks the OpenAI shape, which is what Ollama and
 * LM Studio do. The first three are the OpenAI shape too, except Anthropic,
 * which has its own; the difference is one function.
 *
 * None of it is there unless gscore is started with AI_ASSISTANT=1 in the
 * environment: no routes, no menu item, no settings. Somebody who has not
 * asked for a model should not find one in their editor.
 */

// assistantOn says whether the assistant exists in this run of the
// program. Read once, from the environment, when the program starts.
var assistantOn = os.Getenv("AI_ASSISTANT") == "1"

// AssistantEnabled reports whether the assistant is switched on.
func AssistantEnabled() bool { return assistantOn }

// assistantSettings is which service writes the music, and how to reach it.
// Kept with the rest of what gscore remembers between sessions.
type assistantSettings struct {
	// Provider is openai, anthropic, mistral or local.
	Provider string `json:"provider,omitempty"`
	// BaseURL is where the service answers. Empty means the provider's
	// own address; a local service has to say — http://localhost:11434/v1
	// is Ollama's.
	BaseURL string `json:"baseURL,omitempty"`
	APIKey  string `json:"apiKey,omitempty"`
	// Model is which of the service's models to ask.
	Model string `json:"model,omitempty"`
}

// providerBases are where each service answers unless told otherwise.
var providerBases = map[string]string{
	"openai":    "https://api.openai.com/v1",
	"anthropic": "https://api.anthropic.com/v1",
	"mistral":   "https://api.mistral.ai/v1",
	"local":     "http://localhost:11434/v1",
}

// defaultModels is the model asked for when none has been chosen.
var defaultModels = map[string]string{
	"openai":    "gpt-4o",
	"anthropic": "claude-opus-5-5",
	"mistral":   "mistral-large-latest",
}

func (s assistantSettings) provider() string {
	if _, ok := providerBases[s.Provider]; ok {
		return s.Provider
	}
	return "local"
}

func (s assistantSettings) base() string {
	if b := strings.TrimRight(strings.TrimSpace(s.BaseURL), "/"); b != "" {
		return b
	}
	return providerBases[s.provider()]
}

func (s assistantSettings) model() string {
	if m := strings.TrimSpace(s.Model); m != "" {
		return m
	}
	return defaultModels[s.provider()]
}

// anthropicVersion is the API version the Anthropic requests are made in.
const anthropicVersion = "2026-10-21"

// assistantClient makes the requests. A field so a test can point it at a
// server of its own.
var assistantClient = &http.Client{}

// assistantCall does one request to a service and hands back the body,
// with the service's own words in the error when it refused.
func assistantCall(ctx context.Context, s assistantSettings, method, path string, body any) ([]byte, error) {
	resp, err := assistantOpen(ctx, s, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// assistantOpen starts a request and hands back the response to read,
// with the service's own words in the error when it refused.
func assistantOpen(ctx context.Context, s assistantSettings, method, path string, body any) (*http.Response, error) {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base()+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.provider() == "anthropic" {
		req.Header.Set("x-api-key", s.APIKey)
		req.Header.Set("anthropic-version", anthropicVersion)
	} else if s.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	}
	resp, err := assistantClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %w", s.base(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, fmt.Errorf("%s answered %s: %s", s.base(), resp.Status, serviceMessage(raw))
	}
	return resp, nil
}

// serviceMessage is what a service said when it refused, which every one
// of them puts in an error object, or the start of the body if not.
func serviceMessage(raw []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &e) == nil {
		if e.Error.Message != "" {
			return e.Error.Message
		}
		if e.Message != "" {
			return e.Message
		}
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// assistantModels asks a service what models it has.
//
// Every service lists them at /models and answers with a data array of
// objects with an id, Anthropic included — the only difference is how the
// key is sent, which assistantCall settles.
func assistantModels(ctx context.Context, s assistantSettings) ([]string, error) {
	path := "/models"
	if s.provider() == "anthropic" {
		path += "?limit=1000"
	}
	raw, err := assistantCall(ctx, s, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		// Ollama's own listing, for a local service asked at /api/tags
		// by mistake; harmless to read here.
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("%s did not answer with a list of models", s.base())
	}
	var out []string
	for _, m := range list.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	for _, m := range list.Models {
		if m.Name != "" {
			out = append(out, m.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// assistantAsk sends the prompt and returns the reply as text, handing
// each piece to onDelta as it arrives.
//
// Streamed, because a model writing fifty bars for a band takes minutes,
// and a panel that says nothing for minutes looks stuck. Every service
// streams the same way at bottom — server-sent events, a "data:" line per
// piece — and differs only in what is inside the line.
func assistantAsk(ctx context.Context, s assistantSettings, system, user string, onDelta func(text, thinking string)) (string, error) {
	if s.provider() == "anthropic" {
		return anthropicAsk(ctx, s, system, user, onDelta)
	}
	resp, err := assistantOpen(ctx, s, http.MethodPost, "/chat/completions", map[string]any{
		"model":  s.model(),
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out strings.Builder
	finish := ""
	err = eachEvent(resp.Body, func(data string) error {
		if data == "[DONE]" {
			return nil
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content textContent `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			return nil
		}
		if ev.Error != nil {
			return errors.New(ev.Error.Message)
		}
		for _, c := range ev.Choices {
			text, thinking := c.Delta.Content.Text, c.Delta.Content.Thinking
			if text != "" || thinking != "" {
				out.WriteString(text)
				onDelta(text, thinking)
			}
			if c.FinishReason != "" {
				finish = c.FinishReason
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if finish == "length" {
		return "", errors.New("the reply was cut off before the music was finished; ask for fewer bars")
	}
	if out.Len() == 0 {
		return "", fmt.Errorf("%s answered with no text", s.base())
	}
	return out.String(), nil
}

// anthropicAsk is the same question in Anthropic's own shape: a system
// field, a messages array, and the answer as content blocks, streamed as
// events typed by what they carry. Thinking is left to the model, which
// on the current models is the default.
func anthropicAsk(ctx context.Context, s assistantSettings, system, user string, onDelta func(text, thinking string)) (string, error) {
	resp, err := assistantOpen(ctx, s, http.MethodPost, "/messages", map[string]any{
		"model":      s.model(),
		"max_tokens": 32000,
		"stream":     true,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out strings.Builder
	stop, why := "", ""
	err = eachEvent(resp.Body, func(data string) error {
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				StopReason  string `json:"stop_reason"`
				StopDetails *struct {
					Explanation string `json:"explanation"`
				} `json:"stop_details"`
			} `json:"delta"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			return nil
		}
		switch ev.Type {
		case "error":
			if ev.Error != nil {
				return errors.New(ev.Error.Message)
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" {
					out.WriteString(ev.Delta.Text)
					onDelta(ev.Delta.Text, "")
				}
			case "thinking_delta":
				// A summary of the reasoning, where the request asked for
				// one; empty by default, and then only the fact of it.
				onDelta("", ev.Delta.Thinking)
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				stop = ev.Delta.StopReason
			}
			if ev.Delta.StopDetails != nil {
				why = ev.Delta.StopDetails.Explanation
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	switch stop {
	case "refusal":
		if why == "" {
			why = "the model declined to answer"
		}
		return "", errors.New(why)
	case "max_tokens":
		return "", errors.New("the reply was cut off before the music was finished; ask for fewer bars")
	}
	if out.Len() == 0 {
		return "", fmt.Errorf("%s answered with no text", s.base())
	}
	return out.String(), nil
}

// textContent is a message's content as the OpenAI shape writes it: a
// string, or — from Mistral's reasoning models, and any model that thinks
// out loud in parts — a list of parts, where the text parts are the words
// and the thinking parts are the reasoning before them, itself a list of
// text parts. Read as a string the list was nothing, and the answer "did
// not answer with a reply"; and read without the thinking, the first
// minute of a hard request showed nothing at all, because the thinking is
// all there is for that minute.
type textContent struct {
	Text, Thinking string
}

func (t *textContent) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		t.Text = s
		return nil
	}
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking []struct {
			Text string `json:"text"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(b, &parts); err != nil {
		return nil // null, or something else again: no words either way
	}
	var text, thinking strings.Builder
	for _, p := range parts {
		switch p.Type {
		case "", "text":
			text.WriteString(p.Text)
		case "thinking":
			for _, th := range p.Thinking {
				thinking.WriteString(th.Text)
			}
		}
	}
	t.Text, t.Thinking = text.String(), thinking.String()
	return nil
}

// eachEvent reads a stream of server-sent events and hands the data of
// each to fn. A data line is one event; the event: lines and the blank
// lines between are the framing, and the JSON says what each one is.
func eachEvent(r io.Reader, fn func(data string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		if err := fn(strings.TrimSpace(strings.TrimPrefix(line, "data:"))); err != nil {
			return err
		}
	}
	return sc.Err()
}

/*
 * The question, and what to do with the answer
 */

// assistantSystem is what the model is told it is doing. The rules are
// about the shape of the answer, because that is what has to be read back:
// the music is its own business.
const assistantSystem = `You are a composer and arranger working inside a music notation editor. You will be given a score in ABC notation and a request. Answer with the music in ABC notation, and nothing else that is not ABC.

Write real music, as an arranger writes for players. A melody has shape and phrasing; the harmony moves; the rhythm varies — eighths and sixteenths, dotted rhythms, syncopation, triplets, rests where a player breathes. Mark the dynamics, the articulation and the phrasing throughout, and use accidentals freely where the music wants them. A bar of plain quarter notes under one dynamic for the whole piece is not an arrangement. Give each instrument its own idiomatic line: a bass line walks or holds roots, a drum kit keeps time with hi-hat, snare and kick, a solo instrument is given room when it leads.

Two kinds of answer:
- A request that adds music — the next bars, more instruments, an ending — is answered with only the new bars, which are put into the score at the cursor.
- A request that changes what is already written — the notes, the key, the meter, the tempo, the title, an instrument's name, or the whole piece reworked — is answered with the whole score as it should now be, and the first line inside the block is %%whole. The whole score replaces what is there.

The ABC the editor reads — use these, and nothing else:
- Pitches with ^ (sharp), _ (flat) and = (natural) before the note; durations as multiples of L: (C2, C/2, C3/2); rests z; chords [CEG]; ties with -; slurs with ( and ); triplets (3CDE and other tuplets (5CDEFG; grace notes {g}; broken rhythm with > and <.
- Dynamics before the note: !pp! !p! !mp! !mf! !f! !ff! !fff! !sfz! !fp!, and hairpins !crescendo(! … !crescendo)! and !diminuendo(! … !diminuendo)!.
- Articulation and ornaments before the note: . for staccato (write it as .C), !accent!, !tenuto!, !marcato!, !wedge! (staccatissimo), !fermata!, !invertedfermata!, !trill!, !trill(! … !trill)!, !mordent!, !uppermordent!, !turn!, !invertedturn!, !arpeggio! before a chord, !breath! after a phrase, !upbow! !downbow!, !+! (stopped), !snap!, !1! to !5! for fingering, !trem1! to !trem3! for a tremolo on one note.
- Lines: !ped! … !ped-up! for the piano's pedal, !8va(! … !8va)! and !8vb(! … !8vb)! for octave lines (the letters stay as printed), !courtesy! before a reminder accidental.
- A dotted slur .( … ) and an acciaccatura {/g}; a text direction as "^rit." before the note.
- A drum kit is a voice with clef=perc, written by position, with these notes: D, hi-hat with foot; E, or G, bass drum; A, low floor tom; B, high floor tom; C low tom; D snare; E low-mid tom; F high tom; G ride cymbal; A crash cymbal; B closed hi-hat, and !open!B for the open hi-hat; c splash; d china; e cowbell. Write the kick and snare with the hi-hat as chords, for example [BG,] B [BD] B for a rock beat in eighths. No accidentals on drums.

How dense real writing is — two excerpts from scores on the shelf, and a band bar:
Bach, Badinerie (flute): b d'/2 b/2 | f b/2 f/2 d f/2 d/2 | B2 F/2 B/2 d/2 B/2 | (c/2 B/2) (c/2 B/2) ^A/2 c/2 e/2 c/2 | d B b d'/2 b/2 | B2 !staccato!d !staccato!d |
Ellington, Bumpty Bump (voice): !mf!d2 c2 B d3 | c2 B d- d4 | c2 B2 A c3 | d2 ^d2 e2 B2 | =d2 c2 B2 A2 | G2 E G- G4 |
A rock bar in 4/4, L:1/8 — V:1 trumpet: !f!(e2 d/2 c/2) B2 .c .c | V:2 bass: E,2 E,2 (G,/2 A,/2) B,2 E,2 | V:3 drums: [BG,] B [BD] B [BG,] [BG,] [BD] !open!B |

Flute solos: write them as the Altès solos are written, or as Magic Malik plays. Altès (10e Solo, 3/4): (d/2 e/2 d/2 f/2 a/2 g/2 f/2 a/2 =b/2 c'/2 b/2 d'/2) | !crescendo(!(g/2 a/2 =a/2 b/2 =b/2 c'/2 _d'/2 =d'/2 e'/2 =e'/2 f'/2 !ff!!crescendo)!_g'/2) | g'3 z z2 | !emphasis!(^f' !emphasis!g' !emphasis!=f' !emphasis!e' !emphasis!d' !emphasis!c') | — long slurred runs that climb the whole range in sixteenths, chromatic passing notes, a swell to the top, then singing accented notes on the way down; elsewhere repeated staccato sixteenths, arpeggios tied into long notes, trills on the held note before a cadence. Magic Malik: jazz flute over a modal groove — syncopated sixteenth lines with accents off the beat, wide leaps answered by a scale, trills and grace notes as inflections, phrases that start late and run across the barline, a rest before the next idea, and the top of the range saved for the climax.

Rules for the answer:
- For new bars, write only the bars that were asked for, not the whole score again.
- Write one V: section per voice, in the same order and with the same V: numbers as the score you were given, even for a voice that only rests (write rests with z).
- Every voice gets exactly the same number of bars, and every bar fills the meter exactly.
- Keep the key (K:), meter (M:) and unit note length (L:) of the score; do not restate them unless they change. Write pitches as they are written for each voice's clef and transposition, as the score does.
- Use | between bars and end each voice with a barline.
- Lyrics and chord symbols only when the request asks for them.
- If the request asks for instruments the score does not have, add a voice for each after the existing ones: a V: line with a number, name="…" and clef= (treble, bass, alto, tenor, or perc for drums), then its bars. A new voice becomes a new staff. A piano is two voices, treble and bass.
- Put the ABC in a single fenced block marked abc. You may add one or two sentences before it saying what you wrote.`

// assistantRequest is the prompt the words are wrapped in.
//
// The score goes without its beaming marks. gscore writes !beamstart! and
// its kin on every beamed note so that a file keeps the beaming somebody
// chose, and a model shown a score full of them writes them back — which
// is noise in the answer and in the asking, where they double the length
// of a busy bar. The beams are the engraver's to work out.
func assistantRequest(score, prompt string, bar int) string {
	score = beamMarks.ReplaceAllString(score, "")
	return fmt.Sprintf("Here is the score so far, in ABC:\n\n%s\n\nThe cursor is at bar %d.\n\nRequest: %s",
		strings.TrimSpace(score), bar, strings.TrimSpace(prompt))
}

var beamMarks = regexp.MustCompile(`!(beamstart|beamcont|beamstop|nobeam)!`)

// abcOfReply finds the ABC in a reply: the fenced block if there is one,
// the whole reply otherwise, with the headers it needs put back where the
// model left them out.
func abcOfReply(reply, header string) (abcText, prose string) {
	prose = reply
	abcText = reply
	if i := strings.Index(reply, "```"); i >= 0 {
		rest := reply[i+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:] // the fence's own line: "```abc"
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			abcText = rest[:j]
			prose = reply[:i] + " " + rest[j+3:]
		} else {
			abcText = rest
			prose = reply[:i]
		}
	}
	// The words on one line: they are a sentence or two, and the log
	// beside the score has no room for the model's paragraphs.
	prose = strings.Join(strings.Fields(prose), " ")
	abcText = strings.TrimSpace(abcText)
	if !hasField(abcText, "K:") {
		abcText = header + "\n" + abcText
	} else if !hasField(abcText, "X:") {
		abcText = "X:1\n" + abcText
	}
	return abcText, prose
}

// hasField says whether an ABC text has a header field at the start of a
// line.
func hasField(text, field string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), field) {
			return true
		}
	}
	return false
}

// abcHeader is the header of an ABC export — everything up to and
// including the K: line — for putting in front of a reply that has none.
func abcHeader(export string) string {
	var out []string
	for _, line := range strings.Split(export, "\n") {
		out = append(out, line)
		if strings.HasPrefix(line, "K:") {
			break
		}
	}
	return strings.Join(out, "\n")
}

// placeBars puts the bars of a generated score into the score at the
// cursor: into the empty bars from there on if there are any, and new
// bars after those for the rest. "The next four bars from here" means the
// bars that are not written yet, and an empty bar at the cursor is one of
// them; a bar with music in it is not, and the new bars go after it.
//
// Voice by voice in order: the first voice of the answer is the first
// staff, and so on. A staff the answer has no voice for gets empty bars,
// so every staff stays the same length — and a voice the score has no
// staff for gets a staff, named and clefed as the answer named it. A
// request for "flute, alto sax, piano and drums" on a new score is a
// request for those staves, and a staff that has nothing on it and no
// name yet takes the name of the voice written onto it.
func placeBars(s *model.Score, gen *model.Score, cursor int) (at, count int) {
	count = gen.MeasureCount()
	bars := s.MeasureCount()
	if count == 0 || bars == 0 {
		return 0, 0
	}
	for i, from := range gen.Parts {
		if i < len(s.Parts) {
			if s.Parts[i].Name == "" && partSilent(s.Parts[i]) {
				adopt(s.Parts[i], from)
			}
			continue
		}
		p := &model.Part{
			Key: s.Parts[0].Key, Time: s.Parts[0].Time,
			Measures: make([]*model.Measure, bars),
		}
		for k := range p.Measures {
			p.Measures[k] = model.NewMeasure()
		}
		adopt(p, from)
		s.Parts = append(s.Parts, p)
	}
	if cursor < 0 || cursor >= bars {
		cursor = bars - 1
	}
	at = cursor
	if !barEmpty(s, cursor) {
		at = cursor + 1
	}
	// How many empty bars there already are from at on.
	empty := 0
	for at+empty < bars && barEmpty(s, at+empty) && empty < count {
		empty++
	}
	if more := count - empty; more > 0 {
		s.InsertMeasures(at+empty-1, more, -1)
	}
	for pi, p := range s.Parts {
		if p.Mirrors() || pi >= len(gen.Parts) {
			continue
		}
		from := gen.Parts[pi]
		for k := 0; k < count && k < len(from.Measures); k++ {
			m := p.Measures[at+k]
			m.Voices = from.Measures[k].Voices
			if len(m.Voices) == 0 {
				m.Voices = []*model.Voice{{}}
			}
		}
	}
	return at, count
}

// replaceScore puts the answer in place of the music: every staff, the
// key, the meter and the tempo, as the model rewrote them. What is kept is
// what is not music — the title and the rest of the identity, unless the
// answer gave a title of its own, the description, the tags, the pictures
// — and the cursor, held to the bars that are still there.
func replaceScore(s *model.Score, gen *model.Score) int {
	if gen.MeasureCount() == 0 {
		return 0
	}
	s.Parts = gen.Parts
	s.Groups = gen.Groups
	if gen.Tempo.BPM > 0 {
		s.Tempo = gen.Tempo
	}
	if gen.Identity.Title != "" {
		s.Identity.Title = gen.Identity.Title
	}
	if s.Cursor != nil {
		if s.Cursor.Part >= len(s.Parts) {
			s.Cursor.Part = 0
		}
		if s.Cursor.Measure >= s.MeasureCount() {
			s.Cursor.Measure = s.MeasureCount() - 1
		}
	}
	return gen.MeasureCount()
}

// adopt gives a staff the name, clef and instrument the answer wrote a
// voice with.
//
// The instrument is guessed from the name, the way Score setup's "Guess
// from the staff names" guesses it, because the ABC a model writes names
// the voice and says nothing about what plays it: every staff the
// assistant made arrived in Score setup as a piano, under a name that
// said flute. A drum kit brings its percussion clef with it. Where the
// name says nothing gscore knows, the staff keeps whatever program the
// answer gave it, or stays a piano.
func adopt(p, from *model.Part) {
	p.Name, p.Clef = from.Name, from.Clef
	if from.Instrument != 0 {
		p.Instrument = from.Instrument
	}
	if from.StaffLines != 0 {
		p.StaffLines = from.StaffLines
	}
	if id, ok := model.GuessSound(from.Name); ok {
		applySound(p, id)
	} else if from.Clef == model.Percussion {
		// A percussion clef with a name nobody recognised is still drums.
		applySound(p, "drum.group.set")
	}
}

// partSilent says whether a staff has nothing written on it at all.
func partSilent(p *model.Part) bool {
	for _, m := range p.Measures {
		for _, v := range m.Voices {
			for _, el := range v.Elements {
				if _, rest := el.(*model.Rest); !rest {
					return false
				}
			}
		}
	}
	return true
}

// barEmpty says whether nothing is written in a bar on any staff.
func barEmpty(s *model.Score, i int) bool {
	for _, p := range s.Parts {
		if p.Mirrors() || i >= len(p.Measures) {
			continue
		}
		for _, v := range p.Measures[i].Voices {
			for _, el := range v.Elements {
				if _, rest := el.(*model.Rest); !rest {
					return false
				}
				if r := el.(*model.Rest); !r.Hidden && len(v.Elements) > 1 {
					return false
				}
			}
		}
	}
	return true
}

/*
 * The routes
 */

func (a *App) assistantRoutes(mux *http.ServeMux) {
	if !assistantOn {
		return
	}
	mux.HandleFunc("/api/assistant/settings", a.handleAssistantSettings)
	mux.HandleFunc("/api/assistant/models", a.handleAssistantModels)
	// Not wrapped as the other edits are: the answer is streamed, so its
	// status goes out before the edit is made, and the undo step is
	// recorded by the handler itself.
	mux.HandleFunc("/api/assistant/compose", a.handleAssistantCompose)
}

func (a *App) handleAssistantSettings(w http.ResponseWriter, r *http.Request) {
	s := loadSettings()
	if r.Method == http.MethodPost {
		q := r.URL.Query()
		if q.Has("provider") {
			s.Assistant.Provider = strings.TrimSpace(q.Get("provider"))
		}
		if q.Has("baseURL") {
			s.Assistant.BaseURL = strings.TrimSpace(q.Get("baseURL"))
		}
		if q.Has("apiKey") {
			s.Assistant.APIKey = strings.TrimSpace(q.Get("apiKey"))
		}
		if q.Has("model") {
			s.Assistant.Model = strings.TrimSpace(q.Get("model"))
		}
		if err := saveSettings(s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]any{
		"provider": s.Assistant.provider(),
		"baseURL":  s.Assistant.BaseURL,
		"apiKey":   s.Assistant.APIKey,
		"model":    s.Assistant.Model,
		"defaults": providerBases,
	})
}

// assistantFrom is the settings a request names, over the ones kept: the
// window asks for the models of what is typed before it is saved.
func assistantFrom(r *http.Request) assistantSettings {
	s := loadSettings().Assistant
	q := r.URL.Query()
	if q.Has("provider") {
		s.Provider = strings.TrimSpace(q.Get("provider"))
	}
	if q.Has("baseURL") {
		s.BaseURL = strings.TrimSpace(q.Get("baseURL"))
	}
	if q.Has("apiKey") {
		s.APIKey = strings.TrimSpace(q.Get("apiKey"))
	}
	if q.Has("model") {
		s.Model = strings.TrimSpace(q.Get("model"))
	}
	return s
}

func (a *App) handleAssistantModels(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	models, err := assistantModels(ctx, assistantFrom(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if models == nil {
		models = []string{}
	}
	writeJSON(w, map[string]any{"models": models})
}

func (a *App) handleAssistantCompose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	prompt := strings.TrimSpace(r.URL.Query().Get("prompt"))
	if prompt == "" {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		prompt = strings.TrimSpace(string(body))
	}
	if prompt == "" {
		http.Error(w, "say what to write", http.StatusBadRequest)
		return
	}
	s := assistantFrom(r)

	// The score as it stands, read under the lock and let go of while the
	// model thinks: a request can take a minute, and the window keeps
	// drawing meanwhile.
	a.lock()
	before := a.take()
	var export bytes.Buffer
	err := abc.Export(&export, a.score)
	cursor := a.score.MeasureCount() - 1
	if a.score.Cursor != nil {
		cursor = a.score.Cursor.Measure
	}
	a.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	header := abcHeader(export.String())

	// The answer is a stream of lines, each a JSON object: progress while
	// the model writes, then done or error. A single answer at the end
	// would leave the window with nothing to say for minutes.
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)
	send := func(v any) {
		json.NewEncoder(w).Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}

	// place puts an answer — the whole of it, or as much as has arrived
	// — into the score. Always onto a fresh copy of the score as it was,
	// so that each partial answer replaces the one before it rather than
	// piling on, and the one undo step takes the lot back. The caller
	// holds the lock.
	//
	// The music is being written onto the page while the model is still
	// writing it, bar by bar, which is the feedback that says what the
	// model is doing: its music, appearing. Somebody who edits meanwhile
	// is editing a score that is about to be replaced, so the first edit
	// of theirs ends the writing instead.
	written := export.String()
	placed := 0
	place := func(text string, whole bool) (at, count int, err error) {
		var now bytes.Buffer
		if abc.Export(&now, a.score) != nil || now.String() != written {
			return 0, 0, errors.New("the score changed while the assistant was writing; ask again")
		}
		abcText, _ := abcOfReply(text, header)
		made, rerr := abc.Import(strings.NewReader(abcText))
		if made == nil || made.MeasureCount() == 0 {
			msg := "the reply was not music gscore could read"
			if rerr != nil {
				msg += ": " + rerr.Error()
			}
			return 0, 0, errors.New(msg)
		}
		fresh := before.score.Clone()
		if whole {
			count = replaceScore(fresh, made)
		} else {
			at, count = placeBars(fresh, made, cursor)
		}
		a.score = fresh
		now.Reset()
		if abc.Export(&now, a.score) == nil {
			written = now.String()
		}
		return at, count, nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	var sofar, thought strings.Builder
	last := time.Time{}
	var stopped error
	reply, err := assistantAsk(ctx, s, assistantSystem, assistantRequest(export.String(), prompt, cursor+1),
		func(delta, thinking string) {
			sofar.WriteString(delta)
			thought.WriteString(thinking)
			// A few times a second is as often as anybody can read.
			if stopped != nil || time.Since(last) < 400*time.Millisecond {
				return
			}
			last = time.Now()
			text := sofar.String()
			prog := progressOf(text)
			if th := thought.String(); th != "" {
				prog["thinking"] = tailOf(th, 160)
			}
			// The bars that are whole so far go onto the page.
			if n, _ := prog["bars"].(int); n > placed {
				if partial := wholeBars(text); partial != "" {
					a.lock()
					_, count, perr := place(partial, hasField(partial, "%%whole"))
					a.mu.Unlock()
					if perr != nil && strings.Contains(perr.Error(), "changed while") {
						stopped = perr
						cancel()
					} else if perr == nil {
						placed = count
						prog["placed"] = count
					}
				}
			}
			send(map[string]any{"progress": prog})
		})
	if stopped != nil {
		err = stopped
	}
	if err != nil {
		a.lock()
		if placed > 0 {
			a.restore(before)
		}
		a.mu.Unlock()
		send(map[string]any{"error": err.Error()})
		return
	}
	abcText, prose := abcOfReply(reply, header)
	whole := hasField(abcText, "%%whole")

	a.lock()
	defer a.mu.Unlock()
	at, count, err := place(reply, whole)
	if err != nil {
		if placed > 0 {
			a.restore(before)
		}
		send(map[string]any{"error": err.Error()})
		return
	}
	a.recordEdit(before)
	send(map[string]any{"done": map[string]any{
		"bars": count, "at": at + 1, "whole": whole, "prose": prose, "abc": abcText,
		"voices": len(a.score.Parts), "staves": len(a.score.Parts),
	}})
}

// tailOf is the last n characters of a text, from a word boundary, for
// showing where a model's reasoning has got to.
func tailOf(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	cut := s[len(s)-n:]
	if i := strings.IndexByte(cut, ' '); i >= 0 {
		cut = cut[i+1:]
	}
	return "…" + cut
}

// wholeBars is as much of an answer as ends on a barline: what has arrived
// up to the last |, which is music that can be read, with whatever came
// after it — half a bar, half a note — left for later.
func wholeBars(text string) string {
	i := strings.LastIndex(text, "|")
	if i < 0 {
		return ""
	}
	return text[:i+1] + "\n"
}

// progressOf is what can be said about an answer that is still arriving:
// how much of it there is, how many voices and bars so far, and the words
// before the music where the model has written some.
func progressOf(text string) map[string]any {
	voices, bars := 0, 0
	inMusic := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "V:") {
			voices++
			inMusic = true
			continue
		}
		if strings.HasPrefix(t, "K:") {
			inMusic = true
		}
		if inMusic && !strings.HasPrefix(t, "%") && (len(t) < 2 || t[1] != ':') {
			bars += strings.Count(t, "|")
		}
	}
	if voices > 1 {
		// Each voice writes the same bars again: the count is one voice's.
		bars = (bars + voices - 1) / voices
	}
	prose := text
	if i := strings.Index(text, "```"); i >= 0 {
		prose = text[:i]
	} else if inMusic {
		prose = ""
	}
	prose = strings.Join(strings.Fields(prose), " ")
	if len(prose) > 300 {
		prose = prose[:300] + "…"
	}
	return map[string]any{"chars": len(text), "voices": voices, "bars": bars, "prose": prose}
}
