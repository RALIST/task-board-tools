package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tools/tb-gui/internal/cli"
)

// recordingEmitter captures every Emit call so tests can assert on Wails
// payloads without standing up a real Wails app.
type recordingEmitter struct {
	events []emittedEvent
}

type emittedEvent struct {
	Name    string
	Payload []any
}

func newRecordingEmitter() *recordingEmitter { return &recordingEmitter{} }

func (e *recordingEmitter) Emit(name string, data ...any) {
	e.events = append(e.events, emittedEvent{Name: name, Payload: data})
}

func (e *recordingEmitter) names() []string {
	out := make([]string, 0, len(e.events))
	for _, ev := range e.events {
		out = append(out, ev.Name)
	}
	return out
}

func newAgentSvcWithStub(t *testing.T, stubBody string) (*AgentService, *recordingEmitter) {
	t.Helper()
	stub := makeStub(t, stubBody)
	board := NewBoardService()
	board.setClient(newClient(t, stub))
	em := newRecordingEmitter()
	svc := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})
	return svc, em
}

func TestAssignAgent_NoBoard(t *testing.T) {
	svc := NewAgentService(AgentServiceOptions{Board: NewBoardService()})
	if err := svc.AssignAgent(context.Background(), "TB-1", "claude"); !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

func TestAssignAgent_HappyPath(t *testing.T) {
	// Echo the args so we can assert the CLI invocation shape.
	svc, _ := newAgentSvcWithStub(t, `
echo "args:" "$@"
exit 0
`)

	if err := svc.AssignAgent(context.Background(), "TB-1", "claude"); err != nil {
		t.Fatalf("AssignAgent: %v", err)
	}
}

func TestAssignAgent_InvalidAgent(t *testing.T) {
	svc, _ := newAgentSvcWithStub(t, `exit 0`)
	err := svc.AssignAgent(context.Background(), "TB-1", "gpt-9000")
	if !errors.Is(err, ErrAgentNotSupported) {
		t.Fatalf("want ErrAgentNotSupported, got %v", err)
	}
}

func TestAssignAgent_ClearByNone(t *testing.T) {
	captured := filepath.Join(t.TempDir(), "args")
	stub := makeStub(t, `
printf '%s\n' "$@" > `+captured+`
exit 0
`)
	board := NewBoardService()
	board.setClient(newClient(t, stub))
	svc := NewAgentService(AgentServiceOptions{Board: board})

	if err := svc.AssignAgent(context.Background(), "TB-1", "none"); err != nil {
		t.Fatalf("AssignAgent: %v", err)
	}
	data, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("captured args: %v", err)
	}
	if !strings.Contains(string(data), "-a\nnone") {
		t.Fatalf("expected -a none, got:\n%s", data)
	}

	// Empty string normalises to none too.
	if err := svc.AssignAgent(context.Background(), "TB-1", ""); err != nil {
		t.Fatalf("AssignAgent empty: %v", err)
	}
}

func TestAssignAgent_TaskNotFound(t *testing.T) {
	svc, _ := newAgentSvcWithStub(t,
		`echo "error: task TB-99 not found in any directory" 1>&2; exit 1`)

	err := svc.AssignAgent(context.Background(), "TB-99", "claude")
	if err == nil {
		t.Fatal("expected error")
	}
	var me *cli.MutationError
	if !errors.As(err, &me) || me.Kind != cli.ErrKindTaskNotFound {
		t.Fatalf("want ErrKindTaskNotFound, got %T %+v", err, err)
	}
}

func TestNormalizeAgent_Aliases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "none"},
		{"none", "none"},
		{"NONE", "none"},
		{"  Claude  ", "claude"},
		{"codex", "codex"},
	}
	for _, c := range cases {
		got, err := normalizeAgent(c.in)
		if err != nil {
			t.Errorf("normalizeAgent(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeAgent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
