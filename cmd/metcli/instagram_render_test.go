package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/steipete/metcli/internal/instagram"
)

func TestCompactWhitespace(t *testing.T) {
	input := "hello \n  world\t\tfoo"
	if got := compactWhitespace(input); got != "hello world foo" {
		t.Fatalf("unexpected compactWhitespace: %q", got)
	}
	if got := compactWhitespace("   "); got != "" {
		t.Fatalf("expected empty for whitespace-only, got %q", got)
	}
}

func TestRenderItemTextStripsTerminalControls(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	renderItemText(writer, instagram.MediaItem{
		Username: "ann\x1b[31m",
		Caption:  "hello\x1b[0m world\u009b",
	})
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.ContainsRune(got, '\x1b') || strings.ContainsRune(got, '\u009b') {
		t.Fatalf("control bytes leaked: %q", got)
	}
	if !strings.Contains(got, "@ann") || !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("visible text missing: %q", got)
	}
}

func TestInlineName(t *testing.T) {
	if got := inlineName(""); got != "instagram.img" {
		t.Fatalf("expected default name, got %q", got)
	}
	if got := inlineName("abc"); got != "abc.img" {
		t.Fatalf("expected abc.img, got %q", got)
	}
	if got := inlineName("foo/bar"); got != "bar.img" {
		t.Fatalf("expected bar.img, got %q", got)
	}
}

func TestEstimateRows(t *testing.T) {
	rows := estimateRows(10, 100, 200, 0.5)
	if rows != 10 {
		t.Fatalf("expected 10 rows, got %d", rows)
	}
	if rows := estimateRows(0, 100, 200, 0.5); rows != 0 {
		t.Fatalf("expected 0 rows for invalid input, got %d", rows)
	}
}
