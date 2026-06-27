export function formatCompactNumber(value) {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number < 1000) return String(Math.max(0, Math.trunc(number || 0)));
  const units = [
    [1000000000, "B"],
    [1000000, "M"],
    [1000, "K"],
  ];
  const [divisor, suffix] = units.find(([base]) => number >= base);
  return `${(number / divisor).toFixed(1).replace(/\.0$/, "")}${suffix}`;
}

export function formatToken(value) {
  return formatCompactNumber(value);
}

export function date(value) {
  return value ? new Date(value).toLocaleString() : "";
}

export function percent(value, fallback = "-") {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number <= 0) return fallback;
  return `${(number * 100).toFixed(1)}%`;
}
