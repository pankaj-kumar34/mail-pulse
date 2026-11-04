import React from "react";
import { ChevronDown } from "lucide-react";

export function NativeSelect({
  className = "",
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <div className="relative inline-flex items-center">
      <select
        className={`h-9 appearance-none rounded-lg border border-[color:var(--input-border)] bg-[color:var(--input-bg)] pl-3 pr-8 text-sm text-[color:var(--content-text)] outline-none focus:border-[color:var(--accent)] focus:ring-2 focus:ring-[color:var(--accent)]/20 cursor-pointer ${className}`}
        {...props}
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-400" />
    </div>
  );
}
