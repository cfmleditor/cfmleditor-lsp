package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// startProfiling writes a CPU profile to CFMLEDITOR_CPUPROFILE and a heap
// profile to CFMLEDITOR_MEMPROFILE when they are set, so a subcommand can be
// profiled against a real workspace without a separate build. The returned
// function stops the CPU profile and writes the heap profile.
func startProfiling() func() {
	cpuPath := os.Getenv("CFMLEDITOR_CPUPROFILE")
	memPath := os.Getenv("CFMLEDITOR_MEMPROFILE")

	var cpu *os.File

	if cpuPath != "" {
		f, err := os.Create(cpuPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cpu profile: %v\n", err)
		} else if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "cpu profile: %v\n", err)

			_ = f.Close()
		} else {
			cpu = f
		}
	}

	return func() {
		if cpu != nil {
			pprof.StopCPUProfile()

			_ = cpu.Close()
		}

		if memPath == "" {
			return
		}

		f, err := os.Create(memPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heap profile: %v\n", err)

			return
		}

		runtime.GC()

		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "heap profile: %v\n", err)
		}

		_ = f.Close()
	}
}
