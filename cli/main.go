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
	case "ready":
		cmdReady(os.Args[2:])
	case "pull":
		cmdPull(os.Args[2:])
	case "start":
		cmdStart(os.Args[2:])
	case "done":
		cmdDone(os.Args[2:])
	case "close":
		cmdClose(os.Args[2:])
	case "edit":
		cmdEdit(os.Args[2:])
	case "attach":
		cmdAttach(os.Args[2:])
	case "assign":
		cmdAssign(os.Args[2:])
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
	case "review":
		cmdReview(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`tb — task board CLI

Usage:
  tb init [path] [--board-path=board] [--prefix=PR] [--refresh-docs]     Initialize or reconcile a board
  tb board [--json]                                                      Print board status (or JSON snapshot)
  tb create "Title" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--parent ID] [--epic] [--legacy-file]
  tb ls [-t tag[,tag...]] [-s size[,size...]] [-m module[,module...]] [-T type[,type...]] [-p priority[,priority...]]
        [--parent ID[,ID...]] [--agent claude|codex|none[,...]] [--search term] [-n N]
        [--status backlog|ready|in-progress|code-review|done|archive|active|all] [--json]
  tb mv <ID> <status>                                                    Move task (status: backlog|ready|in-progress|code-review|done|archive)
  tb ready <ID>                                                          Promote a backlog task to ready (canonical kanban commitment column)
  tb pull [<ID>]                                                         Pull the next highest-priority ready task into in-progress
  tb review --submit <ID>                                                Submit in-progress (or review-failed ready/backlog) task to code-review
  tb review --target <ID> file|-                                         Write ## Review Target section
  tb review --notes <ID> file|-                                          Write ## Reviewer Notes section
  tb review --findings <ID> file|-                                       Write ## Review Findings section
  tb review --fail <ID> file|-                                           Fail review: write findings, move to ready, mark review-failed
  tb start <ID>                                                          Start working (warns when called on a backlog task — pulls should come from ready)
  tb done <ID>                                                           Mark done
  tb edit <ID> [-p P0] [-T type] [-s M] [-m module] [-t tags] [-a claude|codex] [--agent-status queued|running|success|failed|cancelled|interrupted|lost|needs-user] [--review-ref value|none] [--title "New title"] [--goal file|-] [--context file|-] [--constraints file|-] [--acceptance file|-] [--user-attention file|-]
  tb attach <ID> <path>...                                               Copy files into task attachments
  tb attach --rm <ID> <attachment-name>...                               Remove task attachments
  tb assign <ID> <agent>                                                 Assign claude|codex and queue for daemon pickup
  tb close <ID>                                                          Archive task
  tb show <ID> [--json]                                                  Print task content (or {metadata, body} JSON)
  tb open <ID>                                                           Open in default editor
  tb epic <ID> [--status active|archive|all]                             Show epic progress
  tb triage [--json]                                                     Find tasks needing grooming (gates the backlog → ready commit)
  tb grep <pattern> [--status backlog|ready|in-progress|code-review|done|archive|active|all] [-s] [-l]   Search tasks by regex
  tb scan [--apply] [--path dir]                                         Find untagged TODOs
  tb regenerate                                                          Regenerate BOARD.md

Canonical kanban flow:
  backlog → ready → in-progress → code-review → done → archive
  Pull from ready into in-progress with ` + "`tb pull`" + `; promote backlog → ready with ` + "`tb ready`" + `.

Status aliases:
  b=backlog  r=ready  ip/wip=in-progress  cr/review=code-review  d=done

Commands:
  init              Initialize board structure; existing boards refresh docs and annotated config with .bak backups
  board             Print board status to stdout (same format as BOARD.md)
  create, new       Create a new task
  ls, list          List and filter tasks
  mv, move          Move task between statuses
  ready             Promote a backlog task to ready (kanban commitment point; must pass triage gate)
  pull              Pull the next highest-priority ready task into in-progress (canonical kanban pull)
  start             Move task to in-progress (push-style — warns when source is backlog; prefer ` + "`tb pull`" + `)
  done              Move task to done
  edit              Edit task metadata, title (--title), and managed body sections
  attach            Copy files into task attachments; --rm: Remove task attachments by name
  assign            Assign a runnable agent and set AgentStatus=queued for daemon pickup
  close             Archive task (moves to archive/)
  show, cat         Print task content to stdout
  open              Open task file in default editor/app
  epic              Show epic task with children and progress
  triage            Find tasks needing grooming (placeholder goals, no module, auto-created)
  grep, search      Full-text regex search across all task files
  scan              Find untagged TODO/FIXME/HACK comments, create tasks, update source
  regenerate, regen Regenerate BOARD.md from directory contents
  review            Code-review flow: submit, set Review Target/Reviewer Notes/Findings, fail back to ready

Task IDs use the configured prefix (default: PR). The prefix is optional in commands —
"tb start 123" and "tb start PR-123" are equivalent.

Configuration:
  tb discovers .tb.yaml by walking up from the current directory.
  Fallback: TB_BOARD_DIR environment variable.

Shell quoting for Markdown command spans:
  Backticks inside DOUBLE quotes are command substitution in bash/zsh — the
  shell runs the inner command BEFORE tb sees its arguments. To keep literal
  ` + "`tb init`" + `-style spans in a title or description, use single quotes:

    tb create 'Try ` + "`tb init`" + ` first' -d 'See ` + "`tb --help`" + `'

  For richer Markdown bodies, prefer a heredoc via tb edit:

    tb edit TB-123 --goal - <<'EOF'
    See ` + "`tb init`" + ` and ` + "`tb --help`" + ` for details.
    EOF

Project refresh:
  Run tb init in an existing project to reconcile generated project files.
  Existing generated docs and the annotated .tb.yaml config surface are refreshed
  from the current templates. Previous versions are saved as .bak files so local
  customizations can be merged back manually. --refresh-docs is accepted for older
  scripts but is no longer required.

Attachments:
  New attachments are stored in the task directory as <status>/<ID>/<filename>.
  Legacy attachments/<filename> files remain supported for compatibility.
  Reserved attachment names: TASK.md, attachments, dotfiles/dotdirs such as
  .agent-state.jsonl, .agent-logs/, .attach.* staging dirs, and .*.tmp.* files.
`)
}
