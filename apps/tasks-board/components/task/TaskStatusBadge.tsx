import { cn } from "@/lib/utils";

type Status = "pending" | "running" | "completed" | "failed";
type Assignee = "user" | "ai" | "both";

interface TaskStatusBadgeProps {
  status: Status;
  className?: string;
}

const statusConfig: Record<Status, { label: string; className: string }> = {
  pending: {
    label: "待办",
    className: "bg-gray-100 text-gray-700 border-gray-200",
  },
  running: {
    label: "进行中",
    className: "bg-blue-100 text-blue-700 border-blue-200",
  },
  completed: {
    label: "已完成",
    className: "bg-green-100 text-green-700 border-green-200",
  },
  failed: {
    label: "失败",
    className: "bg-red-100 text-red-700 border-red-200",
  },
};

export function TaskStatusBadge({ status, className }: TaskStatusBadgeProps) {
  const config = statusConfig[status];
  return (
    <span
      className={cn(
        "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border",
        config.className,
        className
      )}
    >
      {config.label}
    </span>
  );
}

interface TaskAssigneeBadgeProps {
  assignee: Assignee;
  className?: string;
}

const assigneeConfig: Record<Assignee, { label: string; className: string }> = {
  user: {
    label: "用户",
    className: "bg-purple-100 text-purple-700",
  },
  ai: {
    label: "AI",
    className: "bg-cyan-100 text-cyan-700",
  },
  both: {
    label: "协作",
    className: "bg-amber-100 text-amber-700",
  },
};

export function TaskAssigneeBadge({
  assignee,
  className,
}: TaskAssigneeBadgeProps) {
  const config = assigneeConfig[assignee];
  return (
    <span
      className={cn(
        "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium",
        config.className,
        className
      )}
    >
      {config.label}
    </span>
  );
}

interface TaskPriorityBadgeProps {
  priority?: "low" | "medium" | "high";
  className?: string;
}

const priorityConfig: Record<
  string,
  { label: string; className: string } | undefined
> = {
  low: { label: "低", className: "bg-slate-100 text-slate-600" },
  medium: { label: "中", className: "bg-yellow-100 text-yellow-700" },
  high: { label: "高", className: "bg-orange-100 text-orange-700" },
};

export function TaskPriorityBadge({
  priority,
  className,
}: TaskPriorityBadgeProps) {
  if (!priority) return null;
  const config = priorityConfig[priority];
  if (!config) return null;

  return (
    <span
      className={cn(
        "inline-flex items-center px-2 py-0.5 rounded text-xs font-medium",
        config.className,
        className
      )}
    >
      {config.label}
    </span>
  );
}
