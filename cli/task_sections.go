package main

import "strings"

type taskSectionRange struct {
	start     int
	bodyStart int
	end       int
}

func findTaskSection(content, heading string) (taskSectionRange, bool) {
	target := normalizeTaskSectionHeading(heading)
	return findTaskSectionWhere(content, func(line string) bool {
		return line == target
	})
}

func findFirstTaskSection(content string, headings []string) (taskSectionRange, bool) {
	wanted := make(map[string]bool, len(headings))
	for _, heading := range headings {
		wanted[normalizeTaskSectionHeading(heading)] = true
	}
	return findTaskSectionWhere(content, func(line string) bool {
		return wanted[line]
	})
}

func findTaskSectionWhere(content string, match func(string) bool) (taskSectionRange, bool) {
	var found taskSectionRange
	hasFound := false
	inFence := false
	fenceMarker := ""

	for offset := 0; offset < len(content); {
		lineEnd, nextOffset := taskSectionLineBounds(content, offset)
		line := strings.TrimSuffix(content[offset:lineEnd], "\r")

		if marker := taskSectionFenceMarker(line); marker != "" {
			if inFence {
				if marker == fenceMarker {
					inFence = false
					fenceMarker = ""
				}
			} else {
				inFence = true
				fenceMarker = marker
			}
		} else if !inFence && strings.HasPrefix(line, "## ") {
			normalized := normalizeTaskSectionHeading(line)
			if hasFound {
				found.end = offset
				return found, true
			}
			if match(normalized) {
				found = taskSectionRange{
					start:     offset,
					bodyStart: nextOffset,
					end:       len(content),
				}
				hasFound = true
			}
		}

		offset = nextOffset
	}

	if hasFound {
		found.end = len(content)
		return found, true
	}
	return taskSectionRange{}, false
}

func normalizeTaskSectionHeading(line string) string {
	return strings.TrimRight(line, " \t")
}

func taskSectionFenceMarker(line string) string {
	if strings.HasPrefix(line, "```") {
		return "```"
	}
	if strings.HasPrefix(line, "~~~") {
		return "~~~"
	}
	return ""
}

func taskSectionLineBounds(content string, offset int) (lineEnd, nextOffset int) {
	if idx := strings.IndexByte(content[offset:], '\n'); idx != -1 {
		lineEnd = offset + idx
		return lineEnd, lineEnd + 1
	}
	return len(content), len(content)
}
