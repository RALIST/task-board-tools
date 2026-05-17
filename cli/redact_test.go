package main

import "testing"

// Mirror of gui/internal/redact/redact_test.go's lineCases. Keep the two
// tables in lockstep so the CLI and GUI redactor agree on outputs.
var redactLineCases = []struct {
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
	{name: "empty unquoted value left alone", in: "api_key=", want: "api_key="},
	{name: "value contains equals", in: "token=abc=def=ghi", want: "token=[REDACTED]"},
	{name: "value contains equals quoted", in: `token="abc=def=ghi"`, want: `token="[REDACTED]"`},
	{name: "bearer too short ignored", in: "Bearer 1234567", want: "Bearer 1234567"},
}

func TestRedactLine(t *testing.T) {
	for _, tc := range redactLineCases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactLine(tc.in)
			if got != tc.want {
				t.Fatalf("redactLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRedactLineLeavesSurroundingTextIntact(t *testing.T) {
	in := "before OPENAI_API_KEY=sk-fake-zzz after"
	want := "before OPENAI_API_KEY=[REDACTED] after"
	if got := redactLine(in); got != want {
		t.Fatalf("redactLine(%q) = %q, want %q", in, got, want)
	}
}

func TestRedactLineMultipleSecretsOnOneLine(t *testing.T) {
	in := "OPENAI_API_KEY=sk-aaa GITHUB_TOKEN=ghp_bbb password=ccc"
	want := "OPENAI_API_KEY=[REDACTED] GITHUB_TOKEN=[REDACTED] password=[REDACTED]"
	if got := redactLine(in); got != want {
		t.Fatalf("redactLine(%q) = %q, want %q", in, got, want)
	}
}

func TestRedactLineDoesNotMatchUnrelatedKeyword(t *testing.T) {
	in := "user typed password into the form"
	if got := redactLine(in); got != in {
		t.Fatalf("redactLine(%q) modified non-assignment line to %q", in, got)
	}
}

func TestRedactTextEqualsLineForCurrentPatterns(t *testing.T) {
	for _, tc := range redactLineCases {
		if got := redactText(tc.in); got != redactLine(tc.in) {
			t.Fatalf("redactText(%q) = %q, want %q", tc.in, got, redactLine(tc.in))
		}
	}
}
