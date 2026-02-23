"use client";

import { Doc } from "@/convex/_generated/dataModel";
import { TaskCard } from "./TaskCard";

type Status = "pending" | "running" | "completed" | "failed";

interface TaskColumnProps {
  title: string;
  status: Status;
  tasks: Doc<"tasks">[];
  color: string;
}

const statusEmoji: Record<Status, string> = {
  pending: "📋",
  running: "🔄",
  completed: "✅",
  failed: "❌",
};

export function TaskColumn({ title, status, tasks, color }: TaskColumnProps) {
  return (
    <div className="flex-1 min-w-[280px] max-w-[400px]">
      <div
        className={`flex items-center gap-2 px-3 py-2 rounded-t-lg ${color}`}
      >
        <span>{statusEmoji[status]}</span>
        <h3 className="font-semibold text-gray-800">{title}</h3>
        <span className="ml-auto bg-white/50 px-2 py-0.5 rounded-full text-sm font-medium">
          {tasks.length}
        </span>
      </div>
      <div className="bg-gray-100 rounded-b-lg p-2 min-h-[200px] space-y-2">
        {tasks.length === 0 ? (
          <div className="text-center text-gray-400 py-8 text-sm">
            暂无任务
          </div>
        ) : (
          tasks.map((task) => <TaskCard key={task._id} task={task} />)
        )}
      </div>
    </div>
  );
}
