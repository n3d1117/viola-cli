package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"viola/internal/newsapi"
)

type fakeOpener struct {
	targets []string
}

func (f *fakeOpener) Open(target string) error {
	f.targets = append(f.targets, target)
	return nil
}

func TestModelTransitionsToReader(t *testing.T) {
	t.Parallel()

	model := New(
		func(context.Context, string, int, int) ([]newsapi.ArticleSummary, error) {
			return []newsapi.ArticleSummary{{ID: "a-1", Title: "One", URL: "https://example.com/a-1"}}, nil
		},
		func(context.Context, string) (newsapi.ArticleDetail, error) {
			return newsapi.ArticleDetail{ID: "a-1", Title: "One", Text: "<p>Body</p>", URL: "https://example.com/a-1"}, nil
		},
		&fakeOpener{},
		"",
		10,
	)

	msg := model.Init()()
	updated, _ := model.Update(msg)
	model = updated.(Model)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}

	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.mode != ModeReader {
		t.Fatalf("expected reader mode")
	}
	if model.selected == nil || model.selected.ID != "a-1" {
		t.Fatalf("unexpected selected item: %+v", model.selected)
	}
}

func TestModelOpenSelectedURL(t *testing.T) {
	t.Parallel()

	opener := &fakeOpener{}
	model := New(
		func(context.Context, string, int, int) ([]newsapi.ArticleSummary, error) {
			return []newsapi.ArticleSummary{{ID: "a-1", Title: "One", URL: "https://example.com/a-1"}}, nil
		},
		func(context.Context, string) (newsapi.ArticleDetail, error) {
			return newsapi.ArticleDetail{}, nil
		},
		opener,
		"",
		10,
	)

	updated, _ := model.Update(model.Init()())
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Runes: []rune("o"), Type: tea.KeyRunes})
	model = updated.(Model)
	if cmd == nil {
		t.Fatalf("expected open command")
	}
	_ = cmd()
	if len(opener.targets) != 1 || opener.targets[0] != "https://example.com/a-1" {
		t.Fatalf("unexpected targets: %+v", opener.targets)
	}
}
