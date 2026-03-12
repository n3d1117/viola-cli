package render

import (
	"strings"
	"testing"
)

func TestNormalizeHTML(t *testing.T) {
	t.Parallel()

	input := `<p>Hello&nbsp;<u>world</u></p><script>alert(1)</script><p><strong>ciao</strong>again</p>`
	output := NormalizeHTML(input)
	if strings.Contains(output, "script") {
		t.Fatalf("expected script to be removed: %q", output)
	}
	if !strings.Contains(output, "<em>world</em>") {
		t.Fatalf("expected u tags to become em: %q", output)
	}
	if !strings.Contains(output, "</strong> again") {
		t.Fatalf("expected missing space fix: %q", output)
	}
}

func TestHTMLToMarkdownFallsBackToText(t *testing.T) {
	t.Parallel()

	output := HTMLToMarkdown("<p>Hello</p><p>World</p>")
	if !strings.Contains(output, "Hello") || !strings.Contains(output, "World") {
		t.Fatalf("unexpected markdown: %q", output)
	}
}

func TestResolveGlamourStyleDefaultsToDark(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GLAMOUR_STYLE", "")

	if got := resolveGlamourStyle(); got != "dark" {
		t.Fatalf("expected dark style, got %q", got)
	}
}

func TestResolveGlamourStyleUsesExplicitStyle(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GLAMOUR_STYLE", "light")

	if got := resolveGlamourStyle(); got != "light" {
		t.Fatalf("expected light style, got %q", got)
	}
}

func TestResolveGlamourStyleDoesNotUseAuto(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GLAMOUR_STYLE", "auto")

	if got := resolveGlamourStyle(); got != "dark" {
		t.Fatalf("expected auto to resolve to dark, got %q", got)
	}
}

func TestResolveGlamourStyleRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GLAMOUR_STYLE", "")

	if got := resolveGlamourStyle(); got != "notty" {
		t.Fatalf("expected notty style, got %q", got)
	}
}

func TestRenderMarkdownUsesResolvedStyle(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GLAMOUR_STYLE", "auto")

	output := RenderMarkdown("# Title\n\nText", 80)
	if strings.TrimSpace(output) == "" {
		t.Fatal("expected rendered markdown output")
	}

	if strings.Contains(output, "\x1b]11;") {
		t.Fatalf("expected no OSC 11 sequence in output: %q", output)
	}
}
