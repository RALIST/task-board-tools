package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const queuedAgentStatus = "queued"

func cmdAssign(args []string) {
	if len(args) != 2 {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "error: task ID and agent are required")
		} else {
			fmt.Fprintln(os.Stderr, "error: too many arguments")
		}
		assignUsage()
		os.Exit(1)
	}

	taskID := normalizeTaskID(args[0])
	agent, err := normalizeRunnableAgent(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		assignUsage()
		os.Exit(1)
	}

	if err := assignTask(cfg.BoardDir, taskID, agent); err != nil {
		fatal("%v", err)
	}

	fmt.Printf("Assigned %s to agent=%s, agentstatus=%s\n", taskID, agent, queuedAgentStatus)
}

func assignUsage() {
	fmt.Fprintln(os.Stderr, "Usage: tb assign <ID> <agent>")
	fmt.Fprintln(os.Stderr, "       agent: claude | codex")
}

func normalizeRunnableAgent(raw string) (string, error) {
	agent := strings.ToLower(strings.TrimSpace(raw))
	if agent == "none" {
		return "", fmt.Errorf("invalid agent %q - assign only accepts runnable agents: claude, codex", raw)
	}
	if !validAgents[agent] {
		return "", fmt.Errorf("invalid agent %q - use: claude, codex", raw)
	}
	return agent, nil
}

func assignTask(boardDir, taskID, agent string) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return err
	}
	taskPath := ref.Path

	data, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", taskPath, err)
	}

	lines := strings.Split(string(data), "\n")
	lines = setField(lines, "Agent", agent)
	lines = setField(lines, "AgentStatus", queuedAgentStatus)

	today := time.Now().Format("2006-01-02")
	content := strings.Join(lines, "\n")
	content = appendLogEntry(content, fmt.Sprintf("- %s: Assigned agent=%s, agentstatus=%s\n", today, agent, queuedAgentStatus))

	if err := writeFileAtomic(taskPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", taskPath, err)
	}
	if err := cleanupOrphanFileFormSibling(boardDir, ref.Status, ref.ID); err != nil {
		return err
	}
	if err := regenerateBoard(boardDir); err != nil {
		return fmt.Errorf("cannot regenerate BOARD.md: %w", err)
	}
	return nil
}
