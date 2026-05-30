package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/ddd-qce/core/lint/crossdomain"
	"github.com/ddd-qce/core/lint/implimport"
	"github.com/ddd-qce/core/lint/publicleak"
	"github.com/ddd-qce/core/lint/wirecomplexity"
	"github.com/ddd-qce/core/lint/wireimport"
	"github.com/ddd-qce/core/lint/wirelogic"
)

func main() {
	multichecker.Main(
		crossdomain.Analyzer,
		publicleak.Analyzer,
		implimport.Analyzer,
		wirecomplexity.Analyzer,
		wireimport.Analyzer,
		wirelogic.Analyzer,
	)
}
