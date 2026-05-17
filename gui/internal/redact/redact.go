// Package redact masks credential-like substrings in free-form text so they
// do not leak into agent logs, JSONL state files, Wails events, or
// regenerated task markdown. The redactor is intentionally conservative:
// it targets a small set of well-known shapes (sensitive key=value or
// key: value assignments, Bearer-style auth headers, and the well-known
// credential env names listed in KnownEnvNames) and replaces only the
// value with [REDACTED], leaving the key, separator, and surrounding
// context intact so reviewers can still tell what was masked.
//
// This file is mirrored verbatim at cli/redact.go to keep the contract
// identical between the CLI and the GUI daemon without introducing a
// shared module. Test tables in both packages assert the same outputs.
package redact

import (
	"regexp"
	"strings"
)

// Placeholder is the string substituted in place of any matched secret value.
const Placeholder = "[REDACTED]"

// KnownEnvNames is the explicit allow-list of credential environment variable
// names whose values must always be masked, even when the assignment text
// does not otherwise match the generic key=value pattern.
var KnownEnvNames = []string{
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

// keyPattern matches the "name part" of a generic sensitive assignment.
const keyPattern = `["']?\b[a-z0-9_-]*(?:api[_-]?key|secret|password|passwd|token|bearer)[a-z0-9_-]*["']?`

var (
	knownEnvNamesGroup = `(?:` + strings.Join(KnownEnvNames, `|`) + `)`

	// Quoted-value assignments. Go's RE2 has no backreferences, so the two
	// quote styles get separate patterns. Both run before the unquoted form
	// so a quoted value with spaces is fully masked. Inside the captured
	// quotes, value content may contain whitespace; we stop at the closing
	// quote or a newline.
	assignmentDoubleQuotedRE = regexp.MustCompile(`(?i)(` + keyPattern + `[\t ]*[:=][\t ]*)"([^"\n]*)"`)
	assignmentSingleQuotedRE = regexp.MustCompile(`(?i)(` + keyPattern + `[\t ]*[:=][\t ]*)'([^'\n]*)'`)

	// Unquoted assignment: value stops at the first whitespace, quote,
	// comma, or newline. Run after the quoted patterns so it doesn't
	// pre-strip half of a quoted-with-space value.
	assignmentUnquotedRE = regexp.MustCompile(`(?i)(` + keyPattern + `[\t ]*[:=][\t ]*)([^\s"',\n]+)`)

	// Bearer-style authorization values. Charset includes the standard
	// base64url + JWT alphabet (digits, ASCII letters, `-`, `_`, `.`, `+`,
	// `/`, `=`). Underscore was previously missing — JWTs split on `.` and
	// each segment can contain `_`, so a token like `eyJ_abc.xxx` was being
	// truncated mid-redaction (TB-203 review finding).
	bearerRE = regexp.MustCompile(`(?i)\b(Bearer[\t ]+)([\w.\-+/=]{8,})`)

	// Quoted/unquoted variants of well-known env-name assignments.
	knownEnvDoubleQuotedRE = regexp.MustCompile(`(?i)\b(` + knownEnvNamesGroup + `)([\t ]*[:=][\t ]*)"([^"\n]*)"`)
	knownEnvSingleQuotedRE = regexp.MustCompile(`(?i)\b(` + knownEnvNamesGroup + `)([\t ]*[:=][\t ]*)'([^'\n]*)'`)
	knownEnvUnquotedRE     = regexp.MustCompile(`(?i)\b(` + knownEnvNamesGroup + `)([\t ]*[:=][\t ]*)([^\s"',\n]+)`)
)

// Line redacts secret-like substrings in a single line of text. The original
// content is returned when no patterns match.
//
// Ordering is significant: quoted-value patterns run before unquoted ones so
// a value containing spaces (e.g. `PASSWORD="hunter 2"`) is fully masked
// instead of being truncated at the first whitespace by the unquoted form.
// Known-env names run before the generic assignment so their match always
// wins (the generic form would still cover them, but routing through the
// specific pattern keeps replacement groups easy to reason about).
func Line(s string) string {
	if s == "" {
		return s
	}
	s = knownEnvDoubleQuotedRE.ReplaceAllString(s, `${1}${2}"`+Placeholder+`"`)
	s = knownEnvSingleQuotedRE.ReplaceAllString(s, `${1}${2}'`+Placeholder+`'`)
	s = knownEnvUnquotedRE.ReplaceAllString(s, "${1}${2}"+Placeholder)
	s = assignmentDoubleQuotedRE.ReplaceAllString(s, `${1}"`+Placeholder+`"`)
	s = assignmentSingleQuotedRE.ReplaceAllString(s, `${1}'`+Placeholder+`'`)
	s = assignmentUnquotedRE.ReplaceAllString(s, "${1}"+Placeholder)
	s = bearerRE.ReplaceAllString(s, "${1}"+Placeholder)
	return s
}

// Text redacts a multi-line string. The current pattern set is line-agnostic
// (each regex stops at newlines), so this is equivalent to Line for now and
// is provided so callers do not need to know the implementation detail.
func Text(s string) string {
	return Line(s)
}
