package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"viola/internal/browser"
	"viola/internal/newsapi"
	"viola/internal/output"
	"viola/internal/render"
	"viola/internal/tui"
)

const rootHelp = `viola reads FirenzeViola news from the private mobile API.

Usage:
  viola <command> [flags]
  viola help [command]

Commands:
  news          Show latest news, search, or read an article
  help          Show help for a command

Examples:
  viola news
  viola news --plain --limit 10
  viola news --search rakow
  viola news --read firenzeviola.it-472733
  viola news --json
`

const newsHelp = `Usage:
  viola news [flags]

Flags:
  --search string   Search query, minimum 2 characters
  --limit int       Number of articles to fetch (default 30, max 100)
  --plain           Force plain text output
  --json            Print JSON instead of plain text or TUI
  --read string     Read one article by id

Examples:
  viola news
  viola news --search rakow
  viola news --limit 10 --plain
  viola news --read firenzeviola.it-472733
  viola news --read firenzeviola.it-472733 --json
`

type NewsService interface {
	FetchArticles(context.Context, int, int) ([]newsapi.ArticleSummary, error)
	SearchArticles(context.Context, string, int, int) ([]newsapi.ArticleSummary, error)
	FetchArticleDetail(context.Context, string) (newsapi.ArticleDetail, error)
}

type TUIRunner func(tui.Model, io.Writer) error
type IsTTYFunc func(*osFile) bool

type osFile interface {
	Fd() uintptr
}

type App struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Context    context.Context
	Service    NewsService
	Opener     browser.Opener
	RunTUI     TUIRunner
	IsTTY      func(io.Reader, io.Writer) bool
	HTTPClient *http.Client
}

type exitError struct {
	Code    int
	Message string
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	app := &App{
		Stdin:      stdin,
		Stdout:     stdout,
		Stderr:     stderr,
		Context:    context.Background(),
		Service:    newsapi.New(nil),
		Opener:     browser.SystemOpener{},
		HTTPClient: http.DefaultClient,
		RunTUI: func(model tui.Model, out io.Writer) error {
			program := tea.NewProgram(model, tea.WithOutput(out), tea.WithAltScreen())
			_, err := program.Run()
			return err
		},
		IsTTY: func(in io.Reader, out io.Writer) bool {
			stdinFile, inOK := in.(osFile)
			stdoutFile, outOK := out.(osFile)
			return inOK && outOK && term.IsTerminal(int(stdinFile.Fd())) && term.IsTerminal(int(stdoutFile.Fd()))
		},
	}

	if err := app.Execute(args); err != nil {
		var coded *exitError
		if errors.As(err, &coded) {
			if coded.Message != "" {
				output.Errorf(stderr, "%s", coded.Message)
			}
			return coded.Code
		}
		output.Errorf(stderr, "%v", err)
		return 1
	}
	return 0
}

func (a *App) Execute(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(a.Stdout, rootHelp)
		return &exitError{Code: 2, Message: "missing command"}
	}

	switch args[0] {
	case "help", "--help", "-h":
		if len(args) > 1 && args[1] == "news" {
			fmt.Fprint(a.Stdout, newsHelp)
			return nil
		}
		fmt.Fprint(a.Stdout, rootHelp)
		return nil
	case "news":
		return a.runNews(args[1:])
	default:
		fmt.Fprint(a.Stdout, rootHelp)
		return &exitError{Code: 2, Message: fmt.Sprintf("unknown command %q", args[0])}
	}
}

func (a *App) runNews(args []string) error {
	flags := flag.NewFlagSet("news", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var search string
	var limit int
	var plain bool
	var jsonMode bool
	var readID string

	flags.StringVar(&search, "search", "", "")
	flags.IntVar(&limit, "limit", 30, "")
	flags.BoolVar(&plain, "plain", false, "")
	flags.BoolVar(&jsonMode, "json", false, "")
	flags.StringVar(&readID, "read", "", "")

	if err := flags.Parse(args); err != nil {
		return &exitError{Code: 2, Message: err.Error()}
	}

	trimmedQuery := strings.TrimSpace(search)
	if trimmedQuery != "" && len([]rune(trimmedQuery)) < 2 {
		return &exitError{Code: 2, Message: "search query must be at least 2 characters"}
	}
	if limit < 1 || limit > 100 {
		return &exitError{Code: 2, Message: "limit must be between 1 and 100"}
	}

	if strings.TrimSpace(readID) != "" {
		return a.runRead(strings.TrimSpace(readID), jsonMode)
	}

	if jsonMode {
		articles, err := a.fetchList(trimmedQuery, 0, limit)
		if err != nil {
			return err
		}
		return output.JSON(a.Stdout, articles)
	}

	if plain || !a.IsTTY(a.Stdin, a.Stdout) {
		articles, err := a.fetchList(trimmedQuery, 0, limit)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(a.Stdout, formatPlainList(articles))
		return err
	}

	model := tui.New(
		func(ctx context.Context, query string, start, step int) ([]newsapi.ArticleSummary, error) {
			if strings.TrimSpace(query) == "" {
				return a.Service.FetchArticles(ctx, start, step)
			}
			return a.Service.SearchArticles(ctx, query, start, step)
		},
		a.Service.FetchArticleDetail,
		a.Opener,
		trimmedQuery,
		limit,
	)
	return a.RunTUI(model, a.Stdout)
}

func (a *App) runRead(id string, jsonMode bool) error {
	detail, err := a.Service.FetchArticleDetail(a.Context, id)
	if err != nil {
		return err
	}
	out := render.BuildDetailOutput(detail)
	if jsonMode {
		return output.JSON(a.Stdout, out)
	}
	_, err = fmt.Fprint(a.Stdout, render.FormatPlainDetail(out))
	return err
}

func (a *App) fetchList(query string, start, limit int) ([]newsapi.ArticleSummary, error) {
	if strings.TrimSpace(query) == "" {
		return a.Service.FetchArticles(a.Context, start, limit)
	}
	return a.Service.SearchArticles(a.Context, query, start, limit)
}

func formatPlainList(articles []newsapi.ArticleSummary) string {
	var builder strings.Builder
	for index, article := range articles {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(article.Title)
		builder.WriteString("\n")
		builder.WriteString("ID: ")
		builder.WriteString(article.ID)
		builder.WriteString("\n")
		if section := strings.TrimSpace(article.Section); section != "" {
			builder.WriteString("Section: ")
			builder.WriteString(section)
			builder.WriteString("\n")
		}
		if date := render.FormatListDate(article.Date); date != "" {
			builder.WriteString("Date: ")
			builder.WriteString(date)
			builder.WriteString("\n")
		}
		builder.WriteString("Link: ")
		builder.WriteString(article.URL)
	}
	builder.WriteString("\n")
	return builder.String()
}

func (e *exitError) Error() string {
	return e.Message
}
