import { EventsOn } from '$lib/wailsjs/runtime/runtime.js';
import { StartBot, GetAllowedUsers, AddAllowedUser } from '$lib/wailsjs/go/main/App.js';
import { botStore } from '$lib/stores/bot.store.svelte';

let isInitialized = false;

function setupEventListeners() {
  EventsOn('bot:qr_code', (qrCode: string) => {
    botStore.qrCode = qrCode;
  });

  EventsOn('bot:new_log', (log: { level: string; message: string }) => {
    const level = log.level.toUpperCase() as 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
    botStore.addLog({ level, message: log.message });

    if (
      log.message.includes('QR login successful') ||
      log.message.includes('Connection successful')
    ) {
      botStore.qrCode = '';
      botStore.status = 'connected';
    }
  });

  EventsOn(
    'bot:new_message',
    (message: { direction: string; userID: string; userName: string; text: string }) => {
      botStore.addMessage({
        direction: message.direction as 'incoming' | 'outgoing',
        userId: message.userID,
        userName: message.userName,
        text: message.text
      });
    }
  );

  EventsOn('bot:start_failed', (error: string) => {
    botStore.error = error;
    botStore.status = 'error';
  });
}

export async function initialize() {
  if (isInitialized) return;
  isInitialized = true;

  setupEventListeners();

  botStore.status = 'connecting';

  try {
    const users = await GetAllowedUsers();
    botStore.setAllowedUsers(users);
    await StartBot();
  } catch (error) {
    console.error('Failed to initialize bot:', error);
    const errorMessage = error instanceof Error ? error.message : 'Failed to initialize bot';
    botStore.error = errorMessage;
    botStore.status = 'error';
  }
}

export async function addAllowedUser(user: string) {
  const trimmedUser = user.trim();
  if (!trimmedUser) return;

  const formattedUser = trimmedUser.includes('@') ? trimmedUser : `${trimmedUser}@s.whatsapp.net`;
  await AddAllowedUser(formattedUser);
  botStore.addAllowedUser(formattedUser);
}
