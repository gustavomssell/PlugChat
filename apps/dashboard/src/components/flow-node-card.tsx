import {
  Clock,
  GitBranch,
  MessageSquare,
  Play,
  Sparkles,
  Square,
  Tag,
  TextCursorInput,
  UserRoundCheck,
  Users,
  Webhook,
  type LucideIcon,
} from "lucide-react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { NODE_LABELS, type NodeType } from "@chatfree/shared";
import { cn } from "@/lib/utils";

export type FlowNodeData = {
  label: string;
  type: NodeType;
  preview?: string;
  config?: Record<string, unknown>;
};

const nodeIcons: Record<NodeType, LucideIcon> = {
  trigger: Play,
  message: MessageSquare,
  condition: GitBranch,
  collect_input: TextCursorInput,
  llm: Sparkles,
  handoff: UserRoundCheck,
  webhook: Webhook,
  tag: Tag,
  assign: Users,
  delay: Clock,
  end: Square,
};

const nodeTone: Record<NodeType, string> = {
  trigger: "bg-success/10 text-success",
  message: "bg-primary/10 text-primary",
  condition: "bg-warning/10 text-warning",
  collect_input: "bg-info/10 text-info",
  llm: "bg-violet-500/10 text-violet-600 dark:text-violet-300",
  handoff: "bg-rose-500/10 text-rose-600 dark:text-rose-300",
  webhook: "bg-cyan-500/10 text-cyan-600 dark:text-cyan-300",
  tag: "bg-teal-500/10 text-teal-600 dark:text-teal-300",
  assign: "bg-indigo-500/10 text-indigo-600 dark:text-indigo-300",
  delay: "bg-muted text-muted-foreground",
  end: "bg-muted text-muted-foreground",
};

export function FlowNodeCard({ data, selected }: NodeProps<Node<FlowNodeData>>) {
  const Icon = nodeIcons[data.type];

  return (
    <div
      className={cn(
        "w-64 rounded-xl border border-border bg-card p-3 shadow-sm transition-all duration-fast",
        selected && "border-primary/60 shadow-md ring-2 ring-primary/20",
      )}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!size-2.5 !border-2 !border-background !bg-border"
      />

      <div className="flex items-center gap-2">
        <span className={cn("flex size-7 items-center justify-center rounded-md", nodeTone[data.type])}>
          <Icon className="size-3.5" />
        </span>
        <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
          {NODE_LABELS[data.type]}
        </span>
      </div>

      <p className="mt-2 text-sm font-medium leading-snug">{data.label}</p>
      {data.preview ? (
        <p className="mt-1 line-clamp-2 text-xs leading-snug text-muted-foreground">
          {data.preview}
        </p>
      ) : null}

      {data.type === "condition" ? (
        <>
          <Handle
            id="true"
            type="source"
            position={Position.Bottom}
            style={{ left: "30%" }}
            className="!size-2.5 !border-2 !border-background !bg-emerald-500"
          />
          <Handle
            id="false"
            type="source"
            position={Position.Bottom}
            style={{ left: "70%" }}
            className="!size-2.5 !border-2 !border-background !bg-rose-500"
          />
        </>
      ) : data.type === "end" ? null : (
        <Handle
          type="source"
          position={Position.Bottom}
          className="!size-2.5 !border-2 !border-background !bg-primary"
        />
      )}
    </div>
  );
}
