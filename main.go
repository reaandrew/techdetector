package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"os"
	"runtime/pprof"
)

const (
	MaxWorkers        = 10
	MaxFileWorkers    = 10
	CloneBaseDir      = "/tmp/techdetector" // You can make this configurable if needed
	XlsxSummaryReport = "summary_report.xlsx"
	XlsxSQLiteDB      = "findings.db"
)

var Version string

func main() {
	logrus.SetOutput(os.Stderr)
	logrus.StandardLogger().SetLevel(logrus.ErrorLevel)

	// Define flags for profiling.
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to file")
	memprofile := flag.String("memprofile", "", "write memory profile to file")
	flag.Parse()

	// Start CPU profiling if requested.
	var cpuFile *os.File
	if *cpuprofile != "" {
		var err error
		cpuFile, err = os.Create(*cpuprofile)
		if err != nil {
			log.Fatalf("could not create CPU profile: %v", err)
		}
		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			log.Fatalf("could not start CPU profile: %v", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			cpuFile.Close()
		}()
	}

	log.Println("Starting in CLI mode")
	cli := &Cli{}

	// Execute CLI function safely (without premature exit)
	if err := cli.Execute(); err != nil {
		log.Errorf("Error executing command: %v", err)
	}

	// Write a memory profile if requested.
	if *memprofile != "" {
		memFile, err := os.Create(*memprofile)
		if err != nil {
			log.Fatalf("could not create memory profile: %v", err)
		}
		defer memFile.Close()
		if err := pprof.WriteHeapProfile(memFile); err != nil {
			log.Fatalf("could not write memory profile: %v", err)
		}
	}

	fmt.Println("Profiling completed")
}
