package providers

import (
	"strings"
)

// ProviderSpec 定义 Provider 的完整规范（参考 nanobot Provider Registry）
// 这是 Provider 匹配和自动配置的单一真实来源
type ProviderSpec struct {
	// Name 配置中的 provider 名称（如 "openai", "deepseek"）
	Name string

	// Keywords 模型名关键词，用于自动匹配 provider
	// 例如：gpt -> openai, claude -> anthropic
	Keywords []string

	// DisplayName 显示名称，用于 status 命令输出
	DisplayName string

	// APIKeyPrefix API Key 前缀，用于通过 key 检测 provider
	// 例如："sk-or-" for OpenRouter, "gsk_" for Groq
	APIKeyPrefix string

	// APIBaseKeyword API Base URL 关键词，用于通过 apiBase 检测 provider
	// 例如："openrouter" for OpenRouter
	APIBaseKeyword string

	// DefaultAPIBase 默认 API Base URL
	DefaultAPIBase string

	// DefaultModel 默认模型
	DefaultModel string

	// IsAnthropic 是否使用 Anthropic API 格式（而非 OpenAI 格式）
	IsAnthropic bool

	// IsGateway 是否是网关类型（如 OpenRouter），可以路由到任意模型
	IsGateway bool

	// LiteLLMPrefix 自动为模型名添加前缀（model -> prefix/model）
	// 例如："dashscope" -> "dashscope/qwen-max"
	LiteLLMPrefix string

	// SkipPrefixes 如果模型名已包含这些前缀，不再重复添加
	// 例如：("dashscope/", "openrouter/")
	SkipPrefixes []string

	// StripModelPrefix 是否在添加前缀前先去除已有前缀
	StripModelPrefix bool
}

// PROVIDERS 内置 Provider 规范注册表（参考 nanobot providers/registry.py）
// 添加新 Provider 只需在此添加一个条目
var PROVIDERS = []ProviderSpec{
	{
		Name:           "openai",
		Keywords:       []string{"gpt", "o1", "o3", "chatgpt"},
		DisplayName:    "OpenAI",
		DefaultAPIBase: "https://api.openai.com/v1",
		DefaultModel:   "gpt-4o",
		IsAnthropic:    false,
		APIKeyPrefix:   "sk-",
	},
	{
		Name:           "anthropic",
		Keywords:       []string{"claude"},
		DisplayName:    "Anthropic",
		DefaultAPIBase: "https://api.anthropic.com",
		DefaultModel:   "claude-3-5-sonnet-20241022",
		IsAnthropic:    true,
		APIKeyPrefix:   "sk-ant-",
	},
	{
		Name:           "deepseek",
		Keywords:       []string{"deepseek"},
		DisplayName:    "DeepSeek",
		DefaultAPIBase: "https://api.deepseek.com/v1",
		DefaultModel:   "deepseek-chat",
		IsAnthropic:    false,
		APIKeyPrefix:   "sk-",
	},
	{
		Name:           "openrouter",
		Keywords:       []string{"openrouter"},
		DisplayName:    "OpenRouter",
		DefaultAPIBase: "https://openrouter.ai/api/v1",
		DefaultModel:   "anthropic/claude-3.5-sonnet",
		IsAnthropic:    false,
		IsGateway:      true,
		APIKeyPrefix:   "sk-or-",
		APIBaseKeyword: "openrouter",
	},
	{
		Name:           "qwen",
		Keywords:       []string{"qwen", "tongyi", "dashscope"},
		DisplayName:    "Qwen (通义千问)",
		DefaultAPIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		DefaultModel:   "qwen-max",
		IsAnthropic:    false,
		LiteLLMPrefix:  "dashscope",
		SkipPrefixes:   []string{"dashscope/", "qwen/"},
	},
	{
		Name:           "qwencoder",
		Keywords:       []string{"qwen3-coder", "qwen-coder", "coding.dashscope"},
		DisplayName:    "Qwen Coder (阿里云编程助手)",
		DefaultAPIBase: "https://coding.dashscope.aliyuncs.com/v1",
		DefaultModel:   "qwen3-coder-plus",
		IsAnthropic:    false,
		APIKeyPrefix:   "sk-sp-",
		APIBaseKeyword: "coding.dashscope",
	},
	{
		Name:           "glm",
		Keywords:       []string{"glm", "chatglm", "codegeex", "zhipu"},
		DisplayName:    "Zhipu GLM (智谱)",
		DefaultAPIBase: "https://open.bigmodel.cn/api/paas/v4",
		DefaultModel:   "glm-4",
		IsAnthropic:    false,
	},
	{
		Name:           "minimax",
		Keywords:       []string{"minimax"},
		DisplayName:    "MiniMax",
		DefaultAPIBase: "https://api.minimax.chat/v1",
		DefaultModel:   "abab6.5s-chat",
		IsAnthropic:    false,
	},
	{
		Name:           "moonshot",
		Keywords:       []string{"moonshot", "kimi"},
		DisplayName:    "Moonshot (Kimi)",
		DefaultAPIBase: "https://api.moonshot.cn/v1",
		DefaultModel:   "moonshot-v1-8k",
		IsAnthropic:    false,
	},
	{
		Name:           "gemini",
		Keywords:       []string{"gemini"},
		DisplayName:    "Google Gemini",
		DefaultAPIBase: "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:   "gemini-1.5-pro",
		IsAnthropic:    false,
	},
	{
		Name:           "groq",
		Keywords:       []string{"groq", "llama", "mixtral", "gemma"},
		DisplayName:    "Groq",
		DefaultAPIBase: "https://api.groq.com/openai/v1",
		DefaultModel:   "llama-3.1-70b-versatile",
		IsAnthropic:    false,
		APIKeyPrefix:   "gsk_",
	},
	{
		Name:           "vllm",
		Keywords:       []string{"vllm"},
		DisplayName:    "vLLM (Local)",
		DefaultAPIBase: "http://localhost:8000/v1",
		DefaultModel:   "",
		IsAnthropic:    false,
		IsGateway:      true,
	},
	{
		Name:             "aihubmix",
		Keywords:         []string{"aihubmix"},
		DisplayName:      "AiHubMix",
		DefaultAPIBase:   "https://aihubmix.com/v1",
		DefaultModel:     "",
		IsAnthropic:      false,
		IsGateway:        true,
		StripModelPrefix: true,
	},
}

// FindSpecByName 根据 provider 名称查找规范
func FindSpecByName(name string) *ProviderSpec {
	for i := range PROVIDERS {
		if PROVIDERS[i].Name == name {
			return &PROVIDERS[i]
		}
	}
	return nil
}

// FindSpecByModel 根据模型名查找 Provider 规范
// 匹配逻辑：检查模型名是否包含 provider 的关键词
func FindSpecByModel(model string) *ProviderSpec {
	modelLower := strings.ToLower(model)
	for i := range PROVIDERS {
		spec := &PROVIDERS[i]
		for _, kw := range spec.Keywords {
			// 关键词匹配（不区分大小写）
			if strings.Contains(modelLower, strings.ToLower(kw)) {
				return spec
			}
		}
	}
	return nil
}

// FindSpecByAPIKey 根据 API Key 前缀查找 Provider 规范
// 优先匹配更长的前缀（更具体的前缀）
func FindSpecByAPIKey(apiKey string) *ProviderSpec {
	var bestMatch *ProviderSpec
	var bestLen int

	for i := range PROVIDERS {
		spec := &PROVIDERS[i]
		if spec.APIKeyPrefix != "" && strings.HasPrefix(apiKey, spec.APIKeyPrefix) {
			// 选择最长前缀匹配
			if len(spec.APIKeyPrefix) > bestLen {
				bestLen = len(spec.APIKeyPrefix)
				bestMatch = spec
			}
		}
	}
	return bestMatch
}

// FindSpecByAPIBase 根据 API Base URL 查找 Provider 规范
func FindSpecByAPIBase(apiBase string) *ProviderSpec {
	if apiBase == "" {
		return nil
	}
	apiBaseLower := strings.ToLower(apiBase)
	for i := range PROVIDERS {
		spec := &PROVIDERS[i]
		if spec.APIBaseKeyword != "" && strings.Contains(apiBaseLower, spec.APIBaseKeyword) {
			return spec
		}
	}
	return nil
}

// NormalizeModel 规范化模型名称（处理前缀）
// 如果 spec 配置了 LiteLLMPrefix，会自动添加前缀
func (s *ProviderSpec) NormalizeModel(model string) string {
	if s.LiteLLMPrefix == "" {
		return model
	}

	// 检查是否需要去除已有前缀
	if s.StripModelPrefix {
		if idx := strings.Index(model, "/"); idx >= 0 {
			model = model[idx+1:]
		}
	}

	// 检查是否已有前缀
	for _, prefix := range s.SkipPrefixes {
		if strings.HasPrefix(model, prefix) {
			return model
		}
	}

	// 添加前缀
	return s.LiteLLMPrefix + "/" + model
}

// IsAnthropicEndpoint 检查是否为 Anthropic 兼容端点
func IsAnthropicEndpoint(apiBase string) bool {
	if apiBase == "" {
		return false
	}
	lower := strings.ToLower(apiBase)
	return strings.Contains(lower, "/anthropic")
}
