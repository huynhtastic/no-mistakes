package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAntigravityAgent_BuildArgs(t *testing.T) {
	aa := &antigravityAgent{bin: "agy"}
	args := aa.buildArgs("fix the bug")

	expected := []string{
		"-p", "fix the bug",
		"--dangerously-skip-permissions",
	}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("arg[%d]: expected %q, got %q", i, want, args[i])
		}
	}
}

func TestAntigravityAgent_BuildArgs_ExtraArgsFirst(t *testing.T) {
	aa := &antigravityAgent{bin: "agy", extraArgs: []string{"--model", "gemini-2.0"}}
	args := aa.buildArgs("fix it")

	expected := []string{
		"--model", "gemini-2.0",
		"-p", "fix it",
		"--dangerously-skip-permissions",
	}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("arg[%d]: expected %q, got %q", i, want, args[i])
		}
	}
}

func TestBuildAntigravityPrompt_WithSchema(t *testing.T) {
	prompt := "do something"
	schema := json.RawMessage(`{"type":"object"}`)
	got := buildAntigravityPrompt(prompt, schema)

	if !strings.Contains(got, "do something") {
		t.Errorf("expected prompt to contain original prompt")
	}
	if !strings.Contains(got, "no-mistakes final output contract") {
		t.Errorf("expected prompt to contain final output contract instructions")
	}
	if !strings.Contains(got, `"type"`) || !strings.Contains(got, `"object"`) {
		t.Errorf("expected prompt to contain JSON schema parts")
	}
}

func TestBuildAntigravityPrompt_NoSchema(t *testing.T) {
	prompt := "do something"
	got := buildAntigravityPrompt(prompt, nil)

	if got != prompt {
		t.Errorf("expected prompt to equal original prompt, got %q", got)
	}
}
