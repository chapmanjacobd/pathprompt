# pathprompt

`pathprompt` is a small Go path selector for scripts and shell integrations. It writes
its interactive UI to stderr, prints the chosen path to stdout, stores history per
namespace, and exits with status 130 when cancelled.

It provides:

- persistent, de-duplicated XDG history with prefix navigation and inline suggestions;
- filesystem completion for relative, absolute, and `~/` paths;
- readline-style editing: arrows, Home/End, Delete, Alt-Backspace/Delete, Ctrl-A/E/U/W/K/L,
  Ctrl-Backspace/Delete/R, Ctrl-C/D;
- no shell, daemon, or background indexer.

## Install

```sh
go install github.com/chapmanjacobd/pathprompt/cmd/pathprompt@latest
```

Or build the checked-out project:

```sh
go build ./cmd/pathprompt
```

## Usage

```sh
pathprompt [options] [namespace] [last-destination]
```

`namespace` defaults to `default`, allowing different callers to keep independent
history. `last-destination` pre-populates the editor. History is written to
`$XDG_STATE_HOME/pathprompt/<namespace>.history`, or
`~/.local/state/pathprompt/<namespace>.history` when `XDG_STATE_HOME` is unset.

Use `-tf` or `--type file` to complete files only, and `-td` or `--type directory`
to complete directories only. The `-t` form is also accepted with `file` or
`directory`.

While editing, Tab completes paths; a second Tab lists ambiguous candidates. Up and Down
navigate history entries matching the initial text. The dim suffix is the newest matching
history entry; Right accepts it. Ctrl-R cycles backward through history entries containing
the initial text.
