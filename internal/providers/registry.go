package providers

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
)

// Registry 提供商注册表（参考 nanobot Provider Registry）
// 作为 Provider 管理的单一真实来源
type Registry struct {
	mu          sync.RWMutex
	providers   map[string]Provider
	specs       map[string]*ProviderSpec // 缓存每个 provider 的规范
	defaultName string
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		specs:     make(map[string]*ProviderSpec),
	}
}

// Register 注册提供商
func (r *Registry) Register(name string, p Provider, spec *ProviderSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
	if spec != nil {
		r.specs[name] = spec
	}
}

// Get 获取提供商
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// GetSpec 获取提供商规范
func (r *Registry) GetSpec(name string) *ProviderSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.specs[name]
}

// List 列出所有提供商
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListWithSpecs 列出所有提供商及其规范
func (r *Registry) ListWithSpecs() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]ProviderInfo, 0, len(r.providers))
	for name, p := range r.providers {
		info := ProviderInfo{
			Name:  name,
			Model: p.Model(),
		}
		if spec, ok := r.specs[name]; ok {
			info.DisplayName = spec.DisplayName
			info.IsGateway = spec.IsGateway
		}
		infos = append(infos, info)
	}
	return infos
}

// ProviderInfo 提供商信息
type ProviderInfo struct {
	Name        string
	DisplayName string
	Model       string
	IsGateway   bool
}

// SetDefault 设置默认 Provider
func (r *Registry) SetDefault(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultName = name
}

// GetDefault 获取默认 Provider
func (r *Registry) GetDefault() (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.defaultName == "" {
		return nil, false
	}
	p, ok := r.providers[r.defaultName]
	return p, ok
}

// InitFromConfig 从配置初始化提供商（参考 nanobot）
// 自动检测 provider 类型并创建对应的实例
func (r *Registry) InitFromConfig(cfg *config.Config) error {
	for name, pc := range cfg.Providers {
		if pc.APIKey == "" {
			logger.Debug("Skipping provider: no API key", "name", name)
			continue
		}

		// 查找 provider 规范
		spec := FindSpecByName(name)
		if spec == nil {
			// 如果没有找到预定义规范，创建一个通用的
			spec = &ProviderSpec{
				Name:        name,
				DisplayName: name,
			}
		}

		// 构建 ProviderConfig
		providerCfg := &ProviderConfig{
			APIKey:        pc.APIKey,
			APIBase:       pc.APIBase,
			Model:         pc.Model,
			Temperature:   pc.Temperature,
			MaxTokens:     pc.MaxTokens,
			Timeout:       pc.Timeout,
			SupportsTools: pc.SupportsTools,
		}

		// 使用规范中的默认值（仅当 config 中为空时）
		if providerCfg.APIBase == "" && spec.DefaultAPIBase != "" {
			providerCfg.APIBase = spec.DefaultAPIBase
		}
		if providerCfg.Model == "" && spec.DefaultModel != "" {
			providerCfg.Model = spec.DefaultModel
		}

		// 决定 Provider 类型：config.json 配置优先
		// 1. 如果 config.json 配置了 apiBase，根据 apiBase 判断
		// 2. 否则，根据 spec.IsAnthropic 判断
		var p Provider
		if pc.APIBase != "" {
			// config.json 配置了 apiBase，以 apiBase 判断为准
			if IsAnthropicEndpoint(pc.APIBase) {
				p = NewAnthropicProvider(name, providerCfg)
			} else {
				p = NewOpenAIProvider(name, providerCfg)
			}
		} else {
			// 使用 spec.go 的默认配置
			if spec.IsAnthropic {
				p = NewAnthropicProvider(name, providerCfg)
			} else {
				p = NewOpenAIProvider(name, providerCfg)
			}
		}

		r.Register(name, p, spec)
		logger.Info("Registered provider", "name", name, "display", spec.DisplayName)
	}

	if len(r.providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	return nil
}

// MatchProvider 根据模型名自动匹配 Provider（参考 nanobot）
// 匹配优先级：
// 1. "provider/model" 格式 -> 直接匹配 provider
// 2. model 是已注册的 provider 名称 -> 返回该 provider
// 3. 通过关键词匹配（gpt -> openai, claude -> anthropic）
// 4. 返回默认 Provider
func (r *Registry) MatchProvider(model string) (Provider, *ProviderSpec) {
	// 1. 尝试解析 "provider/model" 格式
	if parts := strings.SplitN(model, "/", 2); len(parts) == 2 {
		providerName := parts[0]
		if p, ok := r.Get(providerName); ok {
			return p, r.GetSpec(providerName)
		}
		// 可能是模型格式如 "anthropic/claude-3-opus"，继续用关键词匹配
	}

	// 2. 检查 model 是否是已注册的 provider 名称
	if p, ok := r.Get(model); ok {
		return p, r.GetSpec(model)
	}

	// 3. 通过关键词匹配
	if spec := FindSpecByModel(model); spec != nil {
		if p, ok := r.Get(spec.Name); ok {
			return p, spec
		}
	}

	// 4. 返回默认 Provider
	if r.defaultName != "" {
		if p, ok := r.Get(r.defaultName); ok {
			return p, r.GetSpec(r.defaultName)
		}
	}

	// 5. 返回第一个可用的 Provider
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, p := range r.providers {
		return p, r.specs[name]
	}

	return nil, nil
}

// DetectProvider 从配置中自动检测 Provider 类型
// 通过 API Key 前缀或 API Base URL 检测
func (r *Registry) DetectProvider(apiKey, apiBase string) *ProviderSpec {
	// 优先通过 API Key 前缀检测
	if apiKey != "" {
		if spec := FindSpecByAPIKey(apiKey); spec != nil {
			return spec
		}
	}

	// 通过 API Base URL 检测
	if apiBase != "" {
		if spec := FindSpecByAPIBase(apiBase); spec != nil {
			return spec
		}
	}

	return nil
}

// NormalizeModel 规范化模型名称
// 根据匹配到的 Provider 规范处理模型前缀
func (r *Registry) NormalizeModel(model string, spec *ProviderSpec) string {
	if spec == nil {
		return model
	}
	return spec.NormalizeModel(model)
}

// ResolveModel 解析模型名称，返回实际使用的模型
// 如果 model 是 "provider/model" 格式，返回 model 部分
// 否则返回原 model
func (r *Registry) ResolveModel(model string) string {
	if parts := strings.SplitN(model, "/", 2); len(parts) == 2 {
		// 检查第一部分是否是已知的 provider
		if FindSpecByName(parts[0]) != nil {
			return parts[1]
		}
	}
	return model
}
