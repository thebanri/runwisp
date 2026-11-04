// SPDX-FileCopyrightText: PoppyCake, s.r.o.
// SPDX-License-Identifier: GPL-3.0-or-later

// Package limoniview is a prototype of the run output view drawn with Limoni
// (github.com/thebanri/limoni) instead of Bubble Tea. It takes the whole
// terminal for one run: the tail page, then the live line stream, with the
// task's own ANSI colours, live progress regions, line numbers, follow mode,
// keyboard and wheel scrolling. The Bubble Tea TUI hands the terminal over
// with tea.Exec and gets it back when the screen closes.
package limoniview

import (
	"context"
	"image/color"
	"io"
	"strconv"
	"sync"

	"github.com/thebanri/limoni"
	"github.com/thebanri/limoni/widgets"

	"github.com/runwisp/runwisp/apps/runwisp/internal/apiclient"
	"github.com/runwisp/runwisp/apps/runwisp/internal/model"
	"github.com/runwisp/runwisp/apps/runwisp/internal/tui/uikit"
)

const (
	tailLines = 1000    // the page shown at once, as the Bubble Tea view loads
	maxLines  = 100_000 // the Bubble Tea view's MaxLogLines
)

// Command returns the screen for run as a tea.ExecCommand: Bubble Tea
// releases the terminal, Run draws until the reader presses q or Esc, and
// Bubble Tea takes the terminal back. The standard streams are the process's
// own, which is what Limoni opens.
func Command(ctx context.Context, client *apiclient.Client, run model.Run) *Screen {
	return &Screen{ctx: ctx, client: client, run: run}
}

// Screen is one run's output view.
type Screen struct {
	ctx    context.Context
	client *apiclient.Client
	run    model.Run
}

func (*Screen) SetStdin(io.Reader)  {}
func (*Screen) SetStdout(io.Writer) {}
func (*Screen) SetStderr(io.Writer) {}

// Run shows the screen until it is closed or ctx ends.
func (s *Screen) Run() error {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	u := newUI(s.run)
	go u.log.follow(ctx, s.client, s.run.ID, limoni.Wakeup)
	err := limoni.RunWithContext(ctx, u.frame, limoni.WithTitle("RunWisp · "+s.run.TaskName))
	if err == context.Canceled && s.ctx.Err() == nil {
		err = nil
	}
	return err
}

// ui draws the screen; the frame function is all of it, so a benchmark can
// drive the same code a terminal does.
type ui struct {
	log    *store
	state  *widgets.LogViewState
	view   *widgets.LogView
	head   string
	status statusText

	bar, title limoni.Style
	keysW      int
}

const keys = "↑↓ PgUp PgDn scroll  ←→ pan  q back"

func newUI(run model.Run) *ui {
	log := &store{}
	st := widgets.NewLogViewState()
	view := &widgets.LogView{ID: "output", Label: "Run output", Source: log, State: st, LineNumbers: true}
	view.Style = cell(uikit.ColorText, uikit.ColorBg)
	view.GutterStyle = cell(uikit.ColorTextMuted, uikit.ColorBg)
	view.LevelStyles[widgets.LevelInfo] = view.Style
	view.LevelStyles[widgets.LevelError] = cell(uikit.ColorError, uikit.ColorBg)
	view.LevelStyles[widgets.LevelTrace] = cell(uikit.ColorTextMuted, uikit.ColorBg)
	view.LevelStyles[widgets.LevelTrace].Modifier |= limoni.ModifierItalic
	u := &ui{log: log, state: st, view: view, head: run.TaskName + "  " + run.ID, keysW: len([]rune(keys))}
	u.bar = cell(uikit.ColorTextDim, uikit.ColorBg)
	u.title = cell(uikit.ColorPrimary, uikit.ColorBg)
	u.title.Modifier |= limoni.ModifierBold
	return u
}

// frame handles ev and draws one frame; false closes the screen.
func (u *ui) frame(f *limoni.Frame, ev *limoni.Event) bool {
	log, st := u.log, u.state
	log.mu.Lock()
	defer log.mu.Unlock()
	if ev != nil && ev.Type == limoni.EventKey {
		k := ev.Key
		if k.Type == limoni.KeyEsc || (k.Type == limoni.KeyRune && k.Ch == 'q') {
			return false
		}
		st.HandleKey(k, log.Len())
	}
	area := f.Area()
	rows := limoni.SplitVertical(area, limoni.Fixed(1), limoni.Fill(), limoni.Fixed(1))
	fill(f, rows[0], u.bar)
	f.Text(int(rows[0].X)+1, int(rows[0].Y), u.head, u.title)
	f.RenderWidget(u.view, rows[1])

	fill(f, rows[2], u.bar)
	state := "paused, End follows"
	switch {
	case log.err != nil:
		state = "stream error"
	case log.done:
		state = "ended"
	case len(log.lines) == 0:
		state = "waiting for output"
	case st.Follow:
		state = "following"
	}
	f.Text(int(rows[2].X)+1, int(rows[2].Y), u.status.get(len(log.lines), state), u.bar)
	f.Text(int(rows[2].X+rows[2].Width)-u.keysW-1, int(rows[2].Y), keys, u.bar)
	return true
}

func fill(f *limoni.Frame, r limoni.Rect, st limoni.Style) {
	f.TextFit(int(r.X), int(r.Y), int(r.Width), "", st)
}

// statusText is the footer's left side; rebuilt only when it changes, so a
// frame allocates nothing.
type statusText struct {
	lines int
	state string
	text  string
}

func (t *statusText) get(lines int, state string) string {
	if t.text == "" || t.lines != lines || t.state != state {
		t.lines, t.state = lines, state
		t.text = strconv.Itoa(lines) + " lines · " + state
	}
	return t.text
}

func cell(fg, bg color.Color) limoni.Style {
	return limoni.Style{Fg: rgb(fg), Bg: rgb(bg)}
}

func rgb(c color.Color) limoni.Color {
	r, g, b, _ := c.RGBA()
	return limoni.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// entry is one committed line of output.
type entry struct {
	n      int64
	stream string
	text   string
}

// store holds the lines the stream delivered and the live progress regions,
// and is the LogView's source. The frame function holds mu while it draws.
type store struct {
	mu      sync.Mutex
	lines   []entry
	regions map[string][]string
	tail    []entry // regions flattened, drawn after the committed lines
	done    bool
	err     error
}

func (s *store) Len() int { return len(s.lines) + len(s.tail) }

func (s *store) at(i int) *entry {
	if i < len(s.lines) {
		return &s.lines[i]
	}
	return &s.tail[i-len(s.lines)]
}

func (s *store) Line(i int) string { return s.at(i).text }

func (s *store) LineNumber(i int) int { return int(s.at(i).n) + 1 }

func (s *store) Level(i int) widgets.LogLevel {
	switch s.at(i).stream {
	case "stderr":
		return widgets.LevelError
	case "system":
		return widgets.LevelTrace
	}
	return widgets.LevelInfo
}

func (s *store) add(n int64, stream, text string) {
	if len(s.lines) >= maxLines {
		drop := maxLines / 10
		s.lines = append(s.lines[:0], s.lines[drop:]...)
	}
	s.lines = append(s.lines, entry{n: n, stream: stream, text: text})
	s.flatten() // committed lines end a region's frame on that stream
}

func (s *store) setRegion(stream string, rows []string) {
	if s.regions == nil {
		s.regions = map[string][]string{}
	}
	if len(rows) == 0 {
		delete(s.regions, stream)
	} else {
		s.regions[stream] = rows
	}
	s.flatten()
}

func (s *store) flatten() {
	s.tail = s.tail[:0]
	next := int64(0)
	if n := len(s.lines); n > 0 {
		next = s.lines[n-1].n + 1
	}
	for _, stream := range [...]string{"stdout", "stderr"} {
		for _, row := range s.regions[stream] {
			s.tail = append(s.tail, entry{n: next, stream: stream, text: row})
			next++
		}
	}
}

func (s *store) evictBelow(first int64) {
	k := 0
	for k < len(s.lines) && s.lines[k].n < first {
		k++
	}
	s.lines = append(s.lines[:0], s.lines[k:]...)
}

// follow loads the tail page, then streams the lines after it, the way the
// Bubble Tea view does, calling wake after each change.
func (s *store) follow(ctx context.Context, client *apiclient.Client, runID string, wake func()) {
	finish := func(err error) {
		s.mu.Lock()
		s.done, s.err = err == nil, err
		s.mu.Unlock()
		wake()
	}
	page, err := client.GetLogPage(ctx, runID, -tailLines, tailLines)
	s.mu.Lock()
	for _, l := range page.Lines {
		s.add(l.N, l.Stream, l.Text)
	}
	s.mu.Unlock()
	wake()
	if err == nil && page.Finalized {
		finish(nil)
		return
	}
	from := int64(-tailLines)
	if n := len(page.Lines); n > 0 {
		from = page.Lines[n-1].N + 1
	}
	ch, err := client.StreamLogLines(ctx, runID, from)
	if err != nil {
		finish(err)
		return
	}
	for msg := range ch {
		s.mu.Lock()
		switch msg.Kind {
		case apiclient.LogStreamMsgKindLine:
			s.add(msg.Line.N, msg.Line.Stream, msg.Line.Text)
		case apiclient.LogStreamMsgKindRegion:
			s.setRegion(msg.Region.Stream, msg.Region.Rows)
		case apiclient.LogStreamMsgKindRotated:
			s.evictBelow(msg.Rotated.FirstAvailable)
		case apiclient.LogStreamMsgKindDropped:
			s.add(msg.Dropped.After, "system", "… "+strconv.FormatInt(msg.Dropped.Count, 10)+" lines dropped")
		case apiclient.LogStreamMsgKindDone:
			s.done = true
		case apiclient.LogStreamMsgKindErr:
			s.err = msg.ErrValue
		}
		s.mu.Unlock()
		wake()
	}
}
