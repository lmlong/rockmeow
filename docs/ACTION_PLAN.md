# LingGuard 下一步行动计划

## 📋 已完成工作

### ✅ 第一阶段优化（已完成）
- [x] 启用 Race 检测 - Makefile 更新
- [x] 数据库连接池优化 - 3 个文件
- [x] 技术债务标记 - 4 个关键文件
- [x] Context 传播分析 - 添加 TODO 注释
- [x] 日志统一化 - 设计决策说明

### ✅ 深度分析（已完成）
- [x] 错误处理分析 - 122+ 处问题
- [x] 并发安全分析 - 2 个关键竞态条件
- [x] API 设计分析 - 多个改进建议

### ✅ P0 竞态条件修复（已完成 - 2025-03-02）
- [x] Session.AddMessage/AddMessageWithMedia 竞态 - 添加 messagesMu sync.RWMutex 保护
- [x] Session.GetHistory 竞态 - 使用 RLock 读取
- [x] Session.Clear 竞态 - 添加锁保护
- [x] forceUnlock 通道竞态 - 添加 forceUnlockMu 保护
- [x] ForceUnlockChannel 竞态 - 加锁返回
- [x] 优化测试文件竞态 - optimizations_test.go 修复
- [x] 所有测试通过 race 检测 ✅
---

### ~~Week 1: 竞态条件修复~~ ✅ 已完成
**优先级**: P0 - CRITICAL  
**预估工作量**: 2 天
**完成日期**: 2025-03-02

#### ✅ 任务 1.1: 修复 Session.AddMessage 竞态
**位置**: `internal/session/manager.go:74-94`  
**实现**: 添加 `messagesMu sync.RWMutex` 保护 Messages 和 UpdatedAt 字段

#### ✅ 任务 1.2: 修复 forceUnlock 通道竞态
**位置**: `internal/session/manager.go:138-140`  
**实现**: 添加 `forceUnlockMu sync.Mutex` 保护通道操作

**验证**: `go test -race ./...` 全部通过 ✅
---

### Week 2: 错误处理改进
**优先级**: P1 - HIGH  
**预估工作量**: 3-5 天

#### 任务 2.1: 创建自定义错误类型
**位置**: 新建 `pkg/errors/errors.go`  
**工作量**: 2 小时

```go
package errors

import "errors"

var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrInvalidInput  = errors.New("invalid input")
    ErrTimeout       = errors.New("operation timeout")
)

type ToolError struct {
    ToolName string
    Phase    string
    Err      error
}

func (e *ToolError) Error() string {
    return fmt.Sprintf("tool %s failed at %s: %v", e.ToolName, e.Phase, e.Err)
}

func (e *ToolError) Unwrap() error {
    return e.Err
}
```

---

#### 任务 2.2: 错误包装改进（分批次）
**文件**:
1. `internal/tools/aigc.go` (18+ 处) - 3 小时
2. `internal/taskboard/tool.go` (15+ 处) - 2 小时
3. `internal/skills/loader.go` (3 处) - 1 小时

**模式**: 将所有 `return "", err` 改为 `return "", fmt.Errorf("...: %w", err)`

**示例**:
```go
// 修改前
if err != nil {
    return "", err
}

// 修改后
if err != nil {
    return "", fmt.Errorf("generate image: %w", err)
}
```

---

## 🚀 第三阶段：测试提升

### Week 3-4: 测试覆盖率提升
**优先级**: P1  
**目标**: 从 20% 提升到 50%  
**预估工作量**: 2 周

#### 任务 3.1: 核心模块测试
**重点模块**:
1. **Provider 集成测试** (3 天)
   - Mock HTTP responses
   - 测试所有 provider 实现
   - 错误处理测试

2. **Channel 并发测试** (2 天)
   - 多用户同时发送消息
   - 消息去重测试
   - 会话管理测试

3. **Memory 系统测试** (2 天)
   - 存储性能测试
   - 检索准确性测试
   - 并发访问测试

4. **Tool 执行测试** (2 天)
   - 工具安全测试
   - 并发工具调用
   - 错误恢复测试

**测试模板**:
```go
// internal/session/manager_test.go
func TestSessionConcurrentAccess(t *testing.T) {
    s := NewManager(NewMemoryStore(), 50)
    session := s.GetOrCreate("test-session")
    
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            session.AddMessage("user", "test message")
        }()
    }
    wg.Wait()
    
    // 验证没有竞态条件
    if len(session.Messages) != 100 {
        t.Errorf("expected 100 messages, got %d", len(session.Messages))
    }
}
```

---

## 📊 第四阶段：性能优化

### Week 5-6: 性能基准测试
**优先级**: P2  
**预估工作量**: 2 周

#### 任务 4.1: 建立性能基准
**目标**:
- 消息处理: P99 < 100ms
- 工具执行: P99 < 50ms
- 记忆检索: P99 < 10ms
- 并发会话: >1000 req/s

**基准测试文件**:
```go
// internal/agent/agent_bench_test.go
func BenchmarkProcessMessage(b *testing.B) {
    ag := setupBenchmarkAgent(b)
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = ag.ProcessMessage(ctx, "test-session", "benchmark message")
    }
}

// pkg/memory/hybrid_bench_test.go
func BenchmarkMemoryRetrieval(b *testing.B) {
    store := setupBenchmarkStore(b)
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = store.Search(ctx, "test query", 10)
    }
}
```

---

#### 任务 4.2: 字符串操作优化
**位置**: 85 处字符串拼接  
**工作量**: 1-2 天

**优化模式**:
```go
// 修改前
result := ""
for _, item := range items {
    result += item + ","  // O(n²)
}

// 修改后
var builder strings.Builder
for _, item := range items {
    builder.WriteString(item)
    builder.WriteString(",")
}
result := builder.String()  // O(n)
```

---

## 🎓 第五阶段：代码质量提升

### Week 7-8: API 标准化
**优先级**: P2  
**预估工作量**: 2 周

#### 任务 5.1: 添加核心接口
**新建接口**:
```go
// internal/tools/registry.go
type Registry interface {
    Register(t Tool)
    Unregister(name string)
    Get(name string) (Tool, bool)
    List() []Tool
    GetToolDefinitions() []map[string]interface{}
}

// internal/session/manager.go
type Manager interface {
    GetOrCreate(sessionID string) *Session
    Get(sessionID string) (*Session, bool)
    Delete(sessionID string)
}

// internal/providers/registry.go
type Registry interface {
    Register(name string, p Provider, spec *ProviderSpec)
    Get(name string) (Provider, bool)
    MatchProvider(model string) (Provider, *ProviderSpec)
    List() []string
}
```

---

#### 任务 5.2: Context 标准化
**工作量**: 2-3 天

**修复位置**:
1. `internal/tools/message.go:12` - 添加 context 参数
2. `pkg/tts/tts.go:338` - 修改为接受 context

**模式**:
```go
// 修改前
func SendMessage(channelName, to, content string) error

// 修改后
func SendMessage(ctx context.Context, channelName, to, content string) error
```

---

#### 任务 5.3: 合并 Cron 接口
**位置**: `internal/tools/cron.go`, `internal/agent/agent.go`  
**工作量**: 1-2 天

**当前**: 3 个接口 (`CronWrapper`, `CronService`, `CronServiceWithChannel`)  
**目标**: 1 个统一接口

```go
// internal/cron/service.go
type Service interface {
    SetChannelContext(channel, to string)
    AddJob(name string, schedule CronSchedule, message string, opts ...JobOption) (*CronJob, error)
    RemoveJob(id string) bool
    EnableJob(id string, enabled bool) *CronJob
    ListJobs(includeDisabled bool) []*CronJob
}
```

---

## 📈 成功指标

### 短期（1 个月）
- [x] 第一阶段优化完成
- [ ] P0 竞态条件修复
- [ ] 测试覆盖率 > 40%
- [ ] 所有测试通过 race 检测

### 中期（3 个月）
- [ ] 测试覆盖率 > 70%
- [ ] P0/P1 问题全部解决
- [ ] 性能基准建立
- [ ] 错误处理标准化

### 长期（6 个月）
- [ ] 测试覆盖率 > 80%
- [ ] API 完全标准化
- [ ] 性能指标全部达标
- [ ] 技术债务 < 10 个 TODO

---

## 🔧 工具使用

### 每日检查
```bash
# 运行测试
make test

# 检查代码质量
./scripts/check-errors.sh
./scripts/check-concurrency.sh
```

### 每周检查
```bash
# 技术债务检查
./scripts/tech-debt-check.sh

# 覆盖率报告
make test-coverage
open coverage.html
```

### 提交前检查
```bash
# 完整检查
make test
make lint
./scripts/check-concurrency.sh
./scripts/check-errors.sh
```

---

## 📚 相关文档

- [第一阶段优化总结](./OPTIMIZATIONS.md)
- [深度分析报告](./DEEP_ANALYSIS.md)
- [综合总结](./ANALYSIS_SUMMARY.md)
- [架构文档](./docs/ARCHITECTURE.md)

---

## 🎉 开始行动

1. **立即**: 修复 P0 竞态条件
2. **本周**: 创建自定义错误类型
3. **下周**: 开始测试覆盖率提升
4. **持续**: 每日运行检查脚本

所有分析、脚本、文档已准备就绪。现在可以开始第二阶段的关键问题修复！ 🚀
