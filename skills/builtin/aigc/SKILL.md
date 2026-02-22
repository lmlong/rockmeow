---
name: aigc
description: 图像和视频生成（通义万相）
metadata: {"nanobot":{"emoji":"🎨"}}
---

# 图像和视频生成

使用 `aigc` 工具生成图片和视频（阿里云通义万相）。

## 触发关键词

- "生成图片"、"画一张图"、"帮我画"
- "生成视频"、"做个视频"
- "把这张图变成视频"
- "用这个视频生成"

## ⚠️ 重要规则

- **每次调用都会生成新内容**，不要从历史记录返回旧路径
- **生成成功后**，使用 memory 工具记录：`{"action": "log", "event": "生成xxx图片，保存到 xxx.png"}`

## 用法

### 文生图
```json
{
  "action": "generate_image",
  "prompt": "一只可爱的猫咪坐在椅子上",
  "size": "1024x1024",
  "style": "anime"
}
```

### 文生视频
```json
{
  "action": "generate_video",
  "prompt": "一只猫在花园里散步",
  "duration": 5
}
```

### 图生视频
```json
{
  "action": "generate_video_from_image",
  "prompt": "猫开始走动",
  "image_path": "/path/to/image.png",
  "duration": 5
}
```

### 视频生视频（保持角色一致性）
```json
{
  "action": "generate_video_from_video",
  "prompt": "人物开始跳舞",
  "video_path": "/path/to/video.mp4",
  "duration": 5
}
```

## 参数说明

| 参数 | 说明 | 必填 |
|------|------|------|
| action | 动作类型 | ✅ |
| prompt | 文字描述 | ✅ |
| image_path | 图片路径（图生视频） | 图生视频时 |
| video_path | 视频路径（视频生视频） | 视频生视频时 |
| duration | 视频时长（秒），默认5，最大10 | ❌ |
| size | 图片尺寸，如 1024x1024 | ❌ |
| style | 风格：anime, realistic, 3d | ❌ |

## 文件路径

| 类型 | 路径 |
|------|------|
| 生成的图片/视频 | `~/.lingguard/workspace/generated/` |
| 聊天下载的媒体 | `~/.lingguard/workspace/media/` |

## 可用模型

| 模型 | 用途 |
|------|------|
| wan2.6-t2i | 文生图（默认） |
| wan2.6-t2v | 文生视频 |
| wan2.6-i2v-flash | 图生视频 |
| wan2.6-r2v-flash | 视频生视频 |

## 示例对话

**用户**: 帮我画一只可爱的小猫
**调用**: `{"action": "generate_image", "prompt": "一只可爱的猫咪，卡通风格"}`

**用户**: 把这张图片变成动起来的视频
**调用**: 先获取图片路径，然后 `{"action": "generate_video_from_image", "prompt": "画面开始动起来", "image_path": "..."}`

## 时长限制

- 文生视频：最大 10 秒
- 图生视频：最大 15 秒（可配置）
- 视频生视频：5 或 10 秒
