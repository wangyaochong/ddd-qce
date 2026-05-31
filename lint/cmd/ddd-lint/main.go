package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/packages"

	"github.com/ddd-qce/core/lint/crossdomain"
	"github.com/ddd-qce/core/lint/implimport"
	"github.com/ddd-qce/core/lint/infraimplement"
	"github.com/ddd-qce/core/lint/publicleak"
	"github.com/ddd-qce/core/lint/wirecomplexity"
	"github.com/ddd-qce/core/lint/wireimport"
	"github.com/ddd-qce/core/lint/wirelogic"
)

var analyzers = []*analysis.Analyzer{
	crossdomain.Analyzer,
	publicleak.Analyzer,
	implimport.Analyzer,
	wirecomplexity.Analyzer,
	wireimport.Analyzer,
	wirelogic.Analyzer,
	infraimplement.Analyzer,
}

func main() {
	start := time.Now()

	// Parse args to get package patterns
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		// Let multichecker handle the usage message
		multichecker.Main(analyzers...)
		return
	}

	// Load packages to collect statistics
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "ddd-lint: loading packages...\n")

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles,
	}
	loadedPkgs, err := packages.Load(cfg, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ddd-lint: error loading packages: %v\n", err)
		os.Exit(1)
	}

	// Collect unique directories and files
	dirSet := make(map[string]bool)
	fileCount := 0

	for _, pkg := range loadedPkgs {
		if pkg.PkgPath == "" {
			continue
		}
		if !dirSet[pkg.PkgPath] {
			dirSet[pkg.PkgPath] = true
			fileCount += len(pkg.GoFiles)
		}
	}

	// Sort directories for consistent output
	dirs := make([]string, 0, len(dirSet))
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	// Print directory list
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "ddd-lint: checking packages...\n")
	for _, dir := range dirs {
		fmt.Fprintf(os.Stderr, "  %s\n", dir)
	}
	fmt.Fprintf(os.Stderr, "\n")

	// Record time after loading
	loadDuration := time.Since(start)

	// Run the actual analysis
	// multichecker.Main will call os.Exit, so we can't add code after it
	// Instead, we'll print the summary before running
	fmt.Fprintf(os.Stderr, "────────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "Pre-analysis:\n")
	fmt.Fprintf(os.Stderr, "  Directories: %d\n", len(dirs))
	fmt.Fprintf(os.Stderr, "  Files:       %d\n", fileCount)
	fmt.Fprintf(os.Stderr, "  Load time:   %v\n", loadDuration.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "────────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "\n")

	// Run multichecker.Main (this will call os.Exit)
	multichecker.Main(analyzers...)
}
