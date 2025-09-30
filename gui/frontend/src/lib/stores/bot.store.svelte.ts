import { SvelteSet } from 'svelte/reactivity';

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

const MAX_LOGS = 100;
const MAX_MESSAGES = 50;

function createBotStore() {
  let status = $state<BotStatus>('disconnected');
  let qrCode = $state('');
  let error = $state('');
  let allowedUsers = $state(new Set<string>());
  let logs = $state<LogEntry[]>([]);
  let messages = $state<Message[]>([]);
  let logIdCounter = 0;
  let messageIdCounter = 0;

  return {
    get status() {
      return status;
    },
    set status(s: BotStatus) {
      status = s;
    },

    get qrCode() {
      return qrCode;
    },
    set qrCode(qr: string) {
      qrCode = qr;
      if (qr) {
        status = 'awaiting-qr';
      }
    },

    get error() {
      return error;
    },
    set error(e: string) {
      error = e;
    },

    get allowedUsers() {
      return Array.from(allowedUsers);
    },
    get messages() {
      return messages;
    },
    get logs() {
      return logs;
    },

    setAllowedUsers(users: string[]) {
      allowedUsers = new SvelteSet(users);
    },

    addAllowedUser(user: string) {
      allowedUsers.add(user);
    },

    addLog(log: Omit<LogEntry, 'id' | 'timestamp'>) {
      const newLog: LogEntry = { ...log, id: logIdCounter++, timestamp: new Date() };
      logs.push(newLog);
      if (logs.length > MAX_LOGS) {
        logs.shift();
      }
    },

    addMessage(msg: Omit<Message, 'id' | 'timestamp'>) {
      const newMessage: Message = { ...msg, id: messageIdCounter++, timestamp: new Date() };
      messages.push(newMessage);
      if (messages.length > MAX_MESSAGES) {
        messages.shift();
      }
    },

    reset() {
      logIdCounter = 0;
      messageIdCounter = 0;
      status = 'disconnected';
      qrCode = '';
      error = '';
      allowedUsers = new SvelteSet();
      logs = [];
      messages = [];
    }
  };
}

export const botStore = createBotStore();
