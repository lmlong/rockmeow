package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lingguard/pkg/httpclient"
)

// searchParams 搜索参数
type searchParams struct {
	Query         string `json:"query"`
	Count         int    `json:"count"`
	SearchDepth   string `json:"searchDepth"`
	IncludeAnswer bool   `json:"includeAnswer"`
}

// UnmarshalJSON 自定义解析，支持 LLM 生成的字符串类型参数
func (p *searchParams) UnmarshalJSON(data []byte) error {
	// 使用临时结构体，字段使用 interface{} 来接受多种类型
	type tempParams struct {
		Query         interface{} `json:"query"`
		Count         interface{} `json:"count"`
		SearchDepth   interface{} `json:"searchDepth"`
		IncludeAnswer interface{} `json:"includeAnswer"`
	}

	var temp tempParams
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// 解析 query (string)
	switch v := temp.Query.(type) {
	case string:
		p.Query = v
	}

	// 解析 count (int 或 string)
	switch v := temp.Count.(type) {
	case float64:
		p.Count = int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			p.Count = n
		}
	}

	// 解析 searchDepth (string)
	switch v := temp.SearchDepth.(type) {
	case string:
		p.SearchDepth = v
	}

	// 解析 includeAnswer (bool 或 string)
	switch v := temp.IncludeAnswer.(type) {
	case bool:
		p.IncludeAnswer = v
	case string:
		p.IncludeAnswer = strings.ToLower(v) == "true"
	}

	return nil
}

// WebSearchTool 网页搜索工具 (支持 Tavily 和博查 AI 搜索)
type WebSearchTool struct {
	tavilyAPIKey string
	bochaAPIKey  string
	maxResults   int
	httpClient   *http.Client
}

// NewWebSearchTool 创建网页搜索工具
func NewWebSearchTool(tavilyAPIKey, bochaAPIKey string, maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 5
	}
	return &WebSearchTool{
		tavilyAPIKey: tavilyAPIKey,
		bochaAPIKey:  bochaAPIKey,
		maxResults:   maxResults,
		httpClient:   httpclient.Default(),
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "搜索"
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
			"searchDepth": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"basic", "advanced"},
				"default":     "basic",
				"description": "Search depth: 'basic' for fast results, 'advanced' for more comprehensive results",
			},
			"includeAnswer": map[string]interface{}{
				"type":        "boolean",
				"default":     true,
				"description": "Include AI-generated answer in results",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p searchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Count <= 0 {
		p.Count = t.maxResults
	}
	if p.Count > 10 {
		p.Count = 10
	}
	if p.SearchDepth == "" {
		p.SearchDepth = "basic"
	}

	// 优先使用 Tavily，失败则回退到博查
	if t.tavilyAPIKey != "" {
		result, err := t.searchWithTavily(ctx, &p)
		if err == nil {
			return result, nil
		}
		// Tavily 失败，尝试博查
		if t.bochaAPIKey != "" {
			return t.searchWithBocha(ctx, &p)
		}
		return fmt.Sprintf("Error: Tavily search failed: %v", err), nil
	}

	// 没有 Tavily，使用博查
	if t.bochaAPIKey != "" {
		return t.searchWithBocha(ctx, &p)
	}

	return "Error: No search API key configured. Set tavilyApiKey or bochaApiKey in config.", nil
}

// searchWithTavily 使用 Tavily API 搜索
func (t *WebSearchTool) searchWithTavily(ctx context.Context, p *searchParams) (string, error) {
	reqBody := map[string]interface{}{
		"api_key":        t.tavilyAPIKey,
		"query":          p.Query,
		"max_results":    p.Count,
		"search_depth":   p.SearchDepth,
		"include_answer": p.IncludeAnswer,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Results) == 0 {
		return fmt.Sprintf("No results for: %s", p.Query), nil
	}

	// 格式化结果
	var lines []string
	lines = append(lines, fmt.Sprintf("Search results for: %s (via Tavily)\n", p.Query))

	if result.Answer != "" {
		lines = append(lines, fmt.Sprintf("📋 Answer: %s\n", result.Answer))
	}

	for i, item := range result.Results {
		if i >= p.Count {
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, item.Title))
		lines = append(lines, fmt.Sprintf("   🔗 %s", item.URL))
		if item.Content != "" {
			lines = append(lines, fmt.Sprintf("   📄 %s", item.Content))
		}
	}

	return strings.Join(lines, "\n"), nil
}

// searchWithBocha 使用博查 AI 搜索 API
func (t *WebSearchTool) searchWithBocha(ctx context.Context, p *searchParams) (string, error) {
	reqBody := map[string]interface{}{
		"query":      p.Query,
		"count":      p.Count,
		"summary":    p.IncludeAnswer,
		"freshness":  "noLimit",
		"searchMode": "standard",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.bocha.cn/v1/web-search", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.bochaAPIKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
		Data    struct {
			WebPages struct {
				Value []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					URL        string `json:"url"`
					Snippet    string `json:"snippet"`
					Summary    string `json:"summary"`
					SiteName   string `json:"siteName"`
					DisplayURL string `json:"displayUrl"`
				} `json:"value"`
			} `json:"webPages"`
			Summary string `json:"summary"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// 博查 API 成功返回 code: 200
	if result.Code != 200 {
		msg := result.Message
		if msg == "" {
			msg = fmt.Sprintf("API returned code %d", result.Code)
		}
		return "", fmt.Errorf("API error: %s", msg)
	}

	if len(result.Data.WebPages.Value) == 0 {
		return fmt.Sprintf("No results for: %s", p.Query), nil
	}

	// 格式化结果
	var lines []string
	lines = append(lines, fmt.Sprintf("Search results for: %s (via 博查AI)\n", p.Query))

	if result.Data.Summary != "" {
		lines = append(lines, fmt.Sprintf("📋 摘要: %s\n", result.Data.Summary))
	}

	for i, item := range result.Data.WebPages.Value {
		if i >= p.Count {
			break
		}
		title := item.Name
		if title == "" {
			title = item.SiteName
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, title))
		lines = append(lines, fmt.Sprintf("   🔗 %s", item.URL))
		content := item.Summary
		if content == "" {
			content = item.Snippet
		}
		if content != "" {
			lines = append(lines, fmt.Sprintf("   📄 %s", content))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func (t *WebSearchTool) IsDangerous() bool { return false }

func (t *WebSearchTool) ShouldLoadByDefault() bool { return false }

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
		httpClient: httpclient.CustomClient(30*time.Second, func(c *http.Client) {
			c.CheckRedirect = func(req *http.Request, via []*http.Request) error { return nil }
		}),
	}
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Description() string {
	return "抓取"
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
	// Validate URL
	if err := t.validateURL(p.URL); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
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
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	finalURL := resp.Request.URL.String()

	// Read body
	// Read body
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(p.MaxChars*2)))
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
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

func (t *WebFetchTool) ShouldLoadByDefault() bool { return false }

// 私有 IP 地址段黑名单（用于 SSRF 防护）
var privateIPBlocks []netip.Prefix

func init() {
	// 初始化私有 IP 地址段
	privateRanges := []string{
		// IPv4 私有地址
		"10.0.0.0/8",     // Class A 私有网络
		"172.16.0.0/12",  // Class B 私有网络
		"192.168.0.0/16", // Class C 私有网络
		"127.0.0.0/8",    // 本地回环
		"169.254.0.0/16", // 链路本地（包含云元数据 169.254.169.254）
		"0.0.0.0/8",      // 当前网络
		"224.0.0.0/4",    // 多播地址
		"240.0.0.0/4",    // 保留地址
		// IPv6 私有地址
		"::1/128",   // 本地回环
		"fc00::/7",  // 唯一本地地址
		"fe80::/10", // 链路本地
		"ff00::/8",  // 多播
	}

	for _, r := range privateRanges {
		if prefix, err := netip.ParsePrefix(r); err == nil {
			privateIPBlocks = append(privateIPBlocks, prefix)
		}
	}
}

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

	// SSRF 防护：检查是否为私有 IP 地址
	host := parsed.Host
	// 移除端口号
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// 解析 IP 地址
	ip, err := netip.ParseAddr(host)
	if err == nil {
		// 是 IP 地址，检查是否在黑名单中
		if t.isPrivateIP(ip) {
			return fmt.Errorf("access to private IP addresses is blocked: %s", host)
		}
	} else {
		// 不是 IP 地址，可能是域名
		// 检查危险域名模式
		lowerHost := strings.ToLower(host)
		dangerousHosts := []string{
			"localhost",
			"local",
			"localhost.localdomain",
			"ip6-localhost",
			"ip6-loopback",
			"metadata.google.internal", // GCP 元数据
			"metadata",                 // Azure 元数据
		}
		for _, dangerous := range dangerousHosts {
			if lowerHost == dangerous || strings.HasSuffix(lowerHost, "."+dangerous) {
				return fmt.Errorf("access to internal hostnames is blocked: %s", host)
			}
		}
	}

	return nil
}

// isPrivateIP 检查 IP 是否在私有地址段
func (t *WebFetchTool) isPrivateIP(ip netip.Addr) bool {
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
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
