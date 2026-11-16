import React from "react";

export function Badge({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${className}`}>
      {children}
    </span>
  );
}

const STATUS: Record<string, { bg: string; color: string; border: string }> = {
  green:     { bg: "#d1fae5", color: "#065f46", border: "#a7f3d0" },
  yellow:    { bg: "#fef3c7", color: "#78350f", border: "#fde68a" },
  red:       { bg: "#fee2e2", color: "#991b1b", border: "#fca5a5" },
  blue:      { bg: "#dbeafe", color: "#1e3a8a", border: "#93c5fd" },
  secondary: { bg: "#f4f4f5", color: "#52525b", border: "#e4e4e7" },
};

export function StatusBadge({ variant = "secondary", children, className = "" }: {
  variant?: string; children: React.ReactNode; className?: string;
}) {
  const s = STATUS[variant] ?? STATUS.secondary;
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium ${className}`}
      style={{ background: s.bg, color: s.color, border: `1px solid ${s.border}` }}
    >
      {children}
    </span>
  );
}
