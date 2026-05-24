package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		usage()
		return fmt.Errorf("missing arguments")
	}

	if args[1] == "--help" || args[1] == "-h" {
		usage()
		return nil
	}

	if len(args) < 3 {
		return fmt.Errorf("missing arguments")
	}

	cmd := args[1]
	subCmd := args[2]

	switch cmd + " " + subCmd {
	case "new aggregate":
		return handleNewAggregate(reorderFlags(args[3:]))
	default:
		return fmt.Errorf("unknown command: %s %s", cmd, subCmd)
	}
}

func reorderFlags(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			if !strings.Contains(args[i], "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}
	return append(flags, positional...)
}

func handleNewAggregate(args []string) error {
	fs := flag.NewFlagSet("new aggregate", flag.ExitOnError)
	fs.Usage = usage
	module := fs.String("module", "", "target module name (e.g., github.com/myorg/myapp)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	name := fs.Arg(0)

	if *module == "" {
		return fmt.Errorf("--module is required")
	}
	if name == "" {
		return fmt.Errorf("aggregate name is required")
	}

	fmt.Printf("Generating aggregate %s for module %s\n", name, *module)
	// TODO: implement generator
	return nil
}

func usage() {
	fmt.Print(`Usage: ddd <command> <subcommand> [options]

Commands:
  new aggregate <name>  Create a new aggregate scaffold

Options:
  --module string       Target module name (required)

Examples:
  ddd new aggregate Order --module github.com/myorg/myapp
`)
}
