package render

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/charmbracelet/glamour"
	"golang.org/x/net/html"

	"viola/internal/newsapi"
)

type DetailOutput struct {
	ID           string                   `json:"id"`
	Date         string                   `json:"date"`
	Title        string                   `json:"title"`
	Section      string                   `json:"section,omitempty"`
	Author       string                   `json:"author,omitempty"`
	URL          string                   `json:"url,omitempty"`
	TextHTML     string                   `json:"text_html"`
	TextMarkdown string                   `json:"text_markdown"`
	VideoURLs    []string                 `json:"video_urls,omitempty"`
	AudioItems   []newsapi.MediaAudio     `json:"audio_items,omitempty"`
	Related      []newsapi.ArticleSummary `json:"related,omitempty"`
}

var (
	openUTagPattern      = regexp.MustCompile(`(?i)<\s*u\b[^>]*>`)
	closeUTagPattern     = regexp.MustCompile(`(?i)</\s*u\s*>`)
	scriptPattern        = regexp.MustCompile(`(?is)<\s*script\b[^>]*>.*?</\s*script\s*>`)
	stylePattern         = regexp.MustCompile(`(?is)<\s*style\b[^>]*>.*?</\s*style\s*>`)
	noscriptPattern      = regexp.MustCompile(`(?is)<\s*noscript\b[^>]*>.*?</\s*noscript\s*>`)
	trimInlineSpace      = regexp.MustCompile(`(?i)\s+(</\s*(?:strong|b|em|i)\s*>)`)
	missingSpaceAfterTag = regexp.MustCompile(`(?i)</\s*((?:strong|b|em|i|a))\s*>(\S)`)
)

func NormalizeHTML(input string) string {
	normalized := strings.ReplaceAll(input, "&nbsp;", " ")
	normalized = scriptPattern.ReplaceAllString(normalized, "")
	normalized = stylePattern.ReplaceAllString(normalized, "")
	normalized = noscriptPattern.ReplaceAllString(normalized, "")
	normalized = openUTagPattern.ReplaceAllString(normalized, "<em>")
	normalized = closeUTagPattern.ReplaceAllString(normalized, "</em>")
	normalized = trimInlineSpace.ReplaceAllString(normalized, "$1")
	normalized = missingSpaceAfterTag.ReplaceAllString(normalized, "</$1> $2")
	return normalized
}

func HTMLToMarkdown(input string) string {
	markdown, err := htmltomarkdown.ConvertString(NormalizeHTML(input))
	if err != nil {
		return strings.TrimSpace(stripHTML(input))
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return strings.TrimSpace(stripHTML(input))
	}
	return markdown
}

func RenderMarkdown(markdown string, width int) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(resolveGlamourStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return markdown
	}
	out, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return strings.TrimSpace(out)
}

func resolveGlamourStyle() string {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return "notty"
	}

	style := strings.TrimSpace(os.Getenv("GLAMOUR_STYLE"))
	if style == "" || strings.EqualFold(style, "auto") {
		return "dark"
	}

	return style
}

func BuildDetailOutput(detail newsapi.ArticleDetail) DetailOutput {
	videoURLs := make([]string, 0)
	for _, link := range detail.Links.Items {
		trimmed := strings.TrimSpace(link.URL)
		lower := strings.ToLower(trimmed)
		if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".m3u8") {
			videoURLs = append(videoURLs, trimmed)
		}
	}

	audioItems := make([]newsapi.MediaAudio, 0)
	for _, item := range detail.Media.Audio {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		audioItems = append(audioItems, item)
	}

	return DetailOutput{
		ID:           detail.ID,
		Date:         detail.Date,
		Title:        detail.Title,
		Section:      strings.TrimSpace(detail.Section),
		Author:       strings.TrimSpace(detail.Author),
		URL:          strings.TrimSpace(detail.URL),
		TextHTML:     detail.Text,
		TextMarkdown: HTMLToMarkdown(detail.Text),
		VideoURLs:    videoURLs,
		AudioItems:   audioItems,
		Related:      detail.Related.Articles,
	}
}

func FormatListDate(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, input)
	if err != nil {
		return input
	}
	local := parsed.In(time.Local)
	if sameDay(local, time.Now().In(time.Local)) {
		return local.Format("15:04")
	}
	return local.Format("2 Jan 2006 15:04")
}

func sameDay(left, right time.Time) bool {
	y1, m1, d1 := left.Date()
	y2, m2, d2 := right.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func stripHTML(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return input
	}
	var buffer bytes.Buffer
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				if buffer.Len() > 0 {
					buffer.WriteByte(' ')
				}
				buffer.WriteString(text)
			}
		}
		if node.Type == html.ElementNode {
			switch node.Data {
			case "p", "br", "div", "li":
				if buffer.Len() > 0 {
					buffer.WriteString("\n")
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	lines := strings.Split(buffer.String(), "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			clean = append(clean, line)
		}
	}
	return strings.Join(clean, "\n\n")
}

func FormatPlainDetail(detail DetailOutput) string {
	var parts []string
	parts = append(parts, detail.Title)
	if detail.Section != "" {
		parts = append(parts, fmt.Sprintf("[%s]", detail.Section))
	}
	if detail.Date != "" {
		parts = append(parts, detail.Date)
	}
	if detail.URL != "" {
		parts = append(parts, detail.URL)
	}
	if detail.Author != "" {
		parts = append(parts, fmt.Sprintf("By %s", detail.Author))
	}
	if detail.TextMarkdown != "" {
		parts = append(parts, detail.TextMarkdown)
	}
	if len(detail.VideoURLs) > 0 || len(detail.AudioItems) > 0 {
		parts = append(parts, "Media")
		for _, url := range detail.VideoURLs {
			parts = append(parts, "- video: "+url)
		}
		for _, item := range detail.AudioItems {
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
