---
name: cron
description: 定时提醒和任务管理。当用户说"提醒我"、"X点提醒"、"X分钟后提醒"、"每天X点"、"定时"、"日程"、"计划任务"、"闹钟"、"别忘了"、"查看定时任务"、"删除定时任务"、"清空所有任务"、"取消提醒"时，必须先加载此 skill
metadata: {"nanobot":{"emoji":"⏰"}}
---

# 定时任务管理

使用 `cron` 工具管理提醒和定时任务。

## ⚠️ 核心规则：execute 参数

| 参数值 | 含义 | 适用场景 |
|--------|------|----------|
| `execute=false` (默认) | 仅发送通知 | 纯提醒、闹钟、备忘 |
| `execute=true` | **先执行任务再推送结果** | 需要搜索、查询、分析、整理、生成等操作 |

## 🚨 必须设置 execute=true 的场景

**以下场景必须设置 `execute=true`，否则任务只会发送文字通知！**

| 场景 | 示例 | execute |
|------|------|:-------:|
| 🔍 搜索/查询 | "每天搜索AI新闻" | `true` |
| 📊 收集/整理 | "每周整理行业动态" | `true` |
| 📈 分析/报告 | "每天分析股票走势" | `true` |
| 🌐 网络请求 | "定时检查网站状态" | `true` |
| 🤖 调用工具 | "定时生成日报" | `true` |
| 📰 推送内容 | "推送天气预报" | `true` |
| 💬 社交互动 | "定时发帖到Moltbook" | `true` |

## 仅提醒场景 (execute=false)

| 场景 | 示例 |
|------|------|
| ⏰ 闹钟 | "7点叫醒我" |
| 📋 会议提醒 | "10点提醒我开会" |
| 💊 服药提醒 | "每8小时提醒我吃药" |
| ☕ 休息提醒 | "每小时提醒我休息" |
| 📝 备忘 | "别忘了下午3点打电话" |

## 触发关键词

当用户提到以下内容时使用此工具：
- "提醒我"、"设置提醒"
- "X分钟后提醒我"
- "定时任务"、"日程"、"计划"
- "每天/每周 X 点提醒"
- "定时搜索/收集/整理/推送"

## 用法

### 查看任务列表
```json
{"action": "list"}
```

### 添加提醒（仅通知）
```json
{
  "action": "add",
  "name": "任务名称",
  "schedule": "at:in 5m",
  "message": "提醒内容"
}
```

### 添加执行任务（搜索、推送等）
```json
{
  "action": "add",
  "name": "每日AI新闻推送",
  "schedule": "cron:0 9 * * *",
  "message": "搜索最新的AI新闻，整理成摘要推送给我",
  "execute": true
}
```

### 删除任务
```json
{"action": "remove", "job_id": "任务ID"}
```

### 启用/禁用任务
```json
{"action": "enable", "job_id": "任务ID"}
{"action": "disable", "job_id": "任务ID"}
```

## Schedule 格式

| 格式 | 示例 | 说明 |
|------|------|------|
| `at:in <时长>` | `at:in 5m` | 5分钟后 |
| `at:in <时长>` | `at:in 1h` | 1小时后 |
| `at:<日期时间>` | `at:2026-02-22 18:00` | 指定时间 |
| `every:<时长>` | `every:1h` | 每小时 |
| `every:<时长>` | `every:24h` | 每天 |
| `cron:<表达式>` | `cron:0 9 * * *` | 每天9点 |
| `cron:<表达式>` | `cron:30 22 * * *` | 每天22:30 |

## 示例对话

### 仅提醒 (execute=false)

**用户**: 5分钟后提醒我休息一下
**调用**: `{"action": "add", "name": "休息提醒", "schedule": "at:in 5m", "message": "该休息一下了！"}`

**用户**: 每天早上9点提醒我开会
**调用**: `{"action": "add", "name": "早会提醒", "schedule": "cron:0 9 * * *", "message": "该开早会了"}`

**用户**: 每小时提醒我喝水
**调用**: `{"action": "add", "name": "喝水提醒", "schedule": "every:1h", "message": "该喝水了！"}`

### 需要执行 (execute=true) ⚠️

**用户**: 每天9点半，搜索最新的AI新闻，整理好后推送给我
**调用**: `{"action": "add", "name": "AI新闻推送", "schedule": "cron:30 9 * * *", "message": "搜索最新的AI新闻，整理成摘要", "execute": true}`

**用户**: 每天早上8点告诉我今天天气
**调用**: `{"action": "add", "name": "每日天气", "schedule": "cron:0 8 * * *", "message": "查询今天北京天气，告诉我温度、是否下雨、穿什么衣服", "execute": true}`

**用户**: 每周五下午5点，总结本周工作
**调用**: `{"action": "add", "name": "周报提醒", "schedule": "cron:0 17 * * 5", "message": "提醒我写周报，并列出本周可能完成的事项", "execute": true}`

**用户**: 每天定时检查某个网站是否正常
**调用**: `{"action": "add", "name": "网站监控", "schedule": "every:1h", "message": "访问 https://example.com 检查是否正常响应", "execute": true}`

**用户**: 每天10点给我推送一条励志语录
**调用**: `{"action": "add", "name": "每日语录", "schedule": "cron:0 10 * * *", "message": "搜索一条励志语录推送给我", "execute": true}`

**用户**: 每天定时发一条动态到Moltbook
**调用**: `{"action": "add", "name": "Moltbook发帖", "schedule": "cron:0 12 * * *", "message": "生成一条有趣的AI相关动态，发到Moltbook", "execute": true}`

### 管理任务

**用户**: 查看我的定时任务
**调用**: `{"action": "list"}`

**用户**: 取消xxx任务
**调用**: `{"action": "remove", "job_id": "xxx"}` (先 list 获取 ID)

## 注意事项

- 这是 LingGuard 的内置定时系统，不是系统 crontab
- 任务会在指定时间发送通知或执行任务到创建任务时的渠道
- 任务数据持久化存储，重启服务不会丢失
- **执行模式**会在时间到后调用 Agent 处理 message 中的任务
- **关键判断**：如果 message 中的内容需要 Agent 执行任何操作（搜索、查询、分析等），必须设置 `execute=true`
