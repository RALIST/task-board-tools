package app

import (
	"encoding/json"
	"strings"
)

// AutoImplementFilter is the persisted shape of the auto-implement query.
// It mirrors the field set TB-289 exposes on `tb ls` so the coordinator
// can hand the filter straight to the CLI without re-implementing
// matching in-process. Field semantics are the same as `tb ls`:
//   - Search: substring match against id+title, case-insensitive.
//   - Types/Priorities/Sizes/Modules/Tags/Agents/Parents: OR-within-field,
//     AND-across-field. Each value matches `tb ls`'s per-field semantics
//     (exact equality for type/priority/size, substring for module, tag
//     name match, normalized parent id, agent enum with "none" sentinel).
//
// The frontend BoardFilter shape uses a single `parentEpic` because the
// board UI is single-epic-focus today; the adapter on save inflates it
// into a one-element Parents slice (or empty). No restore path exists
// yet — the persisted filter is one-way state for the coordinator.
type AutoImplementFilter struct {
	Search     string   `json:"search"`
	Types      []string `json:"types"`
	Priorities []string `json:"priorities"`
	Modules    []string `json:"modules"`
	Sizes      []string `json:"sizes"`
	Tags       []string `json:"tags"`
	Agents     []string `json:"agents"`
	Parents    []string `json:"parents"`
}

// UnmarshalJSON tolerates the legacy text-DSL string form by silently
// resetting to the zero value. loadPreferences logs a one-line migration
// warning when it spots the legacy shape; here we only need to keep the
// decode from failing so a stale `preferences.json` doesn't error.
func (f *AutoImplementFilter) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		// Legacy text form: reset to empty filter without erroring.
		*f = AutoImplementFilter{}
		return nil
	}
	type alias AutoImplementFilter
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*f = AutoImplementFilter(a)
	return nil
}

// IsEmpty reports whether the filter constrains nothing — used to gate
// SetAutoImplementEnabled and to surface the "needs filter" signal in
// AutoImplementStatus.
func (f AutoImplementFilter) IsEmpty() bool {
	if strings.TrimSpace(f.Search) != "" {
		return false
	}
	return len(f.Types) == 0 &&
		len(f.Priorities) == 0 &&
		len(f.Modules) == 0 &&
		len(f.Sizes) == 0 &&
		len(f.Tags) == 0 &&
		len(f.Agents) == 0 &&
		len(f.Parents) == 0
}

// normalize trims whitespace from every value and drops empty segments,
// matching how `tb ls` parses comma-separated input. Returns a copy.
func (f AutoImplementFilter) normalize() AutoImplementFilter {
	return AutoImplementFilter{
		Search:     strings.TrimSpace(f.Search),
		Types:      cleanStringSlice(f.Types),
		Priorities: cleanStringSlice(f.Priorities),
		Modules:    cleanStringSlice(f.Modules),
		Sizes:      cleanStringSlice(f.Sizes),
		Tags:       cleanStringSlice(f.Tags),
		Agents:     cleanStringSlice(f.Agents),
		Parents:    cleanStringSlice(f.Parents),
	}
}

func cleanStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// toLsArgs converts the filter into the comma-separated flag arguments
// `tb ls` understands. The status flag is supplied by the caller.
func (f AutoImplementFilter) toLsArgs() []string {
	var args []string
	if joined := strings.Join(f.Types, ","); joined != "" {
		args = append(args, "-T", joined)
	}
	if joined := strings.Join(f.Priorities, ","); joined != "" {
		args = append(args, "-p", joined)
	}
	if joined := strings.Join(f.Modules, ","); joined != "" {
		args = append(args, "-m", joined)
	}
	if joined := strings.Join(f.Sizes, ","); joined != "" {
		args = append(args, "-s", joined)
	}
	if joined := strings.Join(f.Tags, ","); joined != "" {
		args = append(args, "-t", joined)
	}
	if joined := strings.Join(f.Agents, ","); joined != "" {
		args = append(args, "--agent", joined)
	}
	if joined := strings.Join(f.Parents, ","); joined != "" {
		args = append(args, "--parent", joined)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		args = append(args, "--search", s)
	}
	return args
}
