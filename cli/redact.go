// Mirror of gui/internal/redact/redact.go. The CLI lives in package main with
// no sub-packages, so the helper is duplicated rather than imported. Keep the
// patterns and the test table in lockstep across the two copies.
package main

import (
	"regexp"
	"strings"
)

const redactPlaceholder = "[REDACTED]"

var redactKnownEnvNames = []string{
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
	"GITHUB_TOKEN",
	"GH_TOKEN",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"SLACK_TOKEN",
	"SLACK_BOT_TOKEN",
	"GOOGLE_API_KEY",
}

// redactKeyPattern matches the "name part" of a generic sensitive assignment.
const redactKeyPattern = `["']?\b[a-z0-9_-]*(?:api[_-]?key|secret|password|passwd|token|bearer)[a-z0-9_-]*["']?`

var (
	redactKnownEnvNamesGroup = `(?:` + strings.Join(redactKnownEnvNames, `|`) + `)`

	redactAssignmentDoubleQuotedRE = regexp.MustCompile(`(?i)(` + redactKeyPattern + `[\t ]*[:=][\t ]*)"([^"\n]*)"`)
	redactAssignmentSingleQuotedRE = regexp.MustCompile(`(?i)(` + redactKeyPattern + `[\t ]*[:=][\t ]*)'([^'\n]*)'`)
	redactAssignmentUnquotedRE     = regexp.MustCompile(`(?i)(` + redactKeyPattern + `[\t ]*[:=][\t ]*)([^\s"',\n]+)`)

	redactBearerRE = regexp.MustCompile(`(?i)\b(Bearer[\t ]+)([\w.\-+/=]{8,})`)

	redactKnownEnvDoubleQuotedRE = regexp.MustCompile(`(?i)\b(` + redactKnownEnvNamesGroup + `)([\t ]*[:=][\t ]*)"([^"\n]*)"`)
	redactKnownEnvSingleQuotedRE = regexp.MustCompile(`(?i)\b(` + redactKnownEnvNamesGroup + `)([\t ]*[:=][\t ]*)'([^'\n]*)'`)
	redactKnownEnvUnquotedRE     = regexp.MustCompile(`(?i)\b(` + redactKnownEnvNamesGroup + `)([\t ]*[:=][\t ]*)([^\s"',\n]+)`)
)

// redactLine masks credential-like values in a single line. Equivalent to
// gui/internal/redact.Line — keep these two implementations byte-for-byte
// equivalent in behavior; the test tables on both sides assert identical
// outputs against a shared corpus.
func redactLine(s string) string {
	if s == "" {
		return s
	}
	s = redactKnownEnvDoubleQuotedRE.ReplaceAllString(s, `${1}${2}"`+redactPlaceholder+`"`)
	s = redactKnownEnvSingleQuotedRE.ReplaceAllString(s, `${1}${2}'`+redactPlaceholder+`'`)
	s = redactKnownEnvUnquotedRE.ReplaceAllString(s, "${1}${2}"+redactPlaceholder)
	s = redactAssignmentDoubleQuotedRE.ReplaceAllString(s, `${1}"`+redactPlaceholder+`"`)
	s = redactAssignmentSingleQuotedRE.ReplaceAllString(s, `${1}'`+redactPlaceholder+`'`)
	s = redactAssignmentUnquotedRE.ReplaceAllString(s, "${1}"+redactPlaceholder)
	s = redactBearerRE.ReplaceAllString(s, "${1}"+redactPlaceholder)
	return s
}

// redactText masks credentials in multi-line text. The current patterns are
// line-agnostic; this wrapper exists so callers don't need to know that.
func redactText(s string) string {
	return redactLine(s)
}
