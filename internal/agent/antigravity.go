package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/kunchenguid/no-mistakes/internal/shellenv"
)

// antigravityAgent spawns the agy CLI for each invocation.
type antigravityAgent struct {
	bin       string
	extraArgs []string
}

func (a *antigravityAgent) Name() string { return "antigravity" }

func (a *antigravityAgent) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	return runWithRetry(ctx, "antigravity", opts, claudeMaxRetries, classifyTransient, nil, func() (*Result, error) {
		return a.runOnce(ctx, opts)
	})
}

func (a *antigravityAgent) Close() error { return nil }

func (a *antigravityAgent) runOnce(ctx context.Context, opts RunOpts) (*Result, error) {
	prompt := buildAntigravityPrompt(opts.Prompt, opts.JSONSchema)
	args := a.buildArgs(prompt)
	cmd := exec.CommandContext(ctx, a.bin, args...)
	cmd.Dir = opts.CWD
	cmd.Stdin = nil
	cmd.Env = gitSafeEnv(opts.CWD)
	shellenv.ConfigureShellCommand(cmd)

	var stderrBuf []byte
	var stderrWG sync.WaitGroup
	started, err := startNativeAgentCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("antigravity start: %w", err)
	}
	defer started.closePipes()
	pid := started.pid()
	emitAgentStarted(opts, "antigravity", pid)

	stderrWG.Add(1)
	go func() {
		defer stderrWG.Done()
		stderrBuf, _ = io.ReadAll(started.stderr)
	}()

	var textBuf strings.Builder
	scanner := bufio.NewScanner(started.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			err = started.waitAfterParseError(ctx.Err())
			stderrWG.Wait()
			retErr := fmt.Errorf("antigravity parse events: %w", err)
			emitAgentExited(opts, "antigravity", pid, retErr)
			return nil, retErr
		default:
		}
		line := scanner.Text()
		textBuf.WriteString(line)
		textBuf.WriteByte('\n')
		if opts.OnChunk != nil {
			opts.OnChunk(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		err = started.waitAfterParseError(err)
		stderrWG.Wait()
		retErr := fmt.Errorf("antigravity parse events: %w", err)
		emitAgentExited(opts, "antigravity", pid, retErr)
		return nil, retErr
	}

	waitErr := started.wait()
	stderrWG.Wait()
	if waitErr != nil {
		stderr := strings.TrimSpace(string(stderrBuf))
		if stderr != "" {
			retErr := fmt.Errorf("antigravity exited: %w: %s", waitErr, stderr)
			emitAgentExited(opts, "antigravity", pid, retErr)
			return nil, retErr
		}
		retErr := fmt.Errorf("antigravity exited: %w", waitErr)
		emitAgentExited(opts, "antigravity", pid, retErr)
		return nil, retErr
	}

	text := strings.TrimSpace(textBuf.String())
	var usage TokenUsage
	if len(text) > 0 {
		usage.OutputTokens = (len(text) + 3) / 4
	}
	if len(prompt) > 0 {
		usage.InputTokens = (len(prompt) + 3) / 4
	}

	res, err := finalizeTextResult("antigravity", text, opts.JSONSchema, usage)
	emitAgentExited(opts, "antigravity", pid, err)
	return res, err
}

func (a *antigravityAgent) buildArgs(prompt string) []string {
	args := make([]string, 0, len(a.extraArgs)+3)
	args = append(args, a.extraArgs...)
	args = append(args,
		"-p", prompt,
		"--dangerously-skip-permissions",
	)
	return args
}

func buildAntigravityPrompt(prompt string, schema json.RawMessage) string {
	if len(schema) == 0 {
		return prompt
	}
	pretty, err := json.MarshalIndent(json.RawMessage(schema), "", "  ")
	if err != nil {
		pretty = []byte(schema)
	}
	return prompt + "\n\n## no-mistakes final output contract\n\n" +
		"When the task is complete, your final assistant response must be only valid JSON matching this JSON Schema. " +
		"Do not wrap it in Markdown fences. Do not include prose before or after the JSON object.\n\n" +
		string(pretty)
}
