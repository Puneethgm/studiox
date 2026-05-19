// Color helpers for per-studio theming.
//
// withAlpha appends a 2-digit hex alpha to a `#RRGGBB` color, producing the
// `#RRGGBBAA` form that all modern browsers support. We use it to derive soft
// tints (e.g. backgrounds, hover states) from a studio's brand color without
// pulling in a color library.

export function withAlpha(hex: string, alpha: number): string {
  const a = Math.max(0, Math.min(1, alpha));
  const hh = Math.round(a * 255).toString(16).padStart(2, '0');
  if (!/^#[0-9a-f]{6}$/i.test(hex)) return hex;
  return hex + hh;
}

export function brandInitials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 0) return '··';
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase();
  return (parts[0]![0]! + parts[1]![0]!).toUpperCase();
}
