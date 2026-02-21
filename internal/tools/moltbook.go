// Package tools 工具实现 - Moltbook AI 社交网络工具
package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	moltbookAPIBase = "https://www.moltbook.com/api/v1"
	moltbookCredDir = ".lingguard/moltbook"
)

// MoltbookTool Moltbook AI 社交网络工具
type MoltbookTool struct {
	apiKey     string
	agentName  string
	httpClient *http.Client
	credPath   string // ~/.lingguard/moltbook/credentials.json
}

// MoltbookCredentials 本地存储的凭证
type MoltbookCredentials struct {
	APIKey    string `json:"api_key"`
	AgentName string `json:"agent_name"`
	AgentID   string `json:"agent_id,omitempty"`
	CreatedAt string `json:"created_at"`
}

// NewMoltbookTool 创建 Moltbook 工具
func NewMoltbookTool(apiKey, agentName string) *MoltbookTool {
	home, _ := os.UserHomeDir()
	credPath := filepath.Join(home, moltbookCredDir, "credentials.json")

	// 如果没有提供 apiKey，尝试从本地凭证加载
	if apiKey == "" {
		if creds, err := loadMoltbookCredentials(credPath); err == nil {
			apiKey = creds.APIKey
			if agentName == "" {
				agentName = creds.AgentName
			}
		}
	}

	// 设置默认 agent 名称
	if agentName == "" {
		agentName = "LingGuard"
	}

	return &MoltbookTool{
		apiKey:    apiKey,
		agentName: agentName,
		credPath:  credPath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			},
		},
	}
}

// Name 返回工具名称
func (t *MoltbookTool) Name() string {
	return "moltbook"
}

// Description 返回工具描述
func (t *MoltbookTool) Description() string {
	return `Moltbook AI 社交网络工具，让 AI Agent 可以在 Moltbook 平台上发帖、评论、投票和社交。

Actions:
- register: 注册新 Agent，获取 API Key
- status: 检查注册状态和配置
- profile: 获取/更新个人资料
- feed: 获取个性化 Feed
- post: 创建帖子
- comment: 发表评论
- upvote: 给帖子/评论投票（+1）
- downvote: 给帖子/评论投票（-1）
- submolts: 列出/创建社区 (submolt)
- subscribe: 订阅社区
- unsubscribe: 取消订阅社区
- follow: 关注其他 Agent
- unfollow: 取消关注
- search: 语义搜索

Usage:
{"action": "register", "name": "MyAgent", "description": "A helpful AI assistant"}
{"action": "status"}
{"action": "feed", "limit": 10}
{"action": "post", "title": "Hello World", "content": "My first post!", "submolt": "general"}
{"action": "comment", "post_id": "xxx", "content": "Great post!"}
{"action": "upvote", "target_id": "xxx", "target_type": "post"}
{"action": "search", "query": "AI agents", "limit": 10}

Rate Limits:
- 100 requests/minute
- 1 post per 30 minutes
- 1 comment per 20 seconds
- 50 comments per day

Security:
- API Key 存储在本地 ~/.lingguard/moltbook/credentials.json
- 只访问 https://www.moltbook.com 域名`
}

// Parameters 返回参数定义
func (t *MoltbookTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"register", "status", "profile", "feed", "post", "comment", "upvote", "downvote", "submolts", "subscribe", "unsubscribe", "follow", "unfollow", "search"},
				"description": "操作类型",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Agent 名称 (用于 register)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Agent 描述 (用于 register)",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "帖子标题 (用于 post)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "内容 (用于 post, comment)",
			},
			"submolt": map[string]interface{}{
				"type":        "string",
				"description": "社区名称 (用于 post, subscribe)",
			},
			"post_id": map[string]interface{}{
				"type":        "string",
				"description": "帖子 ID (用于 comment)",
			},
			"target_id": map[string]interface{}{
				"type":        "string",
				"description": "目标 ID (用于 upvote, downvote)",
			},
			"target_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"post", "comment"},
				"description": "目标类型 (用于 upvote, downvote)",
			},
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "Agent ID (用于 follow, unfollow)",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "搜索关键词 (用于 search)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "返回数量限制 (默认 10)",
			},
			"create": map[string]interface{}{
				"type":        "boolean",
				"description": "是否创建 (用于 submolts)",
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行工具
func (t *MoltbookTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action      string `json:"action"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Title       string `json:"title"`
		Content     string `json:"content"`
		Submolt     string `json:"submolt"`
		PostID      string `json:"post_id"`
		TargetID    string `json:"target_id"`
		TargetType  string `json:"target_type"`
		AgentID     string `json:"agent_id"`
		Query       string `json:"query"`
		Limit       int    `json:"limit"`
		Create      bool   `json:"create"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	switch params.Action {
	case "register":
		return t.register(ctx, params.Name, params.Description)
	case "status":
		return t.status()
	case "profile":
		return t.profile(ctx)
	case "feed":
		return t.feed(ctx, params.Limit)
	case "post":
		return t.createPost(ctx, params.Title, params.Content, params.Submolt)
	case "comment":
		return t.createComment(ctx, params.PostID, params.Content)
	case "upvote":
		return t.vote(ctx, params.TargetID, params.TargetType, 1)
	case "downvote":
		return t.vote(ctx, params.TargetID, params.TargetType, -1)
	case "submolts":
		return t.submolts(ctx, params.Submolt, params.Create)
	case "subscribe":
		return t.subscribe(ctx, params.Submolt, true)
	case "unsubscribe":
		return t.subscribe(ctx, params.Submolt, false)
	case "follow":
		return t.follow(ctx, params.AgentID, true)
	case "unfollow":
		return t.follow(ctx, params.AgentID, false)
	case "search":
		return t.search(ctx, params.Query, params.Limit)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// register 注册新 Agent
func (t *MoltbookTool) register(ctx context.Context, name, description string) (string, error) {
	if name == "" {
		name = t.agentName
	}
	if description == "" {
		description = "AI Assistant powered by LingGuard"
	}

	reqBody := map[string]string{
		"name":        name,
		"description": description,
	}

	var resp struct {
		APIKey           string `json:"api_key"`
		AgentID          string `json:"agent_id"`
		ClaimURL         string `json:"claim_url"`
		VerificationCode string `json:"verification_code"`
		Message          string `json:"message"`
		Error            string `json:"error"`
	}

	if err := t.doRequest(ctx, "POST", "/agents/register", reqBody, &resp, false); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("register failed: %s", resp.Error)
	}

	// 保存凭证到本地
	if resp.APIKey != "" {
		creds := &MoltbookCredentials{
			APIKey:    resp.APIKey,
			AgentName: name,
			AgentID:   resp.AgentID,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		if err := t.saveCredentials(creds); err != nil {
			return "", fmt.Errorf("save credentials: %w", err)
		}
		t.apiKey = resp.APIKey
		t.agentName = name
	}

	result := fmt.Sprintf("注册成功！\nAgent: %s\nAgent ID: %s\nAPI Key: %s\n",
		name, resp.AgentID, resp.APIKey)
	if resp.ClaimURL != "" {
		result += fmt.Sprintf("\n访问以下链接认领你的 Agent:\n%s\n", resp.ClaimURL)
		if resp.VerificationCode != "" {
			result += fmt.Sprintf("验证码: %s\n", resp.VerificationCode)
		}
	}
	return result, nil
}

// status 检查状态
func (t *MoltbookTool) status() (string, error) {
	if t.apiKey == "" {
		return "未注册。请先使用 register action 注册 Agent。\n示例: {\"action\": \"register\", \"name\": \"LingGuard\"}", nil
	}

	// 检查本地凭证
	creds, err := loadMoltbookCredentials(t.credPath)
	if err != nil {
		return fmt.Sprintf("已配置但无法读取本地凭证。\nAPI Key: %s...\nAgent Name: %s",
			t.apiKey[:min(20, len(t.apiKey))], t.agentName), nil
	}

	return fmt.Sprintf("已注册\nAgent Name: %s\nAgent ID: %s\nAPI Key: %s...\n注册时间: %s",
		creds.AgentName, creds.AgentID, creds.APIKey[:min(20, len(creds.APIKey))], creds.CreatedAt), nil
}

// profile 获取/更新个人资料
func (t *MoltbookTool) profile(ctx context.Context) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	var resp struct {
		AgentID     string `json:"agent_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Karma       int    `json:"karma"`
		CreatedAt   string `json:"created_at"`
		Posts       int    `json:"posts"`
		Comments    int    `json:"comments"`
		Followers   int    `json:"followers"`
		Following   int    `json:"following"`
		Error       string `json:"error"`
	}

	if err := t.doRequest(ctx, "GET", "/agents/me", nil, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("get profile failed: %s", resp.Error)
	}

	return fmt.Sprintf("Agent: %s\nID: %s\n描述: %s\nKarma: %d\n帖子: %d\n评论: %d\n关注者: %d\n正在关注: %d\n注册时间: %s",
		resp.Name, resp.AgentID, resp.Description, resp.Karma, resp.Posts, resp.Comments, resp.Followers, resp.Following, resp.CreatedAt), nil
}

// feed 获取个性化 Feed
func (t *MoltbookTool) feed(ctx context.Context, limit int) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if limit <= 0 {
		limit = 10
	}

	var resp struct {
		Posts []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Content   string `json:"content"`
			Author    string `json:"author_name"`
			Submolt   string `json:"submolt"`
			Upvotes   int    `json:"upvotes"`
			Comments  int    `json:"comments"`
			CreatedAt string `json:"created_at"`
		} `json:"posts"`
		Error string `json:"error"`
	}

	endpoint := fmt.Sprintf("/feed?limit=%d", limit)
	if err := t.doRequest(ctx, "GET", endpoint, nil, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("get feed failed: %s", resp.Error)
	}

	if len(resp.Posts) == 0 {
		return "Feed 为空，尝试关注更多 Agent 或订阅社区", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Feed (共 %d 条) ===\n\n", len(resp.Posts)))
	for i, post := range resp.Posts {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, post.Submolt, post.Title))
		if len(post.Content) > 100 {
			sb.WriteString(fmt.Sprintf("   %s...\n", post.Content[:100]))
		} else {
			sb.WriteString(fmt.Sprintf("   %s\n", post.Content))
		}
		sb.WriteString(fmt.Sprintf("   by %s | 👍 %d | 💬 %d | %s\n\n",
			post.Author, post.Upvotes, post.Comments, post.ID))
	}
	return sb.String(), nil
}

// createPost 创建帖子
func (t *MoltbookTool) createPost(ctx context.Context, title, content, submolt string) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if submolt == "" {
		submolt = "general"
	}

	reqBody := map[string]string{
		"title":   title,
		"content": content,
		"submolt": submolt,
	}

	var resp struct {
		PostID  string `json:"post_id"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := t.doRequest(ctx, "POST", "/posts", reqBody, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("create post failed: %s", resp.Error)
	}

	return fmt.Sprintf("发帖成功！\nPost ID: %s\n标题: %s\n社区: %s\n\n注意: 每个帖子间隔至少 30 分钟",
		resp.PostID, title, submolt), nil
}

// createComment 发表评论
func (t *MoltbookTool) createComment(ctx context.Context, postID, content string) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if postID == "" {
		return "", fmt.Errorf("post_id is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	reqBody := map[string]string{
		"post_id": postID,
		"content": content,
	}

	var resp struct {
		CommentID string `json:"comment_id"`
		Message   string `json:"message"`
		Error     string `json:"error"`
	}

	if err := t.doRequest(ctx, "POST", "/comments", reqBody, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("create comment failed: %s", resp.Error)
	}

	return fmt.Sprintf("评论成功！\nComment ID: %s\n内容: %s", resp.CommentID, content), nil
}

// vote 投票
func (t *MoltbookTool) vote(ctx context.Context, targetID, targetType string, value int) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if targetID == "" {
		return "", fmt.Errorf("target_id is required")
	}
	if targetType == "" {
		targetType = "post"
	}

	reqBody := map[string]interface{}{
		"target_id":   targetID,
		"target_type": targetType,
		"value":       value,
	}

	var resp struct {
		Message  string `json:"message"`
		NewScore int    `json:"new_score"`
		Error    string `json:"error"`
	}

	if err := t.doRequest(ctx, "POST", "/vote", reqBody, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("vote failed: %s", resp.Error)
	}

	voteType := "upvote"
	if value < 0 {
		voteType = "downvote"
	}
	return fmt.Sprintf("%s 成功！\nTarget: %s\n新分数: %d", voteType, targetID, resp.NewScore), nil
}

// submolts 列出/创建社区
func (t *MoltbookTool) submolts(ctx context.Context, name string, create bool) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if create && name != "" {
		// 创建社区
		reqBody := map[string]string{
			"name": name,
		}
		var resp struct {
			SubmoltID string `json:"submolt_id"`
			Message   string `json:"message"`
			Error     string `json:"error"`
		}
		if err := t.doRequest(ctx, "POST", "/submolts", reqBody, &resp, true); err != nil {
			return "", err
		}
		if resp.Error != "" {
			return "", fmt.Errorf("create submolt failed: %s", resp.Error)
		}
		return fmt.Sprintf("社区创建成功！\nSubmolt ID: %s\n名称: %s", resp.SubmoltID, name), nil
	}

	// 列出社区
	var resp struct {
		Submolts []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Members     int    `json:"members"`
			Posts       int    `json:"posts"`
		} `json:"submolts"`
		Error string `json:"error"`
	}

	endpoint := "/submolts"
	if name != "" {
		endpoint = fmt.Sprintf("/submolts?name=%s", name)
	}

	if err := t.doRequest(ctx, "GET", endpoint, nil, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("get submolts failed: %s", resp.Error)
	}

	if len(resp.Submolts) == 0 {
		return "没有找到社区", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 社区 (共 %d 个) ===\n\n", len(resp.Submolts)))
	for i, sm := range resp.Submolts {
		sb.WriteString(fmt.Sprintf("%d. r/%s\n", i+1, sm.Name))
		if sm.Description != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", sm.Description))
		}
		sb.WriteString(fmt.Sprintf("   成员: %d | 帖子: %d\n\n", sm.Members, sm.Posts))
	}
	return sb.String(), nil
}

// subscribe 订阅/取消订阅社区
func (t *MoltbookTool) subscribe(ctx context.Context, submolt string, subscribe bool) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if submolt == "" {
		return "", fmt.Errorf("submolt is required")
	}

	action := "subscribe"
	if !subscribe {
		action = "unsubscribe"
	}

	reqBody := map[string]string{
		"submolt": submolt,
	}

	var resp struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := t.doRequest(ctx, "POST", "/submolts/"+action, reqBody, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("%s failed: %s", action, resp.Error)
	}

	actionCN := "订阅"
	if !subscribe {
		actionCN = "取消订阅"
	}
	return fmt.Sprintf("%s成功！\n社区: %s", actionCN, submolt), nil
}

// follow 关注/取消关注
func (t *MoltbookTool) follow(ctx context.Context, agentID string, follow bool) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if agentID == "" {
		return "", fmt.Errorf("agent_id is required")
	}

	action := "follow"
	if !follow {
		action = "unfollow"
	}

	reqBody := map[string]string{
		"agent_id": agentID,
	}

	var resp struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := t.doRequest(ctx, "POST", "/agents/"+action, reqBody, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("%s failed: %s", action, resp.Error)
	}

	actionCN := "关注"
	if !follow {
		actionCN = "取消关注"
	}
	return fmt.Sprintf("%s成功！\nAgent ID: %s", actionCN, agentID), nil
}

// search 语义搜索
func (t *MoltbookTool) search(ctx context.Context, query string, limit int) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("not registered, please run register first")
	}

	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	if limit <= 0 {
		limit = 10
	}

	var resp struct {
		Results []struct {
			ID        string  `json:"id"`
			Type      string  `json:"type"`
			Title     string  `json:"title"`
			Content   string  `json:"content"`
			Score     float64 `json:"score"`
			Author    string  `json:"author_name"`
			CreatedAt string  `json:"created_at"`
		} `json:"results"`
		Error string `json:"error"`
	}

	endpoint := fmt.Sprintf("/search?q=%s&limit=%d", query, limit)
	if err := t.doRequest(ctx, "GET", endpoint, nil, &resp, true); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", fmt.Errorf("search failed: %s", resp.Error)
	}

	if len(resp.Results) == 0 {
		return fmt.Sprintf("搜索 '%s' 没有找到结果", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 搜索结果: %s (共 %d 条) ===\n\n", query, len(resp.Results)))
	for i, r := range resp.Results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, r.Type, r.Title))
		if len(r.Content) > 80 {
			sb.WriteString(fmt.Sprintf("   %s...\n", r.Content[:80]))
		} else {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Content))
		}
		sb.WriteString(fmt.Sprintf("   by %s | 相似度: %.2f | %s\n\n",
			r.Author, r.Score, r.ID))
	}
	return sb.String(), nil
}

// doRequest 执行 HTTP 请求
func (t *MoltbookTool) doRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}, auth bool) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := moltbookAPIBase + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if auth && t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	return nil
}

// saveCredentials 保存凭证到本地
func (t *MoltbookTool) saveCredentials(creds *MoltbookCredentials) error {
	dir := filepath.Dir(t.credPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(t.credPath, data, 0600)
}

// loadMoltbookCredentials 从本地加载凭证
func loadMoltbookCredentials(path string) (*MoltbookCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var creds MoltbookCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// IsDangerous 返回是否为危险操作
func (t *MoltbookTool) IsDangerous() bool {
	return false
}

// SetAPIKey 设置 API Key
func (t *MoltbookTool) SetAPIKey(key string) {
	t.apiKey = key
}
