#!/bin/bash
# Compares the run output view in Bubble Tea and in the Limoni prototype by
# measuring the `runwisp tui` process (not the daemon) while it follows the
# firehose task in runwisp.toml here (~1,000 coloured lines a second): CPU
# over 20 s after a 5 s warm-up, RSS, and the bytes it writes to its pty
# (tmux pipe-pane). Three rounds, alternating. Needs tmux and a built binary:
#
#   go build -o /tmp/runwisp ./apps/runwisp/cmd/runwisp
#   apps/runwisp/internal/tui/limoniview/measure/measure.sh /tmp/runwisp
set -eu
B=${1:?path to a runwisp binary}
HERE=$(cd "$(dirname "$0")" && pwd)
W=$(mktemp -d); SOCK=$W/rw.sock
"$B" daemon -c "$HERE/runwisp.toml" --data "$W/data" --socket "$SOCK" --port 19477 >"$W/daemon.log" 2>&1 &
DAEMON=$!; trap 'tmux kill-session -t rwmeasure 2>/dev/null; kill $DAEMON; rm -rf "$W"' EXIT
sleep 4
ticks() { awk '{print $14+$15}' "/proc/$1/stat"; } # utime+stime, in clock ticks
hz=$(getconf CLK_TCK)
for round in 1 2 3; do
  for v in bubbletea limoni; do
    env=""; [ $v = limoni ] && env="RUNWISP_TUI_OUTPUT=limoni"
    tmux new-session -d -s rwmeasure -x 140 -y 45 "env $env TERM=xterm-256color COLORTERM=truecolor $B tui --socket $SOCK"
    sleep 3
    tmux send-keys -t rwmeasure Down; sleep 0.4; tmux send-keys -t rwmeasure Enter; sleep 1
    tmux send-keys -t rwmeasure r; sleep 0.8; tmux send-keys -t rwmeasure y # run now; the output view opens
    sleep 5
    pid=$(tmux display -p -t rwmeasure '#{pane_pid}')
    tmux pipe-pane -o -t rwmeasure "cat > $W/out"
    t0=$(ticks "$pid"); sleep 20; t1=$(ticks "$pid")
    rss=$(awk '/VmRSS/{print $2}' "/proc/$pid/status")
    tmux pipe-pane -t rwmeasure
    frames=$(awk 'BEGIN{RS="\033\\[\\?2026h"} END{print NR-1}' "$W/out") # one synchronized update per frame
    cpu=$(awk -v t="$((t1 - t0))" -v hz="$hz" 'BEGIN{printf "%.1f", t*100/hz/20}')
    printf '%-9s round %d  cpu %5s%%  rss %6d KB  bytes %8d  frames %d\n' \
      $v $round "$cpu" "$rss" "$(stat -c %s "$W/out")" "$frames"
    tmux kill-session -t rwmeasure
    sleep 22 # let the run finish before the next one starts
  done
done
