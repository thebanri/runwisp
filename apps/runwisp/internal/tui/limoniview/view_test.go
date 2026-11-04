// SPDX-FileCopyrightText: PoppyCake, s.r.o.
// SPDX-License-Identifier: GPL-3.0-or-later

package limoniview

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/thebanri/limoni/widgets"
)

func lines(s *store) []string {
	out := make([]string, s.Len())
	for i := range out {
		out[i] = s.Line(i)
	}
	return out
}

func TestStoreDrawsRegionsAfterTheCommittedLines(t *testing.T) {
	s := &store{}
	s.add(0, "stdout", "one")
	s.add(1, "stderr", "two")
	s.setRegion("stdout", []string{"[##..] 50%"})
	if got := lines(s); len(got) != 3 || got[2] != "[##..] 50%" {
		t.Fatalf("lines %q", got)
	}
	if s.LineNumber(2) != 3 || s.Level(1) != widgets.LevelError || s.Level(0) != widgets.LevelInfo {
		t.Fatalf("number %d, levels %v %v", s.LineNumber(2), s.Level(1), s.Level(0))
	}
	// The region settles into a committed line and its overlay clears.
	s.setRegion("stdout", nil)
	s.add(2, "stdout", "[####] 100%")
	if got := lines(s); len(got) != 3 || got[2] != "[####] 100%" {
		t.Fatalf("after settling %q", got)
	}
}

func TestStoreEvictsRotatedLinesAndCaps(t *testing.T) {
	s := &store{}
	for n := int64(0); n < 10; n++ {
		s.add(n, "stdout", "x")
	}
	s.evictBelow(4)
	if s.Len() != 6 || s.LineNumber(0) != 5 {
		t.Fatalf("after rotation: %d lines, first %d", s.Len(), s.LineNumber(0))
	}
	s = &store{}
	for n := int64(0); n <= maxLines; n++ {
		s.add(n, "stdout", "x")
	}
	if s.Len() > maxLines || s.LineNumber(s.Len()-1) != maxLines+1 {
		t.Fatalf("cap: %d lines, last %d", s.Len(), s.LineNumber(s.Len()-1))
	}
}

// A burst of wakes becomes one now and one at the end of the interval.
func TestThrottleCoalescesAndKeepsTheLast(t *testing.T) {
	var n atomic.Int32
	th := &throttle{every: 50 * time.Millisecond, wake: func() { n.Add(1) }}
	for range 100 {
		th.kick()
	}
	if got := n.Load(); got != 1 {
		t.Fatalf("%d wakes at once, want 1", got)
	}
	time.Sleep(120 * time.Millisecond)
	if got := n.Load(); got != 2 {
		t.Fatalf("%d wakes after the interval, want 2", got)
	}
}
