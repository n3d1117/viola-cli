package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"viola/internal/browser"
	"viola/internal/newsapi"
	"viola/internal/render"
)

type FetchListFunc func(context.Context, string, int, int) ([]newsapi.ArticleSummary, error)
type FetchDetailFunc func(context.Context, string) (newsapi.ArticleDetail, error)

type Mode int

const (
	ModeList Mode = iota
	ModeReader
)

type articleItem struct {
	article newsapi.ArticleSummary
}

func (i articleItem) FilterValue() string {
	return strings.TrimSpace(i.article.Title + " " + i.article.Section + " " + i.article.URL)
}

func (i articleItem) Title() string {
	return i.article.Title
}

func (i articleItem) Description() string {
	tag := strings.TrimSpace(i.article.Badge)
	if tag == "" {
		tag = strings.TrimSpace(i.article.Section)
	}
	date := render.FormatListDate(i.article.Date)
	parts := make([]string, 0, 3)
	if tag != "" {
		parts = append(parts, tag)
	}
	if date != "" {
		parts = append(parts, date)
	}
	if strings.TrimSpace(i.article.URL) != "" {
		parts = append(parts, i.article.URL)
	}
	return strings.Join(parts, " | ")
}

type listLoadedMsg struct {
	articles []newsapi.ArticleSummary
	query    string
	reset    bool
	err      error
}

type detailLoadedMsg struct {
	detail newsapi.ArticleDetail
	err    error
}

type errMsg struct {
	err error
}

type Model struct {
	width       int
	height      int
	mode        Mode
	list        list.Model
	search      textinput.Model
	reader      viewport.Model
	fetchList   FetchListFunc
	fetchDetail FetchDetailFunc
	opener      browser.Opener
	pageSize    int
	nextStart   int
	query       string
	articles    []newsapi.ArticleSummary
	selected    *render.DetailOutput
	loading     bool
	errText     string
}

func New(fetchList FetchListFunc, fetchDetail FetchDetailFunc, opener browser.Opener, query string, pageSize int) Model {
	items := []list.Item{}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	listModel := list.New(items, delegate, 0, 0)
	listModel.Title = "Viola News"
	listModel.SetShowStatusBar(true)
	listModel.SetFilteringEnabled(false)
	listModel.SetShowHelp(false)

	search := textinput.New()
	search.Placeholder = "Search news"
	search.SetValue(query)

	reader := viewport.New(0, 0)

	return Model{
		mode:        ModeList,
		list:        listModel,
		search:      search,
		reader:      reader,
		fetchList:   fetchList,
		fetchDetail: fetchDetail,
		opener:      opener,
		pageSize:    pageSize,
		query:       strings.TrimSpace(query),
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadList(true)
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, max(msg.Height-4, 6))
		m.reader.Width = max(msg.Width-4, 20)
		m.reader.Height = max(msg.Height-6, 6)
		if m.selected != nil {
			m.reader.SetContent(m.renderReader(*m.selected))
		}
		return m, nil

	case listLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, nil
		}
		if msg.reset {
			m.articles = dedupe(msg.articles)
		} else {
			m.articles = dedupe(append(m.articles, msg.articles...))
		}
		m.nextStart = len(m.articles)
		items := make([]list.Item, 0, len(m.articles))
		for _, article := range m.articles {
			items = append(items, articleItem{article: article})
		}
		m.list.SetItems(items)
		m.errText = ""
		m.list.Title = titleForQuery(msg.query)
		return m, nil

	case detailLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errText = msg.err.Error()
			return m, nil
		}
		output := render.BuildDetailOutput(msg.detail)
		m.selected = &output
		m.mode = ModeReader
		m.reader.SetContent(m.renderReader(output))
		m.errText = ""
		return m, nil

	case errMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
		}
		return m, nil

	case tea.KeyMsg:
		if m.mode == ModeReader {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "b", "esc":
				m.mode = ModeList
				return m, nil
			case "o":
				if m.selected != nil && strings.TrimSpace(m.selected.URL) != "" {
					return m, openURL(m.opener, m.selected.URL)
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.reader, cmd = m.reader.Update(msg)
			return m, cmd
		}

		if m.search.Focused() {
			switch msg.String() {
			case "enter":
				query := strings.TrimSpace(m.search.Value())
				if query != "" && len([]rune(query)) < 2 {
					m.errText = "search query must be at least 2 characters"
					m.search.Blur()
					return m, nil
				}
				m.query = query
				m.search.Blur()
				return m, m.loadList(true)
			case "esc":
				m.search.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.search.Focus()
			return m, nil
		case "r":
			return m, m.loadList(true)
		case "n":
			return m, m.loadList(false)
		case "enter":
			if item, ok := m.list.SelectedItem().(articleItem); ok {
				m.loading = true
				return m, loadDetailCmd(m.fetchDetail, item.article.ID)
			}
		case "o":
			if item, ok := m.list.SelectedItem().(articleItem); ok && strings.TrimSpace(item.article.URL) != "" {
				return m, openURL(m.opener, item.article.URL)
			}
		}
	}

	if m.mode == ModeList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(message)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	status := ""
	if m.loading {
		status = "Loading..."
	} else if m.errText != "" {
		status = "Error: " + m.errText
	} else if m.search.Focused() {
		status = "Search: " + m.search.View()
	} else if m.mode == ModeList {
		status = "enter read • / search • r refresh • n next • o open • q quit"
	} else {
		status = "b back • o open • q quit"
	}

	if m.mode == ModeReader {
		return m.reader.View() + "\n\n" + status
	}
	return m.list.View() + "\n" + status
}

func (m Model) loadList(reset bool) tea.Cmd {
	if m.loading {
		return nil
	}
	m.loading = true
	start := m.nextStart
	if reset {
		start = 0
	}
	return loadListCmd(m.fetchList, m.query, start, m.pageSize, reset)
}

func (m Model) renderReader(output render.DetailOutput) string {
	parts := []string{output.Title}
	if output.Section != "" {
		parts = append(parts, "["+output.Section+"]")
	}
	if output.Date != "" {
		parts = append(parts, render.FormatListDate(output.Date))
	}
	if output.URL != "" {
		parts = append(parts, output.URL)
	}
	body := render.RenderMarkdown(output.TextMarkdown, max(m.reader.Width, 60))
	parts = append(parts, body)
	if len(output.VideoURLs) > 0 || len(output.AudioItems) > 0 {
		parts = append(parts, "Media")
		for _, url := range output.VideoURLs {
			parts = append(parts, "- video: "+url)
		}
		for _, item := range output.AudioItems {
			label := strings.TrimSpace(item.Description)
			if label == "" {
				label = strings.TrimSpace(item.Source)
			}
			if label == "" {
				label = "audio"
			}
			parts = append(parts, fmt.Sprintf("- %s: %s", label, item.URL))
		}
	}
	return strings.Join(parts, "\n\n")
}

func loadListCmd(fetch FetchListFunc, query string, start, step int, reset bool) tea.Cmd {
	return func() tea.Msg {
		articles, err := fetch(context.Background(), query, start, step)
		return listLoadedMsg{articles: articles, query: query, reset: reset, err: err}
	}
}

func loadDetailCmd(fetch FetchDetailFunc, id string) tea.Cmd {
	return func() tea.Msg {
		detail, err := fetch(context.Background(), id)
		return detailLoadedMsg{detail: detail, err: err}
	}
}

func openURL(opener browser.Opener, url string) tea.Cmd {
	return func() tea.Msg {
		if opener == nil {
			return nil
		}
		if err := opener.Open(url); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func titleForQuery(query string) string {
	if strings.TrimSpace(query) == "" {
		return "Viola News"
	}
	return "Viola News: " + query
}

func dedupe(articles []newsapi.ArticleSummary) []newsapi.ArticleSummary {
	seen := make(map[string]struct{}, len(articles))
	out := make([]newsapi.ArticleSummary, 0, len(articles))
	for _, article := range articles {
		if _, ok := seen[article.ID]; ok {
			continue
		}
		seen[article.ID] = struct{}{}
		out = append(out, article)
	}
	return out
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
