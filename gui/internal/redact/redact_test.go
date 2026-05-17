package redact

import "testing"

// Reusable test table — mirrored in cli/redact_test.go so the CLI's copy
// of this package stays in lockstep.
var lineCases = []struct {
	name string
	in   string
	want string
}{
	{name: "known env equals", in: "OPENAI_API_KEY=sk-fake-1234567890", want: "OPENAI_API_KEY=[REDACTED]"},
	{name: "known env equals quoted", in: `OPENAI_API_KEY="sk-fake-1234"`, want: `OPENAI_API_KEY="[REDACTED]"`},
	{name: "known env exported", in: "export GITHUB_TOKEN=ghp_fakefakefake", want: "export GITHUB_TOKEN=[REDACTED]"},
	{name: "known env colon", in: "ANTHROPIC_API_KEY: sk-ant-fake-zzz", want: "ANTHROPIC_API_KEY: [REDACTED]"},
	{name: "generic api-key assignment", in: "api_key=abcdef123", want: "api_key=[REDACTED]"},
	{name: "generic api-key colon", in: "api-key: abcdef123", want: "api-key: [REDACTED]"},
	{name: "password assignment", in: "password=hunter2", want: "password=[REDACTED]"},
	{name: "token json", in: `"token": "abc.def.ghi"`, want: `"token": "[REDACTED]"`},
	{name: "secret prefixed", in: "MY_DEPLOY_SECRET=topsecret", want: "MY_DEPLOY_SECRET=[REDACTED]"},
	{name: "bearer header", in: "Authorization: Bearer ey.abc.def-xyz", want: "Authorization: Bearer [REDACTED]"},
	{name: "bearer mid sentence", in: "request used Bearer abcd1234efgh to auth", want: "request used Bearer [REDACTED] to auth"},
	{name: "bearer with jwt underscore", in: "auth: Bearer eyJhbGc_FakeMidJWT.payload_part.signature_with_underscores", want: "auth: Bearer [REDACTED]"},
	{name: "no secret untouched", in: "stdout: hello world", want: "stdout: hello world"},
	{name: "empty stays empty", in: "", want: ""},
	{name: "non-credential token-like word", in: "token", want: "token"},
	{name: "case-insensitive key", in: "Password = letMeIn", want: "Password = [REDACTED]"},
	// TB-203 review additions: quoted values with spaces must be fully masked.
	{name: "quoted value with spaces", in: `password="hunter two with space"`, want: `password="[REDACTED]"`},
	{name: "single-quoted value with spaces", in: `password='let me in'`, want: `password='[REDACTED]'`},
	{name: "known env quoted value with spaces", in: `OPENAI_API_KEY="sk fake with space"`, want: `OPENAI_API_KEY="[REDACTED]"`},
	// Empty assignment must not crash and either redact to nothing or leave alone.
	{name: "empty unquoted value left alone", in: "api_key=", want: "api_key="},
	// Multi-equals: only the part after the first separator is the value.
	{name: "value contains equals", in: "token=abc=def=ghi", want: "token=[REDACTED]"},
	{name: "value contains equals quoted", in: `token="abc=def=ghi"`, want: `token="[REDACTED]"`},
	// Bearer must require at least 8 chars (avoid bleeding into "Bearer" prose).
	{name: "bearer too short ignored", in: "Bearer 1234567", want: "Bearer 1234567"},
}

func TestLineRedacts(t *testing.T) {
	for _, tc := range lineCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Line(tc.in)
			if got != tc.want {
				t.Fatalf("Line(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTextIsLineForCurrentPatterns(t *testing.T) {
	for _, tc := range lineCases {
		if got := Text(tc.in); got != Line(tc.in) {
			t.Fatalf("Text(%q) = %q, want %q (Line result)", tc.in, got, Line(tc.in))
		}
	}
}

func TestLineLeavesSurroundingTextIntact(t *testing.T) {
	in := "before OPENAI_API_KEY=sk-fake-zzz after"
	want := "before OPENAI_API_KEY=[REDACTED] after"
	if got := Line(in); got != want {
		t.Fatalf("Line(%q) = %q, want %q", in, got, want)
	}
}

func TestLineMultipleSecretsOnOneLine(t *testing.T) {
	in := "OPENAI_API_KEY=sk-aaa GITHUB_TOKEN=ghp_bbb password=ccc"
	want := "OPENAI_API_KEY=[REDACTED] GITHUB_TOKEN=[REDACTED] password=[REDACTED]"
	if got := Line(in); got != want {
		t.Fatalf("Line(%q) = %q, want %q", in, got, want)
	}
}

func TestLineDoesNotMatchUnrelatedKeyword(t *testing.T) {
	in := "user typed password into the form"
	if got := Line(in); got != in {
		t.Fatalf("Line(%q) modified non-assignment line to %q", in, got)
	}
}
