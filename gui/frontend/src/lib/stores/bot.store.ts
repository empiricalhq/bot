import { writable, derived } from 'svelte/store';

export type BotStatus = 'disconnected' | 'connecting' | 'awaiting-qr' | 'connected' | 'error';

export interface LogEntry {
  id: string;
  timestamp: Date;
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
  message: string;
}

export interface Message {
  id: string;
  timestamp: Date;
  direction: 'incoming' | 'outgoing';
  userId: string;
  userName: string;
  text: string;
}

interface BotState {
  status: BotStatus;
  qrCode: string;
  error: string;
  allowedUsers: Set<string>;
  logs: LogEntry[];
  messages: Message[];
}

function createBotStore() {
  const { subscribe, set, update } = writable<BotState>({
    status: 'disconnected',
    qrCode: '',
    error: '',
    allowedUsers: new Set(),
    logs: [],
    messages: []
  });

  return {
    subscribe,
    setStatus: (status: BotStatus) => update((s) => ({ ...s, status })),
    setQRCode: (qrCode: string) => update((s) => ({ ...s, qrCode, status: 'awaiting-qr' })),
    clearQRCode: () => update((s) => ({ ...s, qrCode: '' })),
    setError: (error: string) => update((s) => ({ ...s, error, status: 'error' })),
    addAllowedUser: (user: string) =>
      update((s) => ({
        ...s,
        allowedUsers: new Set([...s.allowedUsers, user])
      })),
    setAllowedUsers: (users: string[]) =>
      update((s) => ({
        ...s,
        allowedUsers: new Set(users)
      })),
    addLog: (log: Omit<LogEntry, 'id' | 'timestamp'>) =>
      update((s) => ({
        ...s,
        logs: [
          ...s.logs,
          {
            ...log,
            id: crypto.randomUUID(),
            timestamp: new Date()
          }
        ].slice(-100) // Keep last 100 logs
      })),
    addMessage: (msg: Omit<Message, 'id' | 'timestamp'>) =>
      update((s) => ({
        ...s,
        messages: [
          ...s.messages,
          {
            ...msg,
            id: crypto.randomUUID(),
            timestamp: new Date()
          }
        ].slice(-50) // Keep last 50 messages
      })),
    reset: () =>
      set({
        status: 'disconnected',
        qrCode: '',
        error: '',
        allowedUsers: new Set(),
        logs: [],
        messages: []
      })
  };
}

export const botStore = createBotStore();

export const botStatus = derived(botStore, ($store) => $store.status);
export const isConnected = derived(botStore, ($store) => $store.status === 'connected');
export const hasQRCode = derived(botStore, ($store) => !!$store.qrCode);
