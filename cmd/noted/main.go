// Command noted is the note server. main stays thin: parse arguments,
// wire dependencies, run. Everything real lives under internal/.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "noted:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = args
	fmt.Println("noted: nothing to serve yet; this binary grows during phase 0")
	return nil
}
