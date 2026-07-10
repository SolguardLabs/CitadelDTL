package main

import (
	"fmt"
	"os"

	"citadeldtl/src/report"
	"citadeldtl/src/scenario"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: citadeldtl run <fixture.json>")
			os.Exit(2)
		}
		result, err := scenario.RunFile(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if err := report.WriteJSON(os.Stdout, result); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "list":
		for _, name := range scenario.BuiltinScenarios() {
			fmt.Println(name)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  citadeldtl run <fixture.json>")
	fmt.Fprintln(os.Stderr, "  citadeldtl list")
}
