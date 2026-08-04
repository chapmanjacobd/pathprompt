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
		fmt.Fprintln(errOut, "Usage: pathprompt [options] [namespace] [last-destination]")
		fmt.Fprintln(errOut, "Print one path selected interactively, or q when cancelled.")
	}
	showVersion := flags.Bool("version", false, "print version")
	var typeFlag pathTypeFlag
	flags.Var(&typeFlag, "type", "complete only files or directories")
	flags.Var(&typeFlag, "t", "complete only files or directories")
	filesOnly := flags.Bool("tf", false, "complete files only")
	directoriesOnly := flags.Bool("td", false, "complete directories only")
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
	pathType, err := resolvePathType(typeFlag, *filesOnly, *directoriesOnly)
	if err != nil {
		fmt.Fprintf(errOut, "pathprompt: %v\n", err)
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
		Completer: complete.New(os.Getenv("HOME"), pathType),
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

type pathTypeFlag struct {
	value complete.PathType
	set   bool
}

func (f *pathTypeFlag) String() string {
	return string(f.value)
}

func (f *pathTypeFlag) Set(value string) error {
	pathType, err := complete.ParseType(value)
	if err != nil {
		return err
	}
	f.value = pathType
	f.set = true
	return nil
}

func resolvePathType(typeFlag pathTypeFlag, filesOnly, directoriesOnly bool) (complete.PathType, error) {
	var selected complete.PathType
	if filesOnly {
		selected = complete.TypeFile
	}
	if directoriesOnly {
		if selected != "" {
			return complete.TypeAny, errors.New("cannot combine -tf and -td")
		}
		selected = complete.TypeDirectory
	}
	if typeFlag.set {
		if selected != "" && selected != typeFlag.value {
			return complete.TypeAny, errors.New("cannot combine --type with a different -tf or -td filter")
		}
		selected = typeFlag.value
	}
	return selected, nil
}
