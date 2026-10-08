#!/usr/bin/env bash
# Runs `unresolved --json --candidates` over named scans and writes the
# candidate lists (resolution-candidates/ by default). Each scan is
#
#   name=dir[,dir...]
#
# and resolves under the .clif.json governing its first directory, as
# `unresolved` always does. The JSON of each run is kept in $RUNS (default
# target/resolution/latest); BASELINE names an earlier run's directory to
# compare with finding by finding.
#
#   make resolution-report CORPUS="masa=/src/MasaCMS wheels=/src/cfwheels,/src/cfwheels/vendor/wheels"
#   make resolution-report CORPUS="..." BASELINE=target/resolution/before LISTS=
#
# An empty LISTS (OUT here) compares without rewriting the lists, which is the cheap check
# to run after a change; the lists are for when a fresh picture is wanted.
set -euo pipefail

: "${OUT=resolution-candidates}"
: "${RUNS:=target/resolution/latest}"
: "${BASELINE:=}"
: "${JOBS:=4}"

if [ "$#" -eq 0 ]; then
	echo "usage: $0 name=dir[,dir...] ..." >&2
	exit 2
fi

mkdir -p target/resolution "$RUNS"
bin=target/resolution/clif
go build -o "$bin" ./cmd/clif

args=()
pids=()

# The scans run JOBS at a time; each one's own summary goes to its .log.
for spec in "$@"; do
	name=${spec%%=*}
	dirs=${spec#*=}

	if [ "$name" = "$spec" ] || [ -z "$dirs" ]; then
		echo "scan $spec is not name=dir[,dir...]" >&2
		exit 2
	fi

	IFS=, read -r -a roots <<<"$dirs"
	echo "scanning $name ..." >&2
	"$bin" unresolved --json --candidates "${roots[@]}" >"$RUNS/$name.json" 2>"$RUNS/$name.log" &
	pids+=($!)
	args+=("$name=$RUNS/$name.json@${roots[0]%/}")

	if [ "${#pids[@]}" -ge "$JOBS" ]; then
		wait "${pids[0]}"
		pids=("${pids[@]:1}")
	fi
done

for pid in "${pids[@]}"; do
	wait "$pid"
done

flags=()
[ -n "$BASELINE" ] && flags+=(-baseline "$BASELINE")

if [ -n "$OUT" ]; then
	go run scripts/candidates_report.go "${flags[@]}" -out "$OUT" "${args[@]}"
else
	go run scripts/candidates_report.go "${flags[@]}" -out "$RUNS/report" "${args[@]}"
fi
