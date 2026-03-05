package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Registry 工具注册表
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Unregister 注销工具
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get 获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有工具
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// GetToolDefinitions 返回默认加载的工具定义
func (r *Registry) GetToolDefinitions() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]map[string]interface{}, 0)
	for _, t := range r.tools {
		if t.ShouldLoadByDefault() {
			defs = append(defs, Definition(t))
		}
	}
	return defs
}

// GetToolDefinitionsByNames 按名称获取工具定义（供 skill 工具使用）
func (r *Registry) GetToolDefinitionsByNames(names []string) []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			defs = append(defs, Definition(t))
		}
	}
	return defs
}

// Execute 执行工具
func (r *Registry) Execute(name string, params json.RawMessage) (string, error) {
	return r.ExecuteWithContext(context.Background(), name, params)
}

// ExecuteWithContext 执行工具（带 context）
func (r *Registry) ExecuteWithContext(ctx context.Context, name string, params json.RawMessage) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	return t.Execute(ctx, params)
}
