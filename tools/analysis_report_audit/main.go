package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"hissebot/internal/ta/reportaudit"
)

func main() {
	path := flag.String("path", "", "analysis.json path")
	spotOnly := flag.Bool("spot-only", true, "reject short trade plans for spot equity reports")
	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, "-path is required")
		os.Exit(2)
	}

	report, err := reportaudit.ValidateFile(*path, reportaudit.Options{SpotOnly: *spotOnly})
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit failed: %v\n", err)
		os.Exit(2)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(2)
	}
	fmt.Println(string(encoded))
	if report.Status != "pass" {
		os.Exit(1)
	}
}
