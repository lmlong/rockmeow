import { v } from "convex/values";
import { queryGeneric, mutationGeneric } from "convex/server";

// 使用通用版本并手动添加类型
const query = queryGeneric;
const mutation = mutationGeneric;

// 定义任务状态类型
type TaskStatus = "pending" | "running" | "completed" | "failed";
type TaskAssignee = "user" | "ai" | "both";
type TaskPriority = "low" | "medium" | "high";

// 获取所有任务
export const list = query({
  args: {},
  handler: async (ctx) => {
    return await ctx.db.query("tasks").order("desc").collect();
  },
});

// 按状态获取任务
export const listByStatus = query({
  args: {
    status: v.optional(v.union(
      v.literal("pending"),
      v.literal("running"),
      v.literal("completed"),
      v.literal("failed"),
    )),
  },
  handler: async (ctx, args) => {
    if (args.status) {
      return await ctx.db
        .query("tasks")
        .withIndex("by_status", (q) => q.eq("status", args.status as TaskStatus))
        .order("desc")
        .collect();
    }
    return await ctx.db.query("tasks").order("desc").collect();
  },
});

// 按会话ID获取任务
export const listBySession = query({
  args: { sessionId: v.string() },
  handler: async (ctx, args) => {
    return await ctx.db
      .query("tasks")
      .withIndex("by_session", (q) => q.eq("sessionId", args.sessionId))
      .order("desc")
      .collect();
  },
});

// 获取单个任务
export const get = query({
  args: { id: v.id("tasks") },
  handler: async (ctx, args) => {
    return await ctx.db.get(args.id);
  },
});

// 创建任务
export const create = mutation({
  args: {
    title: v.string(),
    description: v.optional(v.string()),
    assignee: v.optional(v.union(
      v.literal("user"),
      v.literal("ai"),
      v.literal("both"),
    )),
    sessionId: v.optional(v.string()),
    subagentId: v.optional(v.string()),
    priority: v.optional(v.union(
      v.literal("low"),
      v.literal("medium"),
      v.literal("high"),
    )),
    tags: v.optional(v.array(v.string())),
    metadata: v.optional(v.object({
      source: v.optional(v.string()),
      command: v.optional(v.string()),
      workingDirectory: v.optional(v.string()),
    })),
  },
  handler: async (ctx, args) => {
    const now = Date.now();
    return await ctx.db.insert("tasks", {
      title: args.title,
      description: args.description,
      status: "pending" as TaskStatus,
      assignee: (args.assignee ?? "ai") as TaskAssignee,
      sessionId: args.sessionId,
      subagentId: args.subagentId,
      priority: args.priority as TaskPriority | undefined,
      tags: args.tags,
      createdAt: now,
      updatedAt: now,
      metadata: args.metadata,
    });
  },
});

// 更新任务状态
export const updateStatus = mutation({
  args: {
    id: v.id("tasks"),
    status: v.union(
      v.literal("pending"),
      v.literal("running"),
      v.literal("completed"),
      v.literal("failed"),
    ),
    result: v.optional(v.string()),
    error: v.optional(v.string()),
  },
  handler: async (ctx, args) => {
    const now = Date.now();
    const updates: Record<string, unknown> = {
      status: args.status,
      updatedAt: now,
    };

    if (args.status === "running") {
      updates.startedAt = now;
    }
    if (args.status === "completed") {
      updates.completedAt = now;
      if (args.result) updates.result = args.result;
    }
    if (args.status === "failed" && args.error) {
      updates.error = args.error;
    }

    await ctx.db.patch(args.id, updates);
    return await ctx.db.get(args.id);
  },
});

// 更新任务
export const update = mutation({
  args: {
    id: v.id("tasks"),
    title: v.optional(v.string()),
    description: v.optional(v.string()),
    assignee: v.optional(v.union(
      v.literal("user"),
      v.literal("ai"),
      v.literal("both"),
    )),
    priority: v.optional(v.union(
      v.literal("low"),
      v.literal("medium"),
      v.literal("high"),
    )),
    tags: v.optional(v.array(v.string())),
  },
  handler: async (ctx, args) => {
    const { id, ...updates } = args;
    const filteredUpdates: Record<string, unknown> = {
      updatedAt: Date.now(),
    };

    if (updates.title !== undefined) filteredUpdates.title = updates.title;
    if (updates.description !== undefined) filteredUpdates.description = updates.description;
    if (updates.assignee !== undefined) filteredUpdates.assignee = updates.assignee;
    if (updates.priority !== undefined) filteredUpdates.priority = updates.priority;
    if (updates.tags !== undefined) filteredUpdates.tags = updates.tags;

    await ctx.db.patch(id, filteredUpdates);
    return await ctx.db.get(id);
  },
});

// 删除任务
export const remove = mutation({
  args: { id: v.id("tasks") },
  handler: async (ctx, args) => {
    await ctx.db.delete(args.id);
    return { success: true };
  },
});

// 批量同步任务（供外部系统调用）
export const syncFromLingguard = mutation({
  args: {
    tasks: v.array(v.object({
      externalId: v.string(),
      title: v.string(),
      description: v.optional(v.string()),
      status: v.union(
        v.literal("pending"),
        v.literal("running"),
        v.literal("completed"),
        v.literal("failed"),
      ),
      assignee: v.optional(v.union(
        v.literal("user"),
        v.literal("ai"),
        v.literal("both"),
      )),
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
      startedAt: v.optional(v.number()),
      completedAt: v.optional(v.number()),
      metadata: v.optional(v.object({
        source: v.optional(v.string()),
        command: v.optional(v.string()),
        workingDirectory: v.optional(v.string()),
      })),
    })),
  },
  handler: async (ctx, args) => {
    const now = Date.now();
    const results: Array<{ externalId: string; action: string }> = [];

    for (const task of args.tasks) {
      // 查找是否已存在相同 externalId 的任务（使用 metadata.source 作为 externalId）
      const existing = await ctx.db
        .query("tasks")
        .filter((q) =>
          q.eq(q.field("metadata.source"), task.externalId)
        )
        .first();

      if (existing) {
        // 更新现有任务
        await ctx.db.patch(existing._id, {
          title: task.title,
          description: task.description,
          status: task.status,
          assignee: (task.assignee ?? existing.assignee) as TaskAssignee,
          sessionId: task.sessionId,
          subagentId: task.subagentId,
          priority: task.priority as TaskPriority | undefined,
          tags: task.tags,
          result: task.result,
          error: task.error,
          startedAt: task.startedAt,
          completedAt: task.completedAt,
          updatedAt: now,
          metadata: task.metadata,
        });
        results.push({ externalId: task.externalId, action: "updated" });
      } else {
        // 创建新任务
        await ctx.db.insert("tasks", {
          title: task.title,
          description: task.description,
          status: task.status,
          assignee: (task.assignee ?? "ai") as TaskAssignee,
          sessionId: task.sessionId,
          subagentId: task.subagentId,
          priority: task.priority as TaskPriority | undefined,
          tags: task.tags,
          result: task.result,
          error: task.error,
          createdAt: now,
          updatedAt: now,
          startedAt: task.startedAt,
          completedAt: task.completedAt,
          metadata: { ...task.metadata, source: task.externalId },
        });
        results.push({ externalId: task.externalId, action: "created" });
      }
    }

    return results;
  },
});

// 清除已完成/失败的任务
export const clearCompleted = mutation({
  args: {},
  handler: async (ctx) => {
    const completed = await ctx.db
      .query("tasks")
      .withIndex("by_status", (q) =>
        q.eq("status", "completed")
      )
      .collect();

    const failed = await ctx.db
      .query("tasks")
      .withIndex("by_status", (q) =>
        q.eq("status", "failed")
      )
      .collect();

    let deleted = 0;
    for (const task of [...completed, ...failed]) {
      await ctx.db.delete(task._id);
      deleted++;
    }

    return { deleted };
  },
});
