import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { AlertCircle, CheckCircle2, Info, X } from "lucide-react";
import { Toast } from "radix-ui";
import { cn } from "@/lib/utils";

export type ToastTone = "default" | "success" | "error";

interface ToastInput {
  title: string;
  description?: string;
  tone?: ToastTone;
}

interface ToastItem extends ToastInput {
  id: number;
}

interface ToastContextValue {
  toast: (input: ToastInput) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let nextToastId = 0;

const toneStyles: Record<ToastTone, { icon: typeof Info; iconClass: string; border: string }> = {
  default: { icon: Info, iconClass: "bg-info/10 text-info", border: "border-border" },
  success: { icon: CheckCircle2, iconClass: "bg-success/10 text-success", border: "border-success/30" },
  error: { icon: AlertCircle, iconClass: "bg-destructive/10 text-destructive", border: "border-destructive/30" },
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const toast = useCallback((input: ToastInput) => {
    nextToastId += 1;
    const item = { ...input, id: nextToastId };
    setItems((current) => [...current.slice(-3), item]);
  }, []);

  const remove = useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      <Toast.Provider duration={4200} swipeDirection="right">
        {children}
        {items.map((item) => {
          const tone = toneStyles[item.tone ?? "default"];
          const Icon = tone.icon;
          return (
            <Toast.Root
              key={item.id}
              defaultOpen
              onOpenChange={(open) => {
                if (!open) remove(item.id);
              }}
              className={cn(
                "flex items-start gap-3 rounded-xl border bg-popover p-3.5 text-popover-foreground shadow-lg",
                "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
                "data-[state=open]:slide-in-from-bottom-2 data-[state=closed]:slide-out-to-right-full",
                "data-[swipe=end]:animate-out data-[swipe=move]:translate-x-[var(--radix-toast-swipe-move-x)] data-[swipe=move]:transition-none",
                tone.border,
              )}
            >
              <span
                className={cn(
                  "mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg",
                  tone.iconClass,
                )}
              >
                <Icon className="size-4" aria-hidden="true" />
              </span>
              <div className="min-w-0 flex-1">
                <Toast.Title className="text-sm font-medium leading-snug">{item.title}</Toast.Title>
                {item.description ? (
                  <Toast.Description className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
                    {item.description}
                  </Toast.Description>
                ) : null}
              </div>
              <Toast.Close
                aria-label="Fechar notificação"
                className="flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              >
                <X className="size-3.5" aria-hidden="true" />
              </Toast.Close>
            </Toast.Root>
          );
        })}
        <Toast.Viewport className="fixed bottom-0 right-0 z-[100] m-0 flex max-h-svh w-full max-w-sm list-none flex-col gap-2 p-4 outline-none" />
      </Toast.Provider>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error("useToast deve ser usado dentro de ToastProvider");
  }
  return context;
}
