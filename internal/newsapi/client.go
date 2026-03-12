package newsapi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultBaseURL   = "https://example.invalid/"
	DefaultAPISlug   = "private-backend"
	DefaultUserAgent = "viola-cli/dev"

	BaseURLEnvVar   = "VIOLA_PRIVATE_API_BASE_URL"
	APISlugEnvVar   = "VIOLA_PRIVATE_API_SLUG"
	UserAgentEnvVar = "VIOLA_PRIVATE_API_USER_AGENT"
)

type UUIDFunc func() string

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UUID       UUIDFunc
}

type ArticleSummary struct {
	ID        string `xml:"id" json:"id"`
	Date      string `xml:"date" json:"date"`
	Title     string `xml:"title" json:"title"`
	Section   string `xml:"section" json:"section,omitempty"`
	Badge     string `xml:"badge" json:"badge,omitempty"`
	URL       string `xml:"url" json:"url,omitempty"`
	Thumb2    string `xml:"thumb2" json:"thumb2,omitempty"`
	FlagMedia string `xml:"flag_media" json:"flag_media,omitempty"`
}

type ArticleDetail struct {
	ID      string `xml:"id" json:"id"`
	Date    string `xml:"date" json:"date"`
	Title   string `xml:"title" json:"title"`
	Section string `xml:"section" json:"section,omitempty"`
	Author  string `xml:"author" json:"author,omitempty"`
	Text    string `xml:"text" json:"text_html,omitempty"`
	URL     string `xml:"url" json:"url,omitempty"`
	Badge   string `xml:"badge" json:"badge,omitempty"`
	Links   Links  `xml:"links" json:"-"`
	Media   Media  `xml:"media" json:"-"`
	Related struct {
		Articles []ArticleSummary `xml:"article" json:"related"`
	} `xml:"related" json:"-"`
}

type Links struct {
	Items []Link `xml:"link"`
}

type Link struct {
	URL  string `xml:"url,attr" json:"url"`
	Text string `xml:",chardata" json:"text,omitempty"`
}

type Media struct {
	Photos []MediaPhoto `xml:"photo"`
	Audio  []MediaAudio `xml:"audio"`
}

type MediaPhoto struct {
	Thumb1      string `xml:"thumb1"`
	Thumb2      string `xml:"thumb2"`
	Author      string `xml:"author"`
	Description string `xml:"description"`
}

type MediaAudio struct {
	Thumb       string `xml:"thumb" json:"thumb,omitempty"`
	URL         string `xml:"url" json:"url,omitempty"`
	Source      string `xml:"source" json:"source,omitempty"`
	Description string `xml:"description" json:"description,omitempty"`
}

type listResponse struct {
	Articles struct {
		Items []ArticleSummary `xml:"article"`
	} `xml:"articles"`
}

type detailResponse struct {
	Articles struct {
		Item ArticleDetail `xml:"article"`
	} `xml:"articles"`
}

func New(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		BaseURL:    configuredBaseURL(),
		HTTPClient: httpClient,
		UUID: func() string {
			return strings.ToUpper(uuid.NewString())
		},
	}
}

func (c *Client) FetchArticles(ctx context.Context, start, step int) ([]ArticleSummary, error) {
	endpoint, err := buildURL(c.BaseURL, map[string]string{
		"charset": "utf-8",
		"start":   fmt.Sprintf("%d", start),
		"step":    fmt.Sprintf("%d", step),
	})
	if err != nil {
		return nil, err
	}

	var payload listResponse
	if err := c.doXML(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	return payload.Articles.Items, nil
}

func (c *Client) SearchArticles(ctx context.Context, query string, start, step int) ([]ArticleSummary, error) {
	endpoint, err := buildURL(c.BaseURL, map[string]string{
		"charset": "utf-8",
		"start":   fmt.Sprintf("%d", start),
		"step":    fmt.Sprintf("%d", step),
		"q":       query,
	})
	if err != nil {
		return nil, err
	}

	var payload listResponse
	if err := c.doXML(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	return payload.Articles.Items, nil
}

func (c *Client) FetchArticleDetail(ctx context.Context, id string) (ArticleDetail, error) {
	endpoint, err := buildURL(c.BaseURL, map[string]string{
		"charset": "utf-8",
		"id":      id,
	})
	if err != nil {
		return ArticleDetail{}, err
	}

	var payload detailResponse
	if err := c.doXML(ctx, endpoint, &payload); err != nil {
		return ArticleDetail{}, err
	}
	if strings.TrimSpace(payload.Articles.Item.ID) == "" {
		return ArticleDetail{}, fmt.Errorf("article %q was not found", id)
	}
	return payload.Articles.Item, nil
}

func (c *Client) doXML(ctx context.Context, endpoint string, out any) error {
	request, err := c.newRequest(ctx, endpoint)
	if err != nil {
		return err
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("%s: %s", response.Status, message)
	}

	if err := xml.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, endpoint string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for key, value := range c.requestHeaders(endpoint) {
		request.Header.Set(key, value)
	}
	return request, nil
}

func (c *Client) requestHeaders(endpoint string) map[string]string {
	generated := c.UUID()
	sum := md5.Sum([]byte(endpoint + configuredAPISlug() + generated))
	secret := hex.EncodeToString(sum[:])
	return map[string]string{
		"X-TCC-Version":   "1.0",
		"X-TCC-UUID":      generated,
		"X-TCC-Secret":    secret,
		"User-Agent":      configuredUserAgent(),
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.9",
	}
}

func configuredBaseURL() string {
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv(BaseURLEnvVar)),
		DefaultBaseURL,
	)
}

func configuredAPISlug() string {
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv(APISlugEnvVar)),
		DefaultAPISlug,
	)
}

func configuredUserAgent() string {
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv(UserAgentEnvVar)),
		DefaultUserAgent,
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildURL(base string, query map[string]string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}

	values := parsed.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}
