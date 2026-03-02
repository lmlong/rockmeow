#!/bin/bash
# 并发安全检查脚本
# 用于检测潜在的并发问题和资源泄漏

set -e

echo "========================================="
echo "  LingGuard 并发安全检查"
echo "  Generated: $(date)"
echo "========================================="
echo ""

PROJECT_ROOT="/home/etsme/cy/go/lingguard"
cd "$PROJECT_ROOT"

echo "🔍 1. 检查未保护的 map 访问"
echo "----------------------------------------"
echo "map 使用总数:"
MAP_COUNT=$(find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -l "map\[" {} + | wc -l)
echo "  $MAP_COUNT 个文件使用 map"

echo ""
echo "使用 sync.Map 的文件:"
find . -name "*.go" -type f ! -path "*/vendor/*" \
    -exec grep -l "sync\.Map" {} + | sed 's|^\./||' || echo "  无"

echo ""
echo "使用 sync.RWMutex 保护的文件:"
find . -name "*.go" -type f ! -path "*/vendor/*" \
    -exec grep -l "sync\.RWMutex\|sync\.Mutex" {} + | sed 's|^\./||' | head -10

echo ""

echo "🔍 2. 检查 Goroutine 启动点"
echo "----------------------------------------"
GOROUTINE_COUNT=$(grep -r "go func\|go\s\+[a-zA-Z]" --include="*.go" . 2>/dev/null \
    | grep -v "test\|vendor" | wc -l)
echo "  找到 $GOROUTINE_COUNT 个 goroutine 启动点"

echo ""
echo "文件分布:"
grep -r "go func" --include="*.go" . 2>/dev/null \
    | grep -v "test\|vendor" \
    | cut -d: -f1 \
    | sort \
    | uniq -c \
    | sort -rn \
    | head -10

echo ""

echo "🔍 3. 检查未 defer 的资源关闭"
echo "----------------------------------------"

echo "未 defer 的 Close() 调用:"
find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -H "\.Close()" {} + \
    | grep -v "defer\|// \|return" \
    | head -10

echo ""

echo "未 defer 的文件操作:"
find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -H "os\.Open\|os\.Create\|os\.OpenFile" {} + \
    | grep -v "defer.*Close" \
    | head -5

echo ""

echo "🔍 4. 检查 HTTP Response Body 关闭"
echo "----------------------------------------"
echo "HTTP 请求未关闭 response body:"
find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -H "\.Do(\|http\.Get\|http\.Post" {} + \
    | grep -v "defer.*Body\.Close" \
    | head -5

echo ""

echo "🔍 5. 检查 Channel 使用"
echo "----------------------------------------"
echo "Channel 定义:"
grep -r "chan " --include="*.go" . 2>/dev/null \
    | grep -v "test\|vendor" \
    | wc -l

echo ""
echo "无缓冲 channel (潜在死锁风险):"
grep -r "chan [A-Z]\|chan [a-z]" --include="*.go" . 2>/dev/null \
    | grep -v "test\|vendor\|chan struct" \
    | grep -v "make(chan.*," \
    | head -5

echo ""

echo "🔍 6. 检查 WaitGroup 使用"
echo "----------------------------------------"
echo "使用 WaitGroup 的文件:"
find . -name "*.go" -type f ! -path "*/vendor/*" \
    -exec grep -l "sync\.WaitGroup" {} + \
    | sed 's|^\./||'

echo ""

echo "🔍 7. 检查 Context 传播"
echo "----------------------------------------"
echo "使用 context.Background() 的文件:"
find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -l "context\.Background()" {} + \
    | sed 's|^\./||' \
    | head -10

echo ""
echo "使用 context.TODO() 的文件:"
find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -l "context\.TODO()" {} + \
    | sed 's|^\./||' || echo "  无"

echo ""

echo "🔍 8. 检查 Select 超时"
echo "----------------------------------------"
echo "使用 select 的文件:"
find . -name "*.go" -type f ! -path "*/vendor/*" ! -name "*_test.go" \
    -exec grep -l "select {" {} + \
    | sed 's|^\./||'

echo ""

echo "✅ 检查完成！"
echo ""
echo "建议："
echo "1. 检查所有 map 访问是否加锁"
echo "2. 确保所有 goroutine 都能优雅关闭"
echo "3. 所有 Close() 调用应该使用 defer"
echo "4. HTTP response body 应该始终关闭"
echo "5. 长时间运行的 goroutine 应该响应 context.Done()"
echo ""
echo "运行 'make test-race' 进行并发检测"
