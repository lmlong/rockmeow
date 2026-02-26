---
name: web
description: 网页搜索和抓取。当用户说"搜索"、"查一下"、"帮我找"、"网上查"、"打开这个网页"、"获取网页内容"、"这个链接说什么"、"抓取网页"时，必须先加载此 skill 了解用法
metadata: {"nanobot":{"emoji":"🔍"}}
---

# 网页搜索和抓取

使用 `web_search` 和 `web_fetch` 工具获取网络信息。

## 触发关键词

- "搜索"、"查一下"、"帮我找"
- "打开这个网页"、"获取网页内容"
- "这个链接的内容是什么"

## web_search - 网页搜索

使用 Tavily AI 搜索网页，返回标题、链接和摘要。

```json
{
  "query": "Go 语言并发编程",
  "count": 5,
  "searchDepth": "basic",
  "includeAnswer": true
}
```

### 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| query | 搜索关键词 | 必填 |
| count | 结果数量 (1-10) | 5 |
| searchDepth | basic/advanced | basic |
| includeAnswer | 是否包含 AI 总结 | true |

## web_fetch - 网页抓取

获取网页内容，自动转换为 Markdown 或纯文本。

```json
{
  "url": "https://example.com/article",
  "extractMode": "markdown",
  "maxChars": 10000
}
```

### 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| url | 网页地址 | 必填 |
| extractMode | markdown/text | markdown |
| maxChars | 最大字符数 | 50000 |

## 使用场景

| 场景 | 工具 |
|------|------|
| 搜索信息 | web_search |
| 获取特定网页内容 | web_fetch |
| 搜索后深入阅读 | 先 web_search，再 web_fetch |

## 示例

### 搜索
```json
{"query": "2024 年 AI 发展趋势", "count": 5}
```

### 抓取网页
```json
{"url": "https://docs.example.com/guide", "extractMode": "markdown"}
```

### 组合使用
1. 搜索找到相关链接
2. 用 web_fetch 获取详细内容

## 注意

- web_search 需要 Tavily API Key
- 某些网站可能禁止抓取
- 内容过长会自动截断
