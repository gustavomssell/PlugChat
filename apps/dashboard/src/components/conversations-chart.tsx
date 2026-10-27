import type { DayCount } from "@chatfree/shared";
import { dayLabel, weekdayLabel } from "@/lib/labels";
import { cn } from "@/lib/utils";

export function ConversationsChart({ data }: { data: DayCount[] }) {
  const max = Math.max(1, ...data.map((item) => item.count));
  const total = data.reduce((sum, item) => sum + item.count, 0);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex h-52 flex-col">
        <div className="relative flex-1">
          <div aria-hidden="true" className="absolute inset-0 flex flex-col justify-between">
            {[0, 1, 2, 3].map((line) => (
              <span key={line} className="block border-t border-dashed border-border/60" />
            ))}
          </div>

          <div
            className="absolute inset-0 flex items-end gap-1.5 px-1 sm:gap-2.5"
          >
            <p className="sr-only">
              Conversas nos últimos 7 dias. Total de {total}.
            </p>
            {data.map((day, index) => {
              const percentage = day.count === 0 ? 0 : Math.max(6, Math.round((day.count / max) * 100));
              return (
                <div
                  key={day.date}
                  role="group"
                  className="group relative flex h-full flex-1 flex-col items-center justify-end gap-1"
                  tabIndex={0}
                  aria-label={`${dayLabel(day.date)}: ${day.count} ${day.count === 1 ? "conversa" : "conversas"}`}
                >
                  <span
                    className={cn(
                      "text-[10px] font-medium tabular-nums transition-opacity",
                      day.count === 0 ? "text-muted-foreground/50" : "text-muted-foreground",
                    )}
                  >
                    {day.count}
                  </span>
                  <div className="relative flex w-full flex-1 items-end justify-center">
                    <div
                      className={cn(
                        "w-full max-w-9 rounded-t-md transition-colors duration-fast group-hover:from-primary/80 group-hover:to-primary group-focus-visible:from-primary/80 group-focus-visible:to-primary",
                        day.count === 0
                          ? "h-1 rounded-full bg-border"
                          : "animate-bar-rise bg-gradient-to-t from-primary/55 to-primary",
                      )}
                      style={
                        day.count === 0
                          ? undefined
                          : {
                              height: `${percentage}%`,
                              animationDelay: `${index * 45}ms`,
                            }
                      }
                    />
                  </div>
                  <span className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 hidden -translate-x-1/2 whitespace-nowrap rounded-md border bg-popover px-2 py-1 text-[10px] font-medium text-popover-foreground shadow-md group-hover:block group-focus-visible:block">
                    {day.count} novas sessões
                  </span>
                </div>
              );
            })}
          </div>
        </div>

        <div className="mt-2 flex gap-1.5 px-1 sm:gap-2.5">
          {data.map((day) => (
            <div key={day.date} className="flex flex-1 flex-col items-center gap-0.5">
              <span className="text-[10px] font-medium capitalize text-muted-foreground">
                {weekdayLabel(day.date)}
              </span>
              <span className="text-[10px] tabular-nums text-muted-foreground/70">
                {dayLabel(day.date)}
              </span>
            </div>
          ))}
        </div>
      </div>

      <table className="sr-only">
        <caption>Conversas iniciadas nos últimos 7 dias</caption>
        <thead>
          <tr>
            <th scope="col">Dia</th>
            <th scope="col">Conversas</th>
          </tr>
        </thead>
        <tbody>
          {data.map((day) => (
            <tr key={day.date}>
              <td>{dayLabel(day.date)}</td>
              <td>{day.count}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
