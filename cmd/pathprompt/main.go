package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/chapmanjacobd/pathprompt/internal/complete"
	"github.com/chapmanjacobd/pathprompt/internal/editor"
	"github.com/chapmanjacobd/pathprompt/internal/history"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in *os.File, out io.Writer, errOut io.Writer) int {
	flags := flag.NewFlagSet("pathprompt", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.Usage = func() {
		fmt.Fprintln(errOut, "Usage: pathprompt [namespace] [last-destination]")
		fmt.Fprintln(errOut, "Print one path selected interactively, or q when cancelled.")
	}
	showVersion := flags.Bool("version", false, "print version")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(out, version)
		return 0
	}

	positional := flags.Args()
	if len(positional) > 2 {
		flags.Usage()
		return 2
	}

	namespace := "default"
	if len(positional) > 0 && positional[0] != "" {
		namespace = positional[0]
	}
	initial := ""
	if len(positional) > 1 {
		initial = positional[1]
	}

	store, err := history.Open(history.FilePath(namespace), history.DefaultLimit)
	if err != nil {
		fmt.Fprintf(errOut, "pathprompt: load history: %v\n", err)
		return 1
	}

	value, err := editor.Read(in, errOut, editor.Config{
		Prompt:    "PATH> ",
		Initial:   initial,
		History:   store,
		Completer: complete.New(os.Getenv("HOME")),
	})
	if errors.Is(err, editor.ErrCancelled) {
		return 130
	}
	if err != nil {
		fmt.Fprintf(errOut, "pathprompt: %v\n", err)
		return 1
	}

	if err := store.Add(value); err != nil {
		fmt.Fprintf(errOut, "pathprompt: save history: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, value)
	return 0
}
