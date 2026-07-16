package main

import (
	"fmt"
	"os"

	"cursor/internal/skillsconfig"
)

func main() {
	workspace := ""
	if len(os.Args) > 1 {
		workspace = os.Args[1]
	}
	skills, err := skillsconfig.Sync(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync skills failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(skillsconfig.DescribeResolved(skills))
}
