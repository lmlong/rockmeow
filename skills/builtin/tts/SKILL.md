---
name: tts
description: 文字转语音（通义千问 TTS）
metadata: {"nanobot":{"emoji":"🔊"}}
---

# 语音合成

使用 `tts` 工具将文字转换为语音（阿里云通义千问 TTS）。

## 触发关键词

- "朗读这段文字"、"读出来"
- "转成语音"、"生成语音"
- "用语音说"、"播放"

## 用法

```json
{
  "action": "synthesize",
  "text": "你好，这是一段测试语音",
  "voice": "Cherry"
}
```

## 可用音色

| 音色 | 风格 | 适用场景 |
|------|------|----------|
| Cherry | 甜美女声 | 默认，通用 |
| Serena | 温柔女声 | 讲故事、抒情 |
| Ethan | 沉稳男声 | 新闻、正式场合 |
| Chelsie | 活力女声 | 活泼、热情 |
| Momo | 可爱童声 | 儿童内容 |
| Vivian | 知性女声 | 教学、解说 |
| Moon | 亲切男声 | 日常对话 |
| Maia | 清澈女声 | 清新、自然 |
| Kai | 磁性男声 | 有感染力 |

## 参数

| 参数 | 说明 | 必填 |
|------|------|------|
| action | 固定值 "synthesize" | ✅ |
| text | 要合成的文本 | ✅ |
| voice | 音色选择，默认 Cherry | ❌ |

## 限制

- 文本最大长度：5000 字符
- 输出格式：WAV (24kHz)
- 音频 URL 有效期：24 小时

## 示例

**用户**: 把这段话读出来：今天天气真好
**调用**: `{"action": "synthesize", "text": "今天天气真好", "voice": "Cherry"}`

**用户**: 用男声朗读这段新闻
**调用**: `{"action": "synthesize", "text": "...", "voice": "Ethan"}`
