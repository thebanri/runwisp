// SPDX-FileCopyrightText: PoppyCake, s.r.o.
// SPDX-License-Identifier: GPL-3.0-or-later

package limoniview

import (
	"testing"

	"github.com/thebanri/limoni/core/buffer"
	lcell "github.com/thebanri/limoni/core/cell"
	"github.com/thebanri/limoni/core/terminal"

	"github.com/runwisp/runwisp/apps/runwisp/internal/model"
)

// BenchmarkOutputViewLimoni is the screen following a task that prints a
// coloured line per frame: append a line, draw the frame, and diff it into
// the bytes a terminal is sent — what Limoni's Terminal does each frame.
// The bytes are the same truecolour ANSI a real frame writes.
func BenchmarkOutputViewLimoni(b *testing.B) {
	lines := BenchLines()
	u := newUI(model.Run{ID: "01J000000000000000000000AB", TaskName: "firehose"})
	for n := range 1000 {
		u.log.add(int64(n), "stdout", lines[n%len(lines)])
	}
	area := lcell.NewRect(0, 0, BenchWidth, BenchHeight)
	front, back := buffer.NewBuffer(area), buffer.NewBuffer(area)
	f := terminal.NewFrame(nil, terminal.NewFocusManager())
	var out []byte
	frame := func() {
		front.Clear()
		f.Buffer = front
		f.Reset()
		u.frame(f, nil)
		var err error
		if out, err = buffer.Diff(front, back, out[:0], true, true); err != nil {
			b.Fatal(err)
		}
	}
	frame()
	bytes := 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := 1000 + i
		u.log.add(int64(n), "stdout", lines[n%len(lines)])
		frame()
		bytes += len(out)
	}
	b.ReportMetric(float64(bytes)/float64(b.N), "bytes/frame")
}
