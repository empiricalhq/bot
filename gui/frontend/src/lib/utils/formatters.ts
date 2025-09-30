import type { LogEntry } from '$lib/stores/bot.store.svelte';

export function getLogColor(level: LogEntry['level']): string {
  switch (level) {
    case 'ERROR':
      return 'text-red-500';
    case 'WARN':
      return 'text-yellow-500';
    case 'INFO':
      return 'text-blue-500';
    default:
      return 'text-slate-500';
  }
}

export function formatTime(date: Date): string {
  return date.toLocaleTimeString('en-US', { hour12: false });
}
