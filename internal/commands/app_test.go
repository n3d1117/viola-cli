package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"viola/internal/browser"
	"viola/internal/newsapi"
	"viola/internal/tui"
)

type fakeService struct {
	articles []newsapi.ArticleSummary
	detail   newsapi.ArticleDetail
	lastQ    string
}

func (f *fakeService) FetchArticles(context.Context, int, int) ([]newsapi.ArticleSummary, error) {
	return f.articles, nil
}

func (f *fakeService) SearchArticles(_ context.Context, query string, _, _ int) ([]newsapi.ArticleSummary, error) {
	f.lastQ = query
	return f.articles, nil
}

func (f *fakeService) FetchArticleDetail(context.Context, string) (newsapi.ArticleDetail, error) {
	return f.detail, nil
}

func TestNewsPlain(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := &App{
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Context: context.Background(),
		Service: &fakeService{
			articles: []newsapi.ArticleSummary{{ID: "a-1", Title: "Hello", URL: "https://example.com/a-1", Section: "Primo Piano", Date: "2026-03-11T19:45:03+01:00"}},
		},
		IsTTY: func(io.Reader, io.Writer) bool { return false },
	}

	if err := app.Execute([]string{"news", "--plain"}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "https://example.com/a-1") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestNewsJSON(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := &App{
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Context: context.Background(),
		Service: &fakeService{
			articles: []newsapi.ArticleSummary{{ID: "a-1", Title: "Hello", URL: "https://example.com/a-1"}},
		},
	}

	if err := app.Execute([]string{"news", "--json"}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var payload []newsapi.ArticleSummary
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload) != 1 || payload[0].ID != "a-1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestNewsSearchPlain(t *testing.T) {
	t.Parallel()

	service := &fakeService{
		articles: []newsapi.ArticleSummary{{ID: "a-1", Title: "Rakow", URL: "https://example.com/a-1"}},
	}
	app := &App{
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Context: context.Background(),
		Service: service,
		IsTTY:   func(io.Reader, io.Writer) bool { return false },
	}

	if err := app.Execute([]string{"news", "--search", "rakow", "--plain"}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if service.lastQ != "rakow" {
		t.Fatalf("unexpected search query: %q", service.lastQ)
	}
}

func TestNewsRead(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := &App{
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Context: context.Background(),
		Service: &fakeService{
			detail: newsapi.ArticleDetail{ID: "a-1", Title: "Hello", Text: "<p>Body</p>", URL: "https://example.com/a-1"},
		},
	}

	if err := app.Execute([]string{"news", "--read", "a-1"}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "Body") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestNewsShortQueryFails(t *testing.T) {
	t.Parallel()

	app := &App{
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Context: context.Background(),
		Service: &fakeService{},
	}

	err := app.Execute([]string{"news", "--search", "a"})
	if err == nil || !strings.Contains(err.Error(), "at least 2 characters") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewsInteractiveUsesRunner(t *testing.T) {
	t.Parallel()

	called := false
	app := &App{
		Stdin:   bytes.NewBuffer(nil),
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Context: context.Background(),
		Service: &fakeService{},
		Opener:  browser.SystemOpener{},
		IsTTY:   func(io.Reader, io.Writer) bool { return true },
		RunTUI: func(model tui.Model, out io.Writer) error {
			called = true
			return nil
		},
	}

	if err := app.Execute([]string{"news"}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !called {
		t.Fatalf("expected TUI runner to be called")
	}
}
