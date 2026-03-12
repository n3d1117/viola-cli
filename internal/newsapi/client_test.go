package newsapi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestHeadersIncludeSignedSecret(t *testing.T) {
	t.Setenv(APISlugEnvVar, "private-slug")
	t.Setenv(UserAgentEnvVar, "viola-cli/test")

	client := New(nil)
	client.UUID = func() string { return "ABC-123" }

	endpoint := "https://example.invalid/?charset=utf-8&start=0&step=3"
	headers := client.requestHeaders(endpoint)
	if headers["X-TCC-UUID"] != "ABC-123" {
		t.Fatalf("unexpected uuid: %q", headers["X-TCC-UUID"])
	}
	if headers["X-TCC-Secret"] != md5Hex(endpoint+"private-slug"+"ABC-123") {
		t.Fatalf("unexpected secret: %q", headers["X-TCC-Secret"])
	}
	if headers["User-Agent"] != "viola-cli/test" {
		t.Fatalf("unexpected user agent: %q", headers["User-Agent"])
	}
}

func TestNewUsesConfiguredBaseURL(t *testing.T) {
	t.Setenv(BaseURLEnvVar, "https://private.example.invalid/")

	client := New(nil)
	if client.BaseURL != "https://private.example.invalid/" {
		t.Fatalf("unexpected base url: %q", client.BaseURL)
	}
}

func TestFetchArticlesDecodesXML(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-TCC-Version") != "1.0" {
			t.Fatalf("missing version header")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<tcc><articles><article><id>a-1</id><date>2026-03-11T19:45:03+01:00</date><title><![CDATA[Title]]></title><section><![CDATA[Primo Piano]]></section><badge><![CDATA[Badge]]></badge><url><![CDATA[https://example.com/a-1]]></url><thumb2><![CDATA[https://example.com/thumb.jpg]]></thumb2><flag_media><![CDATA[5]]></flag_media></article></articles></tcc>`))
	}))
	defer server.Close()

	client := New(server.Client())
	client.BaseURL = server.URL
	client.UUID = func() string { return "ABC-123" }

	items, err := client.FetchArticles(context.Background(), 0, 3)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != "a-1" || items[0].Title != "Title" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestFetchArticleDetailNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><tcc><articles><article></article></articles></tcc>`))
	}))
	defer server.Close()

	client := New(server.Client())
	client.BaseURL = server.URL

	_, err := client.FetchArticleDetail(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func md5Hex(input string) string {
	sum := md5.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}
