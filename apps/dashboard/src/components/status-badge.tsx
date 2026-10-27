import type { ConversationStatus } from "@chatfree/shared";
import {
  conversationStatusDot,
  conversationStatusLabel,
  conversationStatusTone,
} from "@/lib/labels";
import { cn } from "@/lib/utils";

export function StatusBadge({
  status,
  className,
  withDot = true,
}: {
  status: ConversationStatus;
  className?: string;
  withDot?: boolean;
}) {
  return (
    <span
      className={cn(
        "inline-flex w-fit shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border px-2 py-0.5 text-[11px] font-medium",
        conversationStatusTone[status],
        className,
      )}
    >
      {withDot ? (
        <span className={cn("size-1.5 rounded-full", conversationStatusDot[status])} aria-hidden="true" />
      ) : null}
      {conversationStatusLabel[status]}
    </span>
  );
}
