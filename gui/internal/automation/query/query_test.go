package query

import (
	"errors"
	"testing"
)

func TestParse_Empty(t *testing.T) {
	for _, in := range []string{"", "   ", "\t\n"} {
		if _, err := Parse(in); !errors.Is(err, ErrEmpty) {
			t.Errorf("Parse(%q) err = %v, want ErrEmpty", in, err)
		}
	}
	if IsValid("") {
		t.Errorf("IsValid(\"\") = true, want false")
	}
}

func TestParse_ACExampleBugSSizeGui(t *testing.T) {
	// AC: "bug, S size, gui" matches S-sized GUI bugs and rejects
	// non-S, non-GUI, or non-bug tasks.
	q, err := Parse("bug, S size, gui")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(q.Terms) != 3 {
		t.Fatalf("Parse: expected 3 terms, got %v", q.Terms)
	}
	if q.Terms[0] != (Term{Field: "type", Value: "bug"}) {
		t.Errorf("term[0]=%v, want type=bug", q.Terms[0])
	}
	if q.Terms[1] != (Term{Field: "size", Value: "S"}) {
		t.Errorf("term[1]=%v, want size=S", q.Terms[1])
	}
	// "gui" is a free-text fall-through that matches module=gui via
	// the case-insensitive id/title/module substring search.
	if q.Terms[2].Field != "" || q.Terms[2].Value != "gui" {
		t.Errorf("term[2]=%v, want free-text gui", q.Terms[2])
	}

	match := Task{ID: "TB-9", Title: "fix card", Type: "bug", Size: "S", Module: "gui"}
	if !Match(q, match) {
		t.Errorf("match S-sized GUI bug should match")
	}

	for _, miss := range []struct {
		name string
		task Task
	}{
		{"non-bug", Task{Type: "feature", Size: "S", Module: "gui"}},
		{"non-S", Task{Type: "bug", Size: "M", Module: "gui"}},
		{"non-gui", Task{Type: "bug", Size: "S", Module: "cli"}},
		{"empty", Task{}},
	} {
		t.Run(miss.name, func(t *testing.T) {
			if Match(q, miss.task) {
				t.Errorf("expected miss for %+v", miss.task)
			}
		})
	}
}

func TestParse_ExplicitFields(t *testing.T) {
	cases := []struct {
		in   string
		want []Term
	}{
		{"type:bug", []Term{{Field: "type", Value: "bug"}}},
		{"priority:p1", []Term{{Field: "priority", Value: "P1"}}},
		{"p:P0", []Term{{Field: "priority", Value: "P0"}}},
		{"size:m", []Term{{Field: "size", Value: "M"}}},
		{"module:gui", []Term{{Field: "module", Value: "gui"}}},
		{"m:cli", []Term{{Field: "module", Value: "cli"}}},
		{"tag:review-failed", []Term{{Field: "tag", Value: "review-failed"}}},
		{"agent:claude", []Term{{Field: "agent", Value: "claude"}}},
		{"parent:TB-177", []Term{{Field: "parent", Value: "TB-177"}}},
		{"epic:tb-177", []Term{{Field: "parent", Value: "TB-177"}}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			q, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.in, err)
			}
			if len(q.Terms) != len(tc.want) || q.Terms[0] != tc.want[0] {
				t.Errorf("Parse(%q) = %v, want %v", tc.in, q.Terms, tc.want)
			}
		})
	}
}

func TestParse_RejectsInvalidExplicit(t *testing.T) {
	cases := []struct {
		in     string
		target error
	}{
		{"type:bogus", ErrInvalidType},
		{"priority:P9", ErrInvalidPriority},
		{"size:XXL", ErrInvalidSize},
		{"P5", nil}, // bare not-priority is fall-through, not an error
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, err := Parse(tc.in)
			if tc.target == nil {
				if err != nil {
					t.Errorf("Parse(%q) unexpected err: %v", tc.in, err)
				}
				return
			}
			if !errors.Is(err, tc.target) {
				t.Errorf("Parse(%q) err = %v, want %v", tc.in, err, tc.target)
			}
		})
	}
}

func TestMatch_BareEnumTokens(t *testing.T) {
	cases := []struct {
		in   string
		task Task
		want bool
	}{
		{"P0", Task{Priority: "P0"}, true},
		{"p2", Task{Priority: "P2"}, true},
		{"P0", Task{Priority: "P1"}, false},
		{"bug", Task{Type: "bug"}, true},
		{"bug", Task{Type: "feature"}, false},
		{"M", Task{Size: "M"}, true},
		{"XL", Task{Size: "XL"}, true},
		{"S", Task{Size: "M"}, false},
	}
	for _, tc := range cases {
		q, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		got := Match(q, tc.task)
		if got != tc.want {
			t.Errorf("Match(%q, %+v) = %v, want %v", tc.in, tc.task, got, tc.want)
		}
	}
}

func TestMatch_FreeTextHitsTitleAndID(t *testing.T) {
	q, err := Parse("router")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !Match(q, Task{Title: "Fix the ROUTER bug"}) {
		t.Errorf("expected title hit (case-insensitive)")
	}
	if !Match(q, Task{ID: "tb-router", Title: "x"}) {
		t.Errorf("expected id hit")
	}
	if !Match(q, Task{Module: "router"}) {
		t.Errorf("expected module hit")
	}
	if Match(q, Task{Title: "unrelated"}) {
		t.Errorf("expected miss")
	}
}

func TestMatch_TagIsCaseSensitive(t *testing.T) {
	q, err := Parse("tag:review-failed")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !Match(q, Task{Tags: []string{"review-failed"}}) {
		t.Errorf("expected exact tag hit")
	}
	if Match(q, Task{Tags: []string{"Review-Failed"}}) {
		t.Errorf("tag matching is case-sensitive per board conventions")
	}
}

func TestMatch_EmptyQueryMatchesNothing(t *testing.T) {
	if Match(Query{}, Task{ID: "TB-1"}) {
		t.Errorf("empty query should match nothing")
	}
}

// TestParse_UnknownExplicitFieldFallback pins the parser contract for
// `field:value` tokens whose field is not a recognised key: they fall
// through to free-text so a saved query never silently changes meaning
// because of a future field addition. Future field expansions MUST add
// an explicit test before re-purposing one of these tokens.
func TestParse_UnknownExplicitFieldFallback(t *testing.T) {
	q, err := Parse("foo:bar")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(q.Terms) != 1 || q.Terms[0].Field != "" {
		t.Fatalf("expected unknown explicit field to fall through to free-text, got %v", q.Terms)
	}
	if q.Terms[0].Value != "foo:bar" {
		t.Errorf("free-text value = %q, want %q", q.Terms[0].Value, "foo:bar")
	}
	// Match semantics: free-text "foo:bar" hits title containing
	// "foo:bar" case-insensitively.
	if !Match(q, Task{Title: "Investigate FOO:BAR pattern"}) {
		t.Errorf("expected case-insensitive title hit")
	}
	if Match(q, Task{Title: "unrelated"}) {
		t.Errorf("expected miss on unrelated task")
	}
}

func TestParse_TrailingComma(t *testing.T) {
	// Stray commas are tolerated; empty segments are skipped.
	q, err := Parse("bug,,, S size ,")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(q.Terms) != 2 {
		t.Fatalf("expected 2 terms, got %d (%v)", len(q.Terms), q.Terms)
	}
}
