package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// helper: create a model and simulate a WindowSizeMsg so it's ready
func readyModel(opts ...func(*model)) model {
	m := initialModel("echo", []string{"hello"}, 2*time.Second, false, false)
	for _, opt := range opts {
		opt(&m)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(model)
}

func withDiff(m *model)    { m.diffMode = true }
func withOutput(s string) func(*model) {
	return func(m *model) { m.output = s }
}

// keyPress builds a tea.KeyPressMsg that matches the given key string.
func keyPress(k string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: []rune(k)[0]}
}

// ─── GNU watch feature: default interval is 2 seconds ───────────────────────

func TestDefaultInterval(t *testing.T) {
	m := initialModel("date", nil, 2*time.Second, false, false)
	if m.interval != 2*time.Second {
		t.Errorf("expected default interval 2s, got %v", m.interval)
	}
}

// ─── GNU watch feature: -n flag sets custom interval ────────────────────────

func TestCustomInterval(t *testing.T) {
	m := initialModel("date", nil, 500*time.Millisecond, false, false)
	if m.interval != 500*time.Millisecond {
		t.Errorf("expected interval 500ms, got %v", m.interval)
	}
}

// ─── GNU watch feature: command is passed to sh -c ──────────────────────────

func TestCommandExecution(t *testing.T) {
	cmd := runCommand("echo", []string{"hello", "world"})
	msg := cmd()
	out := msg.(commandOutputMsg)
	if out.err != nil {
		t.Fatalf("unexpected error: %v", out.err)
	}
	if strings.TrimSpace(out.output) != "hello world" {
		t.Errorf("expected 'hello world', got %q", out.output)
	}
}

func TestCommandWithPipe(t *testing.T) {
	cmd := runCommand("echo hello | tr a-z A-Z", nil)
	msg := cmd()
	out := msg.(commandOutputMsg)
	if out.err != nil {
		t.Fatalf("unexpected error: %v", out.err)
	}
	if strings.TrimSpace(out.output) != "HELLO" {
		t.Errorf("expected 'HELLO', got %q", out.output)
	}
}

func TestCommandFailureSetsErr(t *testing.T) {
	cmd := runCommand("exit 1", nil)
	msg := cmd()
	out := msg.(commandOutputMsg)
	if out.err == nil {
		t.Error("expected error for exit 1, got nil")
	}
}

// ─── GNU watch feature: periodic execution via tick ─────────────────────────

func TestTickTriggersCommandAndNextTick(t *testing.T) {
	m := readyModel()
	updated, cmd := m.Update(tickMsg(time.Now()))
	um := updated.(model)

	if !um.executing {
		t.Error("expected executing=true after tick")
	}
	if cmd == nil {
		t.Error("expected batch cmd after tick (command + next tick)")
	}
}

// ─── GNU watch feature: output is displayed ─────────────────────────────────

func TestOutputDisplayed(t *testing.T) {
	m := readyModel()
	updated, _ := m.Update(commandOutputMsg{output: "hello world\n"})
	um := updated.(model)

	if um.output != "hello world\n" {
		t.Errorf("expected output to be stored, got %q", um.output)
	}

	content := um.buildContent()
	if !strings.Contains(content, "hello world") {
		t.Errorf("expected content to contain 'hello world', got %q", content)
	}
}

// ─── GNU watch feature: -d highlights differences ───────────────────────────

func TestDiffModeHighlightsInsertedLines(t *testing.T) {
	m := readyModel(withDiff)
	m.output = "line1\nline2"
	m.prevOutput = "line1"

	content := m.buildContent()
	// line1 should be unchanged (prefixed with spaces)
	if !strings.Contains(content, "  line1") {
		t.Errorf("expected unchanged line to be indented, got %q", content)
	}
	// line2 should be marked as inserted
	if !strings.Contains(content, "line2") {
		t.Errorf("expected inserted line2 in output, got %q", content)
	}
}

func TestDiffModeCollapsesDeletedLines(t *testing.T) {
	m := readyModel(withDiff)
	m.output = "line1"
	m.prevOutput = "line1\nline2\nline3\nline4"

	content := m.buildContent()
	if !strings.Contains(content, "3 lines removed") {
		t.Errorf("expected collapsed delete summary, got %q", content)
	}
}

func TestDiffModeSingleDeletedLine(t *testing.T) {
	m := readyModel(withDiff)
	m.output = "line1\nline3"
	m.prevOutput = "line1\nline2\nline3"

	content := m.buildContent()
	if !strings.Contains(content, "1 line removed") {
		t.Errorf("expected '1 line removed', got %q", content)
	}
	// Should NOT say "lines" (plural)
	if strings.Contains(content, "1 lines") {
		t.Errorf("singular should not use plural, got %q", content)
	}
}

func TestDiffModeReplacementShowsChange(t *testing.T) {
	m := readyModel(withDiff)
	m.output = "line1\nline2-new\nline3"
	m.prevOutput = "line1\nline2-old\nline3"

	content := m.buildContent()
	// Single replacement should show as change, not removal + insertion
	if strings.Contains(content, "removed") {
		t.Errorf("single-line replacement should not show removal marker, got %q", content)
	}
	if !strings.Contains(content, "line2-new") {
		t.Errorf("expected replaced line content in output, got %q", content)
	}
}

func TestDiffModeMovedLinesNotMarked(t *testing.T) {
	m := readyModel(withDiff)
	// Insert a line at the top — lines below should NOT be marked changed
	m.prevOutput = "aaa\nbbb\nccc"
	m.output = "new\naaa\nbbb\nccc"

	diffs := computeDiff(
		strings.Split(m.prevOutput, "\n"),
		strings.Split(m.output, "\n"),
	)
	// aaa, bbb, ccc should all be diffEqual
	equalCount := 0
	for _, d := range diffs {
		if d.op == diffEqual {
			equalCount++
		}
	}
	if equalCount != 3 {
		t.Errorf("expected 3 equal lines (aaa,bbb,ccc), got %d equals in %v", equalCount, diffs)
	}
}

// ─── GNU watch feature: -d initial render has consistent margin ─────────────

func TestDiffModeInitialRenderHasMargin(t *testing.T) {
	m := readyModel(withDiff)
	m.output = "line1\nline2"
	m.prevOutput = "" // no previous output yet

	content := m.buildContent()
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("line %d missing 2-char margin on initial diff render: %q", i, line)
		}
	}
}

// ─── GNU watch feature: header shows interval and command ───────────────────

func TestHeaderShowsIntervalAndCommand(t *testing.T) {
	m := readyModel()
	m.width = 120
	header := m.renderHeader()
	if !strings.Contains(header, "2.0s") {
		t.Errorf("header should show interval, got %q", header)
	}
	if !strings.Contains(header, "echo") {
		t.Errorf("header should show command, got %q", header)
	}
}

func TestHeaderShowsCommandWithArgs(t *testing.T) {
	m := initialModel("ls", []string{"-la", "/tmp"}, 2*time.Second, false, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	um := updated.(model)
	header := um.renderHeader()
	if !strings.Contains(header, "ls -la /tmp") {
		t.Errorf("header should show full command, got %q", header)
	}
}

// ─── GNU watch feature: header shows current time ───────────────────────────

func TestHeaderShowsTimeAfterRun(t *testing.T) {
	m := readyModel()
	m.lastRun = time.Date(2025, 6, 15, 14, 30, 45, 0, time.Local)
	m.width = 120
	header := m.renderHeader()
	if !strings.Contains(header, "14:30:45") {
		t.Errorf("header should show last run time, got %q", header)
	}
}

// ─── GNU watch feature: -t / --no-title hides header (we use fullscreen) ────

func TestFullscreenHidesHeaderAndFooter(t *testing.T) {
	m := readyModel()
	m.output = "some output"
	m.updateViewportContent()

	// Toggle fullscreen
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'f'})
	um := updated.(model)

	if !um.fullscreen {
		t.Fatal("expected fullscreen=true after pressing f")
	}

	v := um.View()
	content := v.Content
	// In fullscreen, header (with "Every") and footer (help bar) should be absent
	if strings.Contains(content, "Every") {
		t.Error("fullscreen view should not contain header")
	}
}

func TestFullscreenTogglesBackToNormal(t *testing.T) {
	m := readyModel()
	m.output = "some output"
	m.updateViewportContent()

	// Toggle on
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'f'})
	// Toggle off
	updated, _ = updated.(model).Update(tea.KeyPressMsg{Code: 'f'})
	um := updated.(model)

	if um.fullscreen {
		t.Error("expected fullscreen=false after double toggle")
	}
}

// ─── GNU watch feature: runs until interrupted (q / ctrl+c) ─────────────────

func TestQuitOnQ(t *testing.T) {
	m := readyModel()
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	// Verify it produces a QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

// ─── Our extension: +/- adjusts interval ────────────────────────────────────

func TestFasterSlower(t *testing.T) {
	m := readyModel()
	original := m.interval

	// Press + to go faster
	updated, _ := m.Update(tea.KeyPressMsg{Code: '+'})
	um := updated.(model)
	if um.interval >= original {
		t.Errorf("expected interval to decrease after +, got %v (was %v)", um.interval, original)
	}

	// Press - to go slower
	updated, _ = um.Update(tea.KeyPressMsg{Code: '-'})
	um = updated.(model)
	if um.interval != original {
		t.Errorf("expected interval back to original after + then -, got %v", um.interval)
	}
}

func TestFasterClampsAtMinimum(t *testing.T) {
	m := readyModel()
	m.interval = 200 * time.Millisecond

	updated, _ := m.Update(tea.KeyPressMsg{Code: '+'})
	um := updated.(model)
	// Should not go below 200ms
	if um.interval < 200*time.Millisecond {
		t.Errorf("interval went below minimum: %v", um.interval)
	}
}

// ─── Our extension: space forces immediate refresh ──────────────────────────

func TestSpaceTriggersRefresh(t *testing.T) {
	m := readyModel()
	updated, cmd := m.Update(tea.KeyPressMsg{Code: ' '})
	um := updated.(model)
	if !um.executing {
		t.Error("expected executing=true after space")
	}
	if cmd == nil {
		t.Error("expected command to be returned for execution")
	}
}

// ─── Our extension: d toggles diff mode at runtime ──────────────────────────

func TestDTogglesDiffMode(t *testing.T) {
	m := readyModel()
	if m.diffMode {
		t.Fatal("diff should be off by default")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd'})
	if !updated.(model).diffMode {
		t.Error("expected diffMode=true after pressing d")
	}

	updated, _ = updated.(model).Update(tea.KeyPressMsg{Code: 'd'})
	if updated.(model).diffMode {
		t.Error("expected diffMode=false after pressing d again")
	}
}

// ─── GNU watch feature: alt screen / fullscreen mode ────────────────────────

func TestViewUsesAltScreen(t *testing.T) {
	m := readyModel()
	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
}

// ─── GNU watch feature: window resize is handled ────────────────────────────

func TestWindowResize(t *testing.T) {
	m := readyModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	um := updated.(model)
	if um.width != 120 || um.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", um.width, um.height)
	}
}

// ─── GNU watch feature: command error is reported ───────────────────────────

func TestCommandErrorIsStored(t *testing.T) {
	m := readyModel()
	updated, _ := m.Update(commandOutputMsg{
		output: "error output",
		err:    fmt.Errorf("exit status 1"),
	})
	um := updated.(model)
	if um.err == nil {
		t.Error("expected err to be set")
	}
	if um.output != "error output" {
		t.Errorf("expected error output to be stored, got %q", um.output)
	}
}

func TestCommandErrorClearedOnSuccess(t *testing.T) {
	m := readyModel()
	// First: error
	updated, _ := m.Update(commandOutputMsg{output: "fail", err: fmt.Errorf("err")})
	// Then: success
	updated, _ = updated.(model).Update(commandOutputMsg{output: "ok", err: nil})
	um := updated.(model)
	if um.err != nil {
		t.Error("expected err to be cleared after successful run")
	}
}

// ─── Diff algorithm: LCS correctness ────────────────────────────────────────

func TestComputeDiffIdentical(t *testing.T) {
	old := []string{"a", "b", "c"}
	result := computeDiff(old, old)
	for _, d := range result {
		if d.op != diffEqual {
			t.Errorf("expected all equal for identical input, got %v", d.op)
		}
	}
}

func TestComputeDiffAllNew(t *testing.T) {
	result := computeDiff(nil, []string{"a", "b"})
	for _, d := range result {
		if d.op != diffInsert {
			t.Errorf("expected all inserts for empty old, got %v", d.op)
		}
	}
	if len(result) != 2 {
		t.Errorf("expected 2 inserts, got %d", len(result))
	}
}

func TestComputeDiffAllDeleted(t *testing.T) {
	result := computeDiff([]string{"a", "b"}, nil)
	for _, d := range result {
		if d.op != diffDelete {
			t.Errorf("expected all deletes for empty new, got %v", d.op)
		}
	}
}

func TestComputeDiffInsertInMiddle(t *testing.T) {
	old := []string{"a", "c"}
	new := []string{"a", "b", "c"}
	result := computeDiff(old, new)

	if len(result) != 3 {
		t.Fatalf("expected 3 ops, got %d: %v", len(result), result)
	}
	if result[0].op != diffEqual || result[0].text != "a" {
		t.Errorf("expected equal 'a', got %v", result[0])
	}
	if result[1].op != diffInsert || result[1].text != "b" {
		t.Errorf("expected insert 'b', got %v", result[1])
	}
	if result[2].op != diffEqual || result[2].text != "c" {
		t.Errorf("expected equal 'c', got %v", result[2])
	}
}

func TestComputeDiffDeleteFromMiddle(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "c"}
	result := computeDiff(old, new)

	if len(result) != 3 {
		t.Fatalf("expected 3 ops, got %d: %v", len(result), result)
	}
	if result[0].op != diffEqual || result[0].text != "a" {
		t.Errorf("expected equal 'a', got %v", result[0])
	}
	if result[1].op != diffDelete || result[1].text != "b" {
		t.Errorf("expected delete 'b', got %v", result[1])
	}
	if result[2].op != diffEqual || result[2].text != "c" {
		t.Errorf("expected equal 'c', got %v", result[2])
	}
}

// ─── Key bindings are correctly configured ──────────────────────────────────

func TestKeyBindings(t *testing.T) {
	tests := []struct {
		name    string
		binding key.Binding
		keys    []string
	}{
		{"quit", keys.Quit, []string{"q", "ctrl+c"}},
		{"suspend", keys.Suspend, []string{"ctrl+z"}},
		{"refresh", keys.Refresh, []string{"space"}},
		{"faster", keys.Faster, []string{"+", "="}},
		{"slower", keys.Slower, []string{"-"}},
		{"diff", keys.ToggleDiff, []string{"d"}},
		{"no-wrap", keys.ToggleWrap, []string{"w"}},
		{"fullscreen", keys.Fullscreen, []string{"f"}},
		{"help", keys.Help, []string{"?"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.binding.Keys()
			if len(got) != len(tt.keys) {
				t.Errorf("expected keys %v, got %v", tt.keys, got)
				return
			}
			for i, k := range tt.keys {
				if got[i] != k {
					t.Errorf("key %d: expected %q, got %q", i, k, got[i])
				}
			}
		})
	}
}

// ─── Help view provides short and full help ─────────────────────────────────

func TestShortHelpContainsAllBindings(t *testing.T) {
	short := keys.ShortHelp()
	// Should have: quit, refresh, faster, slower, diff, no-wrap, fullscreen, help
	if len(short) < 8 {
		t.Errorf("expected at least 8 short help bindings, got %d", len(short))
	}
}

func TestFullHelpIsGrouped(t *testing.T) {
	full := keys.FullHelp()
	if len(full) != 3 {
		t.Errorf("expected 3 help columns, got %d", len(full))
	}
}

// ─── Init returns commands for first run, tick, and spinner ─────────────────

func TestInitReturnsBatchCmd(t *testing.T) {
	m := initialModel("echo", []string{"test"}, 2*time.Second, false, false)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a batch command (runCommand + tick + spinner)")
	}
}

// ─── No-wrap mode sets viewport SoftWrap ────────────────────────────────────

func TestNoWrapDisablesSoftWrap(t *testing.T) {
	m := readyModel(func(m *model) { m.noWrap = true })
	if m.viewport.SoftWrap {
		t.Error("expected SoftWrap=false when noWrap=true")
	}
}

func TestDefaultEnablesSoftWrap(t *testing.T) {
	m := readyModel()
	if !m.viewport.SoftWrap {
		t.Error("expected SoftWrap=true by default")
	}
}

func TestWKeyTogglesSoftWrap(t *testing.T) {
	m := readyModel()
	if !m.viewport.SoftWrap {
		t.Fatal("expected SoftWrap=true initially")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'w'})
	um := updated.(model)
	if um.viewport.SoftWrap {
		t.Error("expected SoftWrap=false after pressing w")
	}
	if !um.noWrap {
		t.Error("expected noWrap=true after pressing w")
	}

	updated, _ = um.Update(tea.KeyPressMsg{Code: 'w'})
	um = updated.(model)
	if !um.viewport.SoftWrap {
		t.Error("expected SoftWrap=true after pressing w again")
	}
}

// ─── Diff with Scandinavian characters ──────────────────────────────────────

func TestComputeDiffWithScandinavianLines(t *testing.T) {
	old := []string{"blåbær", "rødgrøt", "ærlig"}
	new := []string{"blåbær", "grønnkål", "ærlig"}
	result := computeDiff(old, new)

	if len(result) != 4 {
		t.Fatalf("expected 4 ops, got %d: %v", len(result), result)
	}
	if result[0].op != diffEqual || result[0].text != "blåbær" {
		t.Errorf("expected equal 'blåbær', got %v", result[0])
	}
	if result[1].op != diffDelete || result[1].text != "rødgrøt" {
		t.Errorf("expected delete 'rødgrøt', got %v", result[1])
	}
	if result[2].op != diffInsert || result[2].text != "grønnkål" {
		t.Errorf("expected insert 'grønnkål', got %v", result[2])
	}
	if result[3].op != diffEqual || result[3].text != "ærlig" {
		t.Errorf("expected equal 'ærlig', got %v", result[3])
	}
}

// ─── Previous output is preserved for diff ──────────────────────────────────

func TestPrevOutputPreserved(t *testing.T) {
	m := readyModel()

	updated, _ := m.Update(commandOutputMsg{output: "first"})
	updated, _ = updated.(model).Update(commandOutputMsg{output: "second"})
	um := updated.(model)

	if um.prevOutput != "first" {
		t.Errorf("expected prevOutput='first', got %q", um.prevOutput)
	}
	if um.output != "second" {
		t.Errorf("expected output='second', got %q", um.output)
	}
}

// ─── lastRun is set after command output ────────────────────────────────────

func TestLastRunSetAfterOutput(t *testing.T) {
	m := readyModel()
	if !m.lastRun.IsZero() {
		t.Error("lastRun should be zero initially")
	}

	before := time.Now()
	updated, _ := m.Update(commandOutputMsg{output: "x"})
	after := time.Now()
	um := updated.(model)

	if um.lastRun.Before(before) || um.lastRun.After(after) {
		t.Errorf("lastRun %v not between %v and %v", um.lastRun, before, after)
	}
}
