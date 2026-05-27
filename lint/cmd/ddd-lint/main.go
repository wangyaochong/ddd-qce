package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/ddd-qce/core/lint/crossdomain"
	"github.com/ddd-qce/core/lint/implimport"
	"github.com/ddd-qce/core/lint/publicleak"
)

func main() {
	multichecker.Main(
		crossdomain.Analyzer,
		publicleak.Analyzer,
		implimport.Analyzer,
	)
}
