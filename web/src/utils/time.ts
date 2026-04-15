/**
 * Format an ISO 8601 timestamp for compact display (node labels).
 * Output: "MM-DD HH:MM" (no year, no seconds)
 * Example: "04-15 15:29"
 */
export function formatTimeShort(iso: string | undefined | null): string {
  if (!iso) return ''
  // Extract from ISO format: "2026-04-15T15:29:55.103Z"
  const m = iso.match(/^\d{4}-(\d{2}-\d{2})T(\d{2}:\d{2})/)
  if (!m) return iso.slice(0, 10)
  return `${m[1]} ${m[2]}`
}

/**
 * Format an ISO 8601 timestamp for detail display (side panel).
 * Output: "YYYY-MM-DD HH:MM:SS"
 * Example: "2026-04-15 15:29:55"
 */
export function formatTimeFull(iso: string | undefined | null): string {
  if (!iso) return ''
  return iso.slice(0, 19).replace('T', ' ')
}
