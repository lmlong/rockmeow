"use client";

import { useState } from "react";
import { useMutation } from "convex/react";
import { Doc, Id } from "@/convex/_generated/dataModel";
import { api } from "@/convex/_generated/api";
import {
  TaskStatusBadge,
  TaskAssigneeBadge,
  TaskPriorityBadge,
} from "@/components/task/TaskStatusBadge";
import { Button } from "@/components/ui/Button";
import {
  Trash2,
  Play,
  Check,
  X,
  MoreVertical,
  Clock,
  User,
  Bot,
} from "lucide-react";

interface TaskCardProps {
  task: Doc<"tasks">;
}

function formatTime(timestamp: number | undefined): string {
  if (!timestamp) return "-";
  return new Date(timestamp).toLocaleString("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function TaskCard({ task }: TaskCardProps) {
  const [showActions, setShowActions] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);

  const updateStatus = useMutation(api.tasks.updateStatus);
  const deleteTask = useMutation(api.tasks.remove);

  const handleStatusChange = async (
    status: "pending" | "running" | "completed" | "failed"
  ) => {
    await updateStatus({ id: task._id, status });
    setShowActions(false);
  };

  const handleDelete = async () => {
    if (confirm("确定要删除此任务吗？")) {
      await deleteTask({ id: task._id });
    }
  };

  return (
    <div
      className="bg-white rounded-lg border shadow-sm hover:shadow-md transition-shadow cursor-pointer"
      onClick={() => setIsExpanded(!isExpanded)}
    >
      <div className="p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="flex-1 min-w-0">
            <h4 className="font-medium text-gray-900 truncate">{task.title}</h4>
            {task.description && (
              <p className="text-sm text-gray-500 mt-1 line-clamp-2">
                {task.description}
              </p>
            )}
          </div>
          <div className="relative">
            <button
              onClick={(e) => {
                e.stopPropagation();
                setShowActions(!showActions);
              }}
              className="p-1 hover:bg-gray-100 rounded"
            >
              <MoreVertical className="w-4 h-4 text-gray-400" />
            </button>
            {showActions && (
              <div
                className="absolute right-0 top-6 bg-white border rounded-lg shadow-lg py-1 z-10 min-w-[120px]"
                onClick={(e) => e.stopPropagation()}
              >
                {task.status !== "running" && (
                  <button
                    onClick={() => handleStatusChange("running")}
                    className="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-2"
                  >
                    <Play className="w-4 h-4 text-blue-500" />
                    开始
                  </button>
                )}
                {task.status !== "completed" && (
                  <button
                    onClick={() => handleStatusChange("completed")}
                    className="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-2"
                  >
                    <Check className="w-4 h-4 text-green-500" />
                    完成
                  </button>
                )}
                {task.status !== "failed" && task.status !== "pending" && (
                  <button
                    onClick={() => handleStatusChange("failed")}
                    className="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 flex items-center gap-2"
                  >
                    <X className="w-4 h-4 text-red-500" />
                    失败
                  </button>
                )}
                <hr className="my-1" />
                <button
                  onClick={handleDelete}
                  className="w-full px-3 py-2 text-left text-sm hover:bg-red-50 text-red-600 flex items-center gap-2"
                >
                  <Trash2 className="w-4 h-4" />
                  删除
                </button>
              </div>
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 mt-2">
          <TaskStatusBadge status={task.status} />
          <TaskAssigneeBadge assignee={task.assignee} />
          {task.priority && <TaskPriorityBadge priority={task.priority} />}
        </div>

        {isExpanded && (
          <div className="mt-3 pt-3 border-t text-sm text-gray-500 space-y-2">
            <div className="flex items-center gap-2">
              <Clock className="w-4 h-4" />
              <span>创建: {formatTime(task.createdAt)}</span>
            </div>
            {task.startedAt && (
              <div className="flex items-center gap-2">
                <Play className="w-4 h-4" />
                <span>开始: {formatTime(task.startedAt)}</span>
              </div>
            )}
            {task.completedAt && (
              <div className="flex items-center gap-2">
                <Check className="w-4 h-4" />
                <span>完成: {formatTime(task.completedAt)}</span>
              </div>
            )}
            {task.tags && task.tags.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {task.tags.map((tag, i) => (
                  <span
                    key={i}
                    className="px-2 py-0.5 bg-gray-100 rounded text-xs"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
            {task.result && (
              <div className="mt-2 p-2 bg-green-50 rounded text-green-800">
                <strong>结果:</strong> {task.result}
              </div>
            )}
            {task.error && (
              <div className="mt-2 p-2 bg-red-50 rounded text-red-800">
                <strong>错误:</strong> {task.error}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
