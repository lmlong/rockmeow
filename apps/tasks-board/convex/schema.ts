import { defineSchema, defineTable } from "convex/server";
import { v } from "convex/values";

export default defineSchema({
  tasks: defineTable({
    title: v.string(),
    description: v.optional(v.string()),
    status: v.union(
      v.literal("pending"),     // 待办
      v.literal("running"),     // 进行中
      v.literal("completed"),   // 已完成
      v.literal("failed"),      // 失败
    ),
    assignee: v.union(
      v.literal("user"),        // 用户
      v.literal("ai"),          // AI 助手
      v.literal("both"),        // 协作
    ),
    sessionId: v.optional(v.string()),
    subagentId: v.optional(v.string()),
    priority: v.optional(v.union(
      v.literal("low"),
      v.literal("medium"),
      v.literal("high"),
    )),
    tags: v.optional(v.array(v.string())),
    result: v.optional(v.string()),
    error: v.optional(v.string()),
    createdAt: v.number(),
    updatedAt: v.number(),
    startedAt: v.optional(v.number()),
    completedAt: v.optional(v.number()),
    metadata: v.optional(v.object({
      source: v.optional(v.string()),
      command: v.optional(v.string()),
      workingDirectory: v.optional(v.string()),
    })),
  })
    .index("by_status", ["status"])
    .index("by_session", ["sessionId"])
    .index("by_assignee", ["assignee"]),
});
