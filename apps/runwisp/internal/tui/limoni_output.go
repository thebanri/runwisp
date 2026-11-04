// SPDX-FileCopyrightText: PoppyCake, s.r.o.
// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/runwisp/runwisp/apps/runwisp/internal/model"
	"github.com/runwisp/runwisp/apps/runwisp/internal/tui/limoniview"
)

// limoniOutput switches the run output view to the Limoni prototype
// (internal/tui/limoniview) for comparison: RUNWISP_TUI_OUTPUT=limoni.
var limoniOutput = os.Getenv("RUNWISP_TUI_OUTPUT") == "limoni"

// limoniClosedMsg reports that the Limoni output screen gave the terminal back.
type limoniClosedMsg struct {
	RunID string
	Err   error
}

// openLimoniOutput hands the terminal to the Limoni screen for run. Bubble
// Tea's loop blocks until it returns, so nothing else draws meanwhile.
func (m *Model) openLimoniOutput(run *model.Run) tea.Cmd {
	id := run.ID
	return tea.Exec(limoniview.Command(m.streams.streamCtx, m.streams.client, *run), func(err error) tea.Msg {
		return limoniClosedMsg{RunID: id, Err: err}
	})
}

// handleLimoniClosed returns to the run list the reader came from.
func (m Model) handleLimoniClosed(msg limoniClosedMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.viewingRun(msg.RunID) {
		cmds = append(cmds, m.closeExecView())
	}
	if msg.Err != nil {
		cmds = append(cmds, m.dialogs.FlashError("Limoni output view: "+msg.Err.Error(), flashLong))
	}
	return m, tea.Batch(cmds...)
}
