package main

import (
	"fmt"
	"os"

	"github.com/ahmedmissouri/noted/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "noted:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = args
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	fmt.Printf("noted: config ok (data dir %q, listen %q); nothing to serve yet\n",
		cfg.DataDir, cfg.ListenAddr)
	return nil
}
