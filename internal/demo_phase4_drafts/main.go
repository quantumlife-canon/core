//go:build ignore

package main

import (
	"fmt"
	"os"

	"quantumlife/internal/demo_phase4_drafts"
)

func main() {
	result := demo_phase4_drafts.RunDemo()
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "Demo failed: %v\n", result.Err)
		os.Exit(1)
	}
	fmt.Print(result.Output)
}
