import { NextRequest, NextResponse } from "next/server";
import { ConvexHttpClient } from "convex/browser";
import { api } from "@/convex/_generated/api";

const convex = new ConvexHttpClient(process.env.NEXT_PUBLIC_CONVEX_URL!);

// API Key 验证
function validateApiKey(request: NextRequest): boolean {
  const apiKey = request.headers.get("X-API-Key");
  const expectedKey = process.env.TASKS_API_KEY;

  if (!expectedKey) {
    console.warn("TASKS_API_KEY not configured");
    return true; // 开发环境下允许无 key
  }

  return apiKey === expectedKey;
}

// GET /api/tasks - 获取任务列表
export async function GET(request: NextRequest) {
  if (!validateApiKey(request)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const { searchParams } = new URL(request.url);
    const status = searchParams.get("status") as
      | "pending"
      | "running"
      | "completed"
      | "failed"
      | null;
    const sessionId = searchParams.get("sessionId");

    let tasks;
    if (sessionId) {
      tasks = await convex.query(api.tasks.listBySession, { sessionId });
    } else if (status) {
      tasks = await convex.query(api.tasks.listByStatus, { status });
    } else {
      tasks = await convex.query(api.tasks.list, {});
    }

    return NextResponse.json({ tasks });
  } catch (error) {
    console.error("Failed to fetch tasks:", error);
    return NextResponse.json(
      { error: "Failed to fetch tasks" },
      { status: 500 }
    );
  }
}

// POST /api/tasks - 创建任务或批量同步
export async function POST(request: NextRequest) {
  if (!validateApiKey(request)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = await request.json();

    // 批量同步模式
    if (body.tasks && Array.isArray(body.tasks)) {
      const results = await convex.mutation(api.tasks.syncFromLingguard, {
        tasks: body.tasks.map((t: Record<string, unknown>) => ({
          externalId: t.externalId as string,
          title: t.title as string,
          description: t.description as string | undefined,
          status: t.status as "pending" | "running" | "completed" | "failed",
          assignee: t.assignee as "user" | "ai" | "both" | undefined,
          sessionId: t.sessionId as string | undefined,
          subagentId: t.subagentId as string | undefined,
          priority: t.priority as "low" | "medium" | "high" | undefined,
          tags: t.tags as string[] | undefined,
          result: t.result as string | undefined,
          error: t.error as string | undefined,
          startedAt: t.startedAt as number | undefined,
          completedAt: t.completedAt as number | undefined,
          metadata: t.metadata as
            | {
                source?: string;
                command?: string;
                workingDirectory?: string;
              }
            | undefined,
        })),
      });
      return NextResponse.json({ results });
    }

    // 单个任务创建
    const { title, description, assignee, sessionId, subagentId, priority, tags, metadata } = body;

    if (!title) {
      return NextResponse.json({ error: "Title is required" }, { status: 400 });
    }

    const taskId = await convex.mutation(api.tasks.create, {
      title,
      description,
      assignee: assignee || "ai",
      sessionId,
      subagentId,
      priority,
      tags,
      metadata,
    });

    return NextResponse.json({ taskId }, { status: 201 });
  } catch (error) {
    console.error("Failed to create task:", error);
    return NextResponse.json(
      { error: "Failed to create task" },
      { status: 500 }
    );
  }
}

// PATCH /api/tasks - 更新任务状态
export async function PATCH(request: NextRequest) {
  if (!validateApiKey(request)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { id, status, result, error } = body;

    if (!id || !status) {
      return NextResponse.json(
        { error: "Task id and status are required" },
        { status: 400 }
      );
    }

    const task = await convex.mutation(api.tasks.updateStatus, {
      id,
      status,
      result,
      error,
    });

    return NextResponse.json({ task });
  } catch (error) {
    console.error("Failed to update task:", error);
    return NextResponse.json(
      { error: "Failed to update task" },
      { status: 500 }
    );
  }
}

// DELETE /api/tasks - 删除任务
export async function DELETE(request: NextRequest) {
  if (!validateApiKey(request)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const { searchParams } = new URL(request.url);
    const id = searchParams.get("id");

    if (!id) {
      return NextResponse.json(
        { error: "Task id is required" },
        { status: 400 }
      );
    }

    await convex.mutation(api.tasks.remove, { id });

    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Failed to delete task:", error);
    return NextResponse.json(
      { error: "Failed to delete task" },
      { status: 500 }
    );
  }
}
