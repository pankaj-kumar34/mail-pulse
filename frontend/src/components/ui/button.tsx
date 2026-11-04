import React from "react";

type Variant = "default" | "outline" | "ghost" | "secondary" | "destructive" | "link";
type Size    = "default" | "sm" | "lg" | "icon" | "icon-sm";

const VARIANT: Record<Variant, string> = {
  default:     "bg-[var(--accent)] text-white hover:bg-[var(--accent-hover)] border-transparent",
  outline:     "bg-white text-[var(--content-text)] border-[var(--content-border)] hover:bg-zinc-50",
  ghost:       "bg-transparent text-[var(--content-text-sub)] hover:bg-zinc-100 border-transparent",
  secondary:   "bg-zinc-100 text-zinc-700 hover:bg-zinc-200 border-transparent",
  destructive: "bg-red-50 text-red-600 hover:bg-red-100 border-red-200",
  link:        "bg-transparent text-[var(--accent)] hover:underline border-transparent p-0 h-auto",
};

const SIZE: Record<Size, string> = {
  default:  "h-9 px-3.5 text-sm gap-1.5",
  sm:       "h-8 px-3 text-sm gap-1",
  lg:       "h-10 px-4 text-sm gap-2",
  icon:     "h-9 w-9",
  "icon-sm":"h-8 w-8",
};

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

export function Button({ variant = "default", size = "default", className = "", children, ...props }: ButtonProps) {
  return (
    <button
      className={`inline-flex items-center justify-center rounded-lg border font-medium transition-colors cursor-pointer disabled:opacity-50 disabled:pointer-events-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]/40 ${VARIANT[variant]} ${SIZE[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}

export { Button as default };
