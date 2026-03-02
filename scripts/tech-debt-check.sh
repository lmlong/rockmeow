#!/bin/bash
# Technical Debt Analysis Script
# Run this script weekly to track technical debt metrics

echo "======================================"
echo "  LingGuard Technical Debt Report"
echo "  Generated: $(date)"
echo "======================================"
echo ""

echo "📋 TODO Comments by Priority:"
echo "--------------------------------------"
echo "P0 (Critical):   $(grep -r 'TODO.*P0' --include='*.go' . 2>/dev/null | wc -l)"
echo "P1 (High):       $(grep -r 'TODO.*P1' --include='*.go' . 2>/dev/null | wc -l)"
echo "P2 (Medium):     $(grep -r 'TODO.*P2' --include='*.go' . 2>/dev/null | wc -l)"
echo "P3 (Low):        $(grep -r 'TODO.*P3' --include='*.go' . 2>/dev/null | wc -l)"
echo "Unprioritized:   $(grep -r 'TODO' --include='*.go' . 2>/dev/null | grep -v 'P0\|P1\|P2\|P3' | wc -l)"
echo ""

echo "🔥 FIXME Comments:"
echo "--------------------------------------"
grep -r 'FIXME' --include='*.go' . 2>/dev/null | wc -l
echo ""

echo "⚠️  HACK Comments:"
echo "--------------------------------------"
grep -r 'HACK' --include='*.go' . 2>/dev/null | wc -l
echo ""

echo "📁 Large Files (>500 lines):"
echo "--------------------------------------"
find . -name '*.go' -type f ! -path './vendor/*' ! -path './.git/*' -exec wc -l {} + 2>/dev/null | awk '$1 > 500 {print $1, $2}' | sort -rn
echo ""

echo "🧪 Test Coverage:"
echo "--------------------------------------"
if command -v go &> /dev/null; then
    go test -cover ./... 2>/dev/null | grep -E '^ok|^---' || echo "Run 'make test-coverage' for detailed coverage"
fi
echo ""

echo "🚨 Context.Background() Usage:"
echo "--------------------------------------"
echo "Total occurrences: $(grep -r 'context.Background()' --include='*.go' . 2>/dev/null | wc -l)"
echo "Files affected:"
grep -r 'context.Background()' --include='*.go' . 2>/dev/null | cut -d: -f1 | sort -u | head -10
echo ""

echo "📊 Code Quality Metrics:"
echo "--------------------------------------"
echo "Production code lines: $(find . -name '*.go' -type f ! -name '*_test.go' ! -path './vendor/*' -exec cat {} + 2>/dev/null | wc -l)"
echo "Test code lines:       $(find . -name '*_test.go' -type f ! -path './vendor/*' -exec cat {} + 2>/dev/null | wc -l)"
echo "Total packages:        $(find . -name '*.go' -type f ! -path './vendor/*' -exec dirname {} \; 2>/dev/null | sort -u | wc -l)"
echo ""

echo "✅ Report generated successfully!"
echo "Next steps:"
echo "1. Review P0 and P1 items"
echo "2. Schedule refactoring for large files"
echo "3. Address FIXME comments"
echo "4. Run 'make test-coverage' for detailed coverage report"
