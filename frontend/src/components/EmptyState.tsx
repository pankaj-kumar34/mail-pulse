import { Mail, Inbox, SearchX, CheckCircle2 } from "lucide-react";

type IconName = "mail" | "inbox" | "search" | "check";

const ICONS: Record<IconName, React.FC<{ size: number; strokeWidth: number; className: string }>> = {
  mail:   (p) => <Mail   {...p} />,
  inbox:  (p) => <Inbox  {...p} />,
  search: (p) => <SearchX {...p} />,
  check:  (p) => <CheckCircle2 {...p} />,
};

interface Props {
  icon?: IconName;
  title: string;
  subtitle?: string;
  compact?: boolean;
}

export function EmptyState({ icon = "mail", title, subtitle, compact = false }: Props) {
  const Icon = ICONS[icon];
  return (
    <div className={`flex flex-col items-center justify-center text-center w-full ${compact ? "py-6 gap-2" : "py-10 gap-3"}`}>
      <div className={`rounded-xl border-2 border-zinc-200 flex items-center justify-center ${compact ? "w-10 h-10" : "w-14 h-14"}`}>
        <Icon size={compact ? 18 : 24} strokeWidth={1.5} className="text-zinc-300" />
      </div>
      <div>
        <p className={`font-medium text-zinc-400 ${compact ? "text-xs" : "text-sm"}`}>{title}</p>
        {subtitle && <p className={`text-zinc-300 mt-0.5 ${compact ? "text-xs" : "text-xs"}`}>{subtitle}</p>}
      </div>
    </div>
  );
}
