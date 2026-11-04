// SPDX-FileCopyrightText: PoppyCake, s.r.o.
// SPDX-License-Identifier: GPL-3.0-or-later

package limoniview

import "strconv"

// The terminal both output-view benchmarks draw on.
const (
	BenchWidth  = 140
	BenchHeight = 45
)

// BenchLines returns the coloured lines both output-view benchmarks stream,
// built once so the benchmarks measure drawing, not formatting.
func BenchLines() []string {
	lines := make([]string, 4096)
	for i := range lines {
		s := strconv.Itoa(i)
		lines[i] = "\x1b[32mINFO\x1b[0m request " + s + " \x1b[1;38;5;208mserved\x1b[22;39m status=200 path=/api/v1/items/" + s
	}
	return lines
}
