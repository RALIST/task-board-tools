package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type generatedDoc struct {
	name    string
	content string
}

type refreshedDoc struct {
	path       string
	backupPath string
	changed    bool
}

func generatedBoardDocs(prefix, boardPath string) []generatedDoc {
	uppercasePrefix := strings.ToUpper(prefix)
	return []generatedDoc{
		{name: "CONVENTIONS.md", content: conventionsTemplate(uppercasePrefix, boardPath)},
		{name: "SKILL.md", content: skillTemplate(uppercasePrefix, boardPath)},
	}
}

func refreshGeneratedDocs(boardDir, prefix, boardPath string) ([]refreshedDoc, error) {
	docs := generatedBoardDocs(prefix, boardPath)
	results := make([]refreshedDoc, 0, len(docs))
	for _, doc := range docs {
		path := filepath.Join(boardDir, doc.name)
		backupPath, changed, err := writeFileWithBackup(path, []byte(doc.content))
		if err != nil {
			return nil, err
		}
		results = append(results, refreshedDoc{path: path, backupPath: backupPath, changed: changed})
	}
	return results, nil
}

func writeFileWithBackup(path string, content []byte) (string, bool, error) {
	var backupPath string
	existing, err := os.ReadFile(path)
	if err == nil {
		if bytes.Equal(existing, content) {
			return "", false, nil
		}
		backup, backupErr := nextBackupPath(path)
		if backupErr != nil {
			return "", false, backupErr
		}
		if writeErr := writeFileAtomic(backup, existing, 0644); writeErr != nil {
			return "", false, fmt.Errorf("cannot back up %s to %s: %w", path, backup, writeErr)
		}
		backupPath = backup
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("cannot read %s: %w", path, err)
	}

	if err := writeFileAtomic(path, content, 0644); err != nil {
		return "", false, fmt.Errorf("cannot write %s: %w", path, err)
	}
	return backupPath, true, nil
}

func nextBackupPath(path string) (string, error) {
	for i := 0; ; i++ {
		candidate := path + ".bak"
		if i > 0 {
			candidate = fmt.Sprintf("%s.bak.%d", path, i)
		}
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("cannot inspect backup path %s: %w", candidate, err)
		}
	}
}

func printRefreshResults(out io.Writer, boardDir string, results []refreshedDoc) {
	changed := false
	for _, result := range results {
		if result.changed {
			changed = true
			break
		}
	}
	if !changed {
		fmt.Fprintf(out, "Board docs already current at %s\n", boardDir)
		return
	}

	fmt.Fprintf(out, "Refreshed board docs at %s\n", boardDir)
	for _, result := range results {
		if !result.changed {
			continue
		}
		name := filepath.Base(result.path)
		if result.backupPath == "" {
			fmt.Fprintf(out, "  %s\n", name)
			continue
		}
		fmt.Fprintf(out, "  %s (backup: %s)\n", name, filepath.Base(result.backupPath))
	}
}
