// SPDX-FileCopyrightText: PoppyCake, s.r.o.
// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/runwisp/runwisp/apps/runwisp/internal/model"
	"github.com/runwisp/runwisp/apps/runwisp/internal/tui/limoniview"
	"github.com/runwisp/runwisp/apps/runwisp/internal/tui/views/execlist"
)

// BenchmarkOutputViewBubbleTea is the run output view following a task that
// prints a coloured line per frame: append a line, then View(). It measures
// building the view string only; Bubble Tea's renderer, which diffs that
// string against the screen and writes the bytes, runs after it and is not
// included. Compare with limoniview's BenchmarkOutputViewLimoni, which does
// include its diff.
func BenchmarkOutputViewBubbleTea(b *testing.B) {
	lines := limoniview.BenchLines()
	m := newTestModel([]model.Task{{Name: "firehose"}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: limoniview.BenchWidth, Height: limoniview.BenchHeight})
	m = nm.(Model)
	run := model.Run{ID: "01J000000000000000000000AB", TaskName: "firehose", Status: model.PhaseRunning}
	ev := execlist.NewExecView(&run)
	m.execView = &ev
	m.execView.SetSize(m.mainSize())
	for n := range 1000 {
		m.execView.Pane.AppendLogLine(int64(n), "stdout", lines[n%len(lines)], 0)
	}
	_ = m.View()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := 1000 + i
		m.execView.Pane.AppendLogLine(int64(n), "stdout", lines[n%len(lines)], 0)
		_ = m.View()
	}
}
