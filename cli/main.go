package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	// Commands that don't need config.
	switch cmd {
	case "init":
		cmdInit(os.Args[2:])
		return
	case "help", "-h", "--help":
		usage()
		return
	}

	// Load config for all other commands.
	var err error
	cfg, err = loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "board":
		cmdBoard(os.Args[2:])
	case "create", "new":
		cmdCreate(os.Args[2:])
	case "ls", "list":
		cmdList(os.Args[2:])
	case "mv", "move":
		cmdMove(os.Args[2:])
	case "start":
		cmdStart(os.Args[2:])
	case "done":
		cmdDone(os.Args[2:])
	case "close":
		cmdClose(os.Args[2:])
	case "edit":
		cmdEdit(os.Args[2:])
	case "show", "cat":
		cmdShow(os.Args[2:])
	case "open":
		cmdOpen(os.Args[2:])
	case "epic":
		cmdEpic(os.Args[2:])
	case "triage":
		cmdTriage(os.Args[2:])
	case "grep", "search":
		cmdGrep(os.Args[2:])
	case "scan":
		cmdScan(os.Args[2:])
	case "regenerate", "regen":
		cmdRegenerate(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`tb — task board CLI

Usage:
  tb init [path] [--board-path=board] [--prefix=PR]                     Initialize board
  tb board [--json]                                                      Print board status (or JSON snapshot)
  tb create "Title" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--parent ID] [--epic]
  tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [--parent ID] [-n N]
        [--status backlog|in-progress|done|archive|active|all] [--json]
  tb mv <ID> <status>                                                    Move task (status: backlog|in-progress|done|archive)
  tb start <ID>                                                          Start working
  tb done <ID>                                                           Mark done
  tb edit <ID> [-p P0] [-T type] [-s M] [-m module] [-t tags] [-a claude|codex] [--agent-status queued|running|success|failed|cancelled]
  tb close <ID>                                                          Archive task
  tb show <ID> [--json]                                                  Print task content (or {metadata, body} JSON)
  tb open <ID>                                                           Open in default editor
  tb epic <ID>                                                           Show epic progress
  tb triage                                                              Find tasks needing grooming
  tb grep <pattern> [--status backlog|in-progress|done|archive|active|all] [-s] [-l]   Search tasks by regex
  tb scan [--apply] [--path dir]                                         Find untagged TODOs
  tb regenerate                                                          Regenerate BOARD.md

Commands:
  init              Initialize board structure (creates .tb.yaml in project root)
  board             Print board status to stdout (same format as BOARD.md)
  create, new       Create a new task
  ls, list          List and filter tasks
  mv, move          Move task between statuses
  start             Move task to in-progress
  done              Move task to done
  edit              Edit task metadata (priority, type, size, module, tags)
  close             Archive task (moves to archive/)
  show, cat         Print task content to stdout
  open              Open task file in default editor/app
  epic              Show epic task with children and progress
  triage            Find tasks needing grooming (placeholder goals, no module, auto-created)
  grep, search      Full-text regex search across all task files
  scan              Find untagged TODO/FIXME/HACK comments, create tasks, update source
  regenerate, regen Regenerate BOARD.md from directory contents

Status aliases:
  b=backlog  ip=in-progress  d=done

Task IDs use the configured prefix (default: PR). The prefix is optional in commands —
"tb start 123" and "tb start PR-123" are equivalent.

Configuration:
  tb discovers .tb.yaml by walking up from the current directory.
  Fallback: TB_BOARD_DIR environment variable.
`)
}
