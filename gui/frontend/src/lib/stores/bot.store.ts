import { writable, derived } from 'svelte/store';

export type BotStatus = 'disconnected' | 'connecting' | 'awaiting-qr' | 'connected' | 'error';

export interface LogEntry {
  id: number;
  timestamp: Date;
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
  message: string;
}

export interface Message {
  id: number;
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

const MAX_LOGS = 100;
const MAX_MESSAGES = 50;
let logIdCounter = 0;
let messageIdCounter = 0;

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
      update((s) => {
        const newUsers = new Set(s.allowedUsers);
        newUsers.add(user);
        return { ...s, allowedUsers: newUsers };
      }),
    setAllowedUsers: (users: string[]) =>
      update((s) => ({
        ...s,
        allowedUsers: new Set(users)
      })),
    addLog: (log: Omit<LogEntry, 'id' | 'timestamp'>) =>
      update((s) => {
        const newLog: LogEntry = {
          ...log,
          id: logIdCounter++,
          timestamp: new Date()
        };

        const newLogs =
          s.logs.length >= MAX_LOGS ? [...s.logs.slice(1), newLog] : [...s.logs, newLog];

        return { ...s, logs: newLogs };
      }),
    addMessage: (msg: Omit<Message, 'id' | 'timestamp'>) =>
      update((s) => {
        const newMessage: Message = {
          ...msg,
          id: messageIdCounter++,
          timestamp: new Date()
        };

        const newMessages =
          s.messages.length >= MAX_MESSAGES
            ? [...s.messages.slice(1), newMessage]
            : [...s.messages, newMessage];

        return { ...s, messages: newMessages };
      }),
    reset: () => {
      logIdCounter = 0;
      messageIdCounter = 0;
      set({
        status: 'disconnected',
        qrCode: '',
        error: '',
        allowedUsers: new Set(),
        logs: [],
        messages: []
      });
    }
  };
}

export const botStore = createBotStore();
export const isConnected = derived(botStore, ($store) => $store.status === 'connected');
