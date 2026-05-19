// Package query parses and matches the saved auto-implement filter
// expression against tasks. The same expression syntax mirrors how
// users already filter the kanban board so a query like
//
//	bug, S size, gui
//
// reads naturally: "S-sized bugs in the gui module".
//
// Syntax (comma-separated terms, AND semantics):
//
//	type:<value>      or bare type enum (feature|bug|tech-debt|improvement|spike)
//	priority:<value>  or bare P0|P1|P2|P3 (also lowercase)
//	size:<value>      or "<S|M|L|XL> size" (trailing "size" suffix)
//	module:<value>    or bare token (falls through to module / free-text)
//	tag:<value>       — case-sensitive tag membership
//	agent:<value>     — claude|codex
//	parent:<value>    — TB-N parent id (epic: is an alias)
//	<anything else>   — free-text against (id, title, module) case-insensitive
//
// Empty queries are rejected by Parse; callers use IsValid to check a
// candidate query before enabling automation.
package query

import (
	"errors"
	"fmt"
	"strings"
)

// Term is a single parsed filter clause. Field is the canonical field
// name ("type", "priority", "size", "module", "tag", "agent", "parent")
// or "" for a free-text term. Value is the user-supplied value with
// surrounding whitespace stripped.
type Term struct {
	Field string
	Value string
}

// Query is the compiled, ready-to-match filter expression.
type Query struct {
	Terms []Term
}

// Task is the value shape Match consumes. It is a narrow projection of
// the GUI BoardService Task type so this package stays free of
// gui/app imports (used from both the daemon and frontend-facing
// validation paths).
type Task struct {
	ID       string
	Title    string
	Type     string
	Priority string
	Size     string
	Module   string
	Tags     []string
	Agent    string
	Parent   string
}

// Errors returned by Parse.
var (
	ErrEmpty           = errors.New("query is empty")
	ErrInvalidPriority = errors.New("invalid priority (use P0-P3)")
	ErrInvalidSize     = errors.New("invalid size (use S, M, L, or XL)")
	ErrInvalidType     = errors.New("invalid type (use feature, bug, tech-debt, improvement, spike)")
)

// validTypes mirrors cli/edit.go's enum so an editor typo here doesn't
// silently degrade to free-text.
var validTypes = map[string]struct{}{
	"feature":     {},
	"bug":         {},
	"tech-debt":   {},
	"improvement": {},
	"spike":       {},
}

var validSizes = map[string]struct{}{
	"S":  {},
	"M":  {},
	"L":  {},
	"XL": {},
}

var validPriorities = map[string]struct{}{
	"P0": {},
	"P1": {},
	"P2": {},
	"P3": {},
}

// Parse turns the comma-separated expression into a Query. Returns
// ErrEmpty when the expression has no usable terms. Returns wrapped
// validation errors when an explicit field carries an invalid value.
// Free-text terms always parse; the heuristic for bare tokens is
// "looks-like-an-enum, else free-text".
func Parse(expr string) (Query, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return Query{}, ErrEmpty
	}

	var terms []Term
	for _, raw := range strings.Split(expr, ",") {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		term, err := parseToken(token)
		if err != nil {
			return Query{}, err
		}
		terms = append(terms, term)
	}
	if len(terms) == 0 {
		return Query{}, ErrEmpty
	}
	return Query{Terms: terms}, nil
}

// IsValid reports whether expr parses to a non-empty query. Convenience
// wrapper for UI/validation paths that don't need the parsed value.
func IsValid(expr string) bool {
	_, err := Parse(expr)
	return err == nil
}

func parseToken(token string) (Term, error) {
	// Explicit field:value form.
	if idx := strings.Index(token, ":"); idx > 0 && idx < len(token)-1 {
		field := strings.ToLower(strings.TrimSpace(token[:idx]))
		value := strings.TrimSpace(token[idx+1:])
		return parseExplicit(field, value)
	}

	// "S size" / "L size" suffix form.
	if lower := strings.ToLower(token); strings.HasSuffix(lower, " size") {
		size := strings.ToUpper(strings.TrimSpace(token[:len(token)-len(" size")]))
		if _, ok := validSizes[size]; !ok {
			return Term{}, fmt.Errorf("%w: %q", ErrInvalidSize, size)
		}
		return Term{Field: "size", Value: size}, nil
	}

	// Priority enum (bare).
	upper := strings.ToUpper(token)
	if _, ok := validPriorities[upper]; ok {
		return Term{Field: "priority", Value: upper}, nil
	}

	// Type enum (bare).
	lower := strings.ToLower(token)
	if _, ok := validTypes[lower]; ok {
		return Term{Field: "type", Value: lower}, nil
	}

	// Bare size enum (S, M, L, XL) without the " size" suffix.
	if _, ok := validSizes[upper]; ok && (upper == "S" || upper == "M" || upper == "L" || upper == "XL") {
		return Term{Field: "size", Value: upper}, nil
	}

	// Free-text fall-through. Lowercase for case-insensitive substring
	// matching at Match time.
	return Term{Field: "", Value: strings.ToLower(token)}, nil
}

func parseExplicit(field, value string) (Term, error) {
	switch field {
	case "type":
		v := strings.ToLower(value)
		if _, ok := validTypes[v]; !ok {
			return Term{}, fmt.Errorf("%w: %q", ErrInvalidType, value)
		}
		return Term{Field: "type", Value: v}, nil
	case "priority", "p":
		v := strings.ToUpper(value)
		if _, ok := validPriorities[v]; !ok {
			return Term{}, fmt.Errorf("%w: %q", ErrInvalidPriority, value)
		}
		return Term{Field: "priority", Value: v}, nil
	case "size":
		v := strings.ToUpper(value)
		if _, ok := validSizes[v]; !ok {
			return Term{}, fmt.Errorf("%w: %q", ErrInvalidSize, value)
		}
		return Term{Field: "size", Value: v}, nil
	case "module", "m":
		return Term{Field: "module", Value: strings.ToLower(value)}, nil
	case "tag":
		// Tags are matched case-sensitively against the literal tag
		// string the CLI writes (see board/CONVENTIONS.md tag taxonomy).
		return Term{Field: "tag", Value: value}, nil
	case "agent":
		return Term{Field: "agent", Value: strings.ToLower(value)}, nil
	case "parent", "epic":
		return Term{Field: "parent", Value: strings.ToUpper(value)}, nil
	default:
		// Unknown field — treat as free-text "<field>:<value>" so the
		// user gets a fall-through rather than an obscure validation
		// error. Lowercased for case-insensitive substring match.
		return Term{Field: "", Value: strings.ToLower(field + ":" + value)}, nil
	}
}

// Match reports whether the task satisfies every term in the query.
// An empty query matches nothing (callers should treat parse errors
// as "filter is invalid", not "filter is empty match-all").
func Match(q Query, t Task) bool {
	if len(q.Terms) == 0 {
		return false
	}
	for _, term := range q.Terms {
		if !matchTerm(term, t) {
			return false
		}
	}
	return true
}

func matchTerm(term Term, t Task) bool {
	switch term.Field {
	case "type":
		return strings.EqualFold(t.Type, term.Value)
	case "priority":
		return strings.EqualFold(t.Priority, term.Value)
	case "size":
		return strings.EqualFold(t.Size, term.Value)
	case "module":
		return strings.EqualFold(t.Module, term.Value)
	case "tag":
		for _, tag := range t.Tags {
			if tag == term.Value {
				return true
			}
		}
		return false
	case "agent":
		return strings.EqualFold(t.Agent, term.Value)
	case "parent":
		return strings.EqualFold(t.Parent, term.Value)
	case "":
		// Free-text matches against id, title, or module (case-insensitive).
		v := term.Value
		if v == "" {
			return false
		}
		return strings.Contains(strings.ToLower(t.ID), v) ||
			strings.Contains(strings.ToLower(t.Title), v) ||
			strings.Contains(strings.ToLower(t.Module), v)
	default:
		return false
	}
}
