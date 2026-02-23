"use client";

import { useState } from "react";
import { useQuery, useMutation } from "convex/react";
import { api } from "@/convex/_generated/api";
import type { Doc } from "@/convex/_generated/dataModel";
import { TaskColumn } from "./TaskColumn";
import { TaskForm } from "@/components/task/TaskForm";
import { Button } from "@/components/ui/Button";
import { Plus, Trash2 } from "lucide-react";

export function TaskBoard() {
  const [showForm, setShowForm] = useState(false);
  const tasks = useQuery(api.tasks.list);
  const clearCompleted = useMutation(api.tasks.clearCompleted);

  if (tasks === undefined) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">加载中...</div>
      </div>
    );
  }

  const pendingTasks = tasks.filter((t: Doc<"tasks">) => t.status === "pending");
  const runningTasks = tasks.filter((t: Doc<"tasks">) => t.status === "running");
  const completedTasks = tasks.filter((t: Doc<"tasks">) => t.status === "completed");
  const failedTasks = tasks.filter((t: Doc<"tasks">) => t.status === "failed");

  const handleClearCompleted = async () => {
    if (confirm("确定要清除所有已完成和失败的任务吗？")) {
      await clearCompleted();
    }
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h1 className="text-2xl font-bold text-gray-900">任务看板</h1>
          <div className="text-sm text-gray-500">
            共 {tasks.length} 项任务
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClearCompleted}
            disabled={completedTasks.length === 0 && failedTasks.length === 0}
          >
            <Trash2 className="w-4 h-4 mr-1" />
            清除已完成
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={() => setShowForm(!showForm)}
          >
            <Plus className="w-4 h-4 mr-1" />
            新建任务
          </Button>
        </div>
      </div>

      {/* Task Form */}
      {showForm && (
        <div className="max-w-md">
          <TaskForm onSuccess={() => setShowForm(false)} onCancel={() => setShowForm(false)} />
        </div>
      )}

      {/* Board Columns */}
      <div className="flex gap-4 overflow-x-auto pb-4">
        <TaskColumn
          title="待办"
          status="pending"
          tasks={pendingTasks}
          color="bg-gray-200"
        />
        <TaskColumn
          title="进行中"
          status="running"
          tasks={runningTasks}
          color="bg-blue-200"
        />
        <TaskColumn
          title="已完成"
          status="completed"
          tasks={completedTasks}
          color="bg-green-200"
        />
        <TaskColumn
          title="失败"
          status="failed"
          tasks={failedTasks}
          color="bg-red-200"
        />
      </div>
    </div>
  );
}
