---
name: weather
description: Get current weather and forecasts for Chinese cities (心知天气 API).
homepage: https://www.seniverse.com/api
metadata: {"nanobot":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---
# Weather 天气查询

使用心知天气 API 查询中国城市天气。

## 天气实况

```bash
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=城市名&language=zh-Hans&unit=c"
```

## 3天天气预报

```bash
curl "https://api.seniverse.com/v3/weather/daily.json?key=SFafUBI6JpbX2AhfD&location=城市名&language=zh-Hans&unit=c&start=0&days=3"
```

## 常用城市代码

可以直接使用城市名（中文或拼音）：

```bash
# 北京
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=beijing&language=zh-Hans&unit=c"

# 上海
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=shanghai&language=zh-Hans&unit=c"

# 广州
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=guangzhou&language=zh-Hans&unit=c"

# 深圳
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=shenzhen&language=zh-Hans&unit=c"

# 成都
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=chengdu&language=zh-Hans&unit=c"

# 杭州
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=hangzhou&language=zh-Hans&unit=c"
```

## 参数说明

| 参数 | 说明 |
|------|------|
| `key` | API 密钥 |
| `location` | 城市（城市名/城市ID/IP地址） |
| `language` | 语言：`zh-Hans`（简体中文）或 `en`（英文） |
| `unit` | 温度单位：`c`（摄氏）或 `f`（华氏） |
| `days` | 预报天数（1-3） |

## 示例用法

```bash
# 查询北京当前天气
curl "https://api.seniverse.com/v3/weather/now.json?key=SFafUBI6JpbX2AhfD&location=北京&language=zh-Hans&unit=c"

# 查询北京3天预报
curl "https://api.seniverse.com/v3/weather/daily.json?key=SFafUBI6JpbX2AhfD&location=北京&language=zh-Hans&unit=c&days=3"
```

## 返回格式

天气实况返回示例：
```json
{
  "results": [{
    "location": {
      "name": "北京"
    },
    "now": {
      "text": "晴",
      "code": "0",
      "temperature": "25"
    }
  }]
}
```
