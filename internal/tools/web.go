package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebSearchTool 网页搜索工具 (使用 Brave Search API)
type WebSearchTool struct {
	apiKey     string
	maxResults int
	httpClient *http.Client
}

// NewWebSearchTool 创建网页搜索工具
func NewWebSearchTool(apiKey string, maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 5
	}
	return &WebSearchTool{
		apiKey:     apiKey,
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web. Returns titles, URLs, and snippets."
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results (1-10)",
				"minimum":     1,
				"maximum":     10,
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	if t.apiKey == "" {
		return "Error: BRAVE_API_KEY not configured. Set it in config or environment variable.", nil
	}

	var p struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Count <= 0 {
		p.Count = t.maxResults
	}
	if p.Count > 10 {
		p.Count = 10
	}

	// Build request URL
	reqURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(p.Query), p.Count)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: Search request failed: %v", err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("Error: Search API returned status %d: %s", resp.StatusCode, string(body)), nil
	}

	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Web.Results) == 0 {
		return fmt.Sprintf("No results for: %s", p.Query), nil
	}

	// Format results
	var lines []string
	lines = append(lines, fmt.Sprintf("Results for: %s\n", p.Query))

	for i, item := range result.Web.Results {
		if i >= p.Count {
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, item.Title))
		lines = append(lines, fmt.Sprintf("   %s", item.URL))
		if item.Description != "" {
			lines = append(lines, fmt.Sprintf("   %s", item.Description))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func (t *WebSearchTool) IsDangerous() bool { return false }

// WebFetchTool 网页抓取工具
type WebFetchTool struct {
	maxChars   int
	httpClient *http.Client
}

// NewWebFetchTool 创建网页抓取工具
func NewWebFetchTool(maxChars int) *WebFetchTool {
	if maxChars <= 0 {
		maxChars = 50000
	}
	return &WebFetchTool{
		maxChars: maxChars,
		httpClient: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
		},
	}
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Description() string {
	return "Fetch URL and extract readable content (HTML to markdown/text)."
}

func (t *WebFetchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to fetch",
			},
			"extractMode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"markdown", "text"},
				"default":     "markdown",
				"description": "Extraction mode",
			},
			"maxChars": map[string]interface{}{
				"type":        "integer",
				"minimum":     100,
				"description": "Maximum characters to return",
			},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		URL         string `json:"url"`
		ExtractMode string `json:"extractMode"`
		MaxChars    int    `json:"maxChars"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.ExtractMode == "" {
		p.ExtractMode = "markdown"
	}
	if p.MaxChars <= 0 {
		p.MaxChars = t.maxChars
	}

	// Validate URL
	if err := t.validateURL(p.URL); err != nil {
		result := map[string]interface{}{
			"error": fmt.Sprintf("URL validation failed: %v", err),
			"url":   p.URL,
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		result := map[string]interface{}{
			"error": err.Error(),
			"url":   p.URL,
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	finalURL := resp.Request.URL.String()

	// Read body
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(p.MaxChars*2)))
	if err != nil {
		result := map[string]interface{}{
			"error": err.Error(),
			"url":   p.URL,
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	var text string
	var extractor string

	// Handle different content types
	if strings.Contains(contentType, "application/json") {
		// JSON content
		var jsonContent interface{}
		if err := json.Unmarshal(body, &jsonContent); err == nil {
			formatted, _ := json.MarshalIndent(jsonContent, "", "  ")
			text = string(formatted)
		} else {
			text = string(body)
		}
		extractor = "json"
	} else if strings.Contains(contentType, "text/html") || t.isHTML(body) {
		// HTML content
		text = t.extractHTML(string(body), p.ExtractMode)
		extractor = "readability"
	} else {
		// Raw content
		text = string(body)
		extractor = "raw"
	}

	// Truncate if needed
	truncated := len(text) > p.MaxChars
	if truncated {
		text = text[:p.MaxChars]
	}

	result := map[string]interface{}{
		"url":       p.URL,
		"finalUrl":  finalURL,
		"status":    resp.StatusCode,
		"extractor": extractor,
		"truncated": truncated,
		"length":    len(text),
		"text":      text,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}

	return string(data), nil
}

func (t *WebFetchTool) IsDangerous() bool { return false }

func (t *WebFetchTool) validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("only http/https allowed, got '%s'", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("missing domain")
	}

	return nil
}

func (t *WebFetchTool) isHTML(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	preview := strings.ToLower(string(body[:min(256, len(body))]))
	return strings.HasPrefix(preview, "<!doctype") || strings.HasPrefix(preview, "<html")
}

func (t *WebFetchTool) extractHTML(html, mode string) string {
	// Simple HTML to text/markdown extraction
	// 1. Remove script and style
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = scriptRe.ReplaceAllString(html, "")
	html = styleRe.ReplaceAllString(html, "")

	if mode == "markdown" {
		// Convert links
		linkRe := regexp.MustCompile(`(?i)<a\s+[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
		html = linkRe.ReplaceAllStringFunc(html, func(m string) string {
			matches := linkRe.FindStringSubmatch(m)
			if len(matches) >= 3 {
				text := t.stripTags(matches[2])
				return fmt.Sprintf("[%s](%s)", text, matches[1])
			}
			return m
		})

		// Convert headings (h1-h6) - process each level separately since Go regexp doesn't support backreferences
		for level := 1; level <= 6; level++ {
			pattern := fmt.Sprintf(`(?i)<h%d[^>]*>(.*?)</h%d>`, level, level)
			headingRe := regexp.MustCompile(pattern)
			prefix := strings.Repeat("#", level)
			html = headingRe.ReplaceAllStringFunc(html, func(m string) string {
				matches := headingRe.FindStringSubmatch(m)
				if len(matches) >= 2 {
					text := t.stripTags(matches[1])
					return fmt.Sprintf("\n%s %s\n", prefix, text)
				}
				return m
			})
		}

		// Convert list items
		liRe := regexp.MustCompile(`(?i)<li[^>]*>(.*?)</li>`)
		html = liRe.ReplaceAllStringFunc(html, func(m string) string {
			matches := liRe.FindStringSubmatch(m)
			if len(matches) >= 2 {
				return fmt.Sprintf("\n- %s", t.stripTags(matches[1]))
			}
			return m
		})

		// Convert paragraph/div breaks
		blockRe := regexp.MustCompile(`(?i)</(p|div|section|article)>`)
		html = blockRe.ReplaceAllString(html, "\n\n")

		// Convert line breaks
		brRe := regexp.MustCompile(`(?i)<(br|hr)\s*/?>`)
		html = brRe.ReplaceAllString(html, "\n")
	}

	// Strip remaining tags
	text := t.stripTags(html)

	// Normalize whitespace
	text = t.normalize(text)

	// Try to get title
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		title := t.stripTags(matches[1])
		if title != "" {
			text = fmt.Sprintf("# %s\n\n%s", title, text)
		}
	}

	return text
}

func (t *WebFetchTool) stripTags(html string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(html, "")
	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	return strings.TrimSpace(text)
}

func (t *WebFetchTool) normalize(text string) string {
	// Normalize multiple spaces to single space
	spaceRe := regexp.MustCompile(`[ \t]+`)
	text = spaceRe.ReplaceAllString(text, " ")

	// Normalize multiple newlines to double
	newlineRe := regexp.MustCompile(`\n{3,}`)
	text = newlineRe.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
