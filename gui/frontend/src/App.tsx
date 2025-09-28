import { useEffect, useState, useRef } from 'react';
import QRCode from 'react-qr-code';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { GetAllowedUsers, AddAllowedUser, GetEnv } from '../wailsjs/go/gui/App';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useToast } from "@/components/ui/use-toast"
import { Toaster } from "@/components/ui/toaster"

interface ChatMessage {
  sender: string;
  message: string;
  timestamp: string;
}

function App() {
  const [qrCode, setQrCode] = useState('');
  const [showQr, setShowQr] = useState(true);
  const [status, setStatus] = useState('Initializing...');
  const [logs, setLogs] = useState<string[]>([]);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [allowedUsers, setAllowedUsers] = useState<string[]>([]);
  const [newUser, setNewUser] = useState('');
  const [env, setEnv] = useState('');
  const { toast } = useToast();
  const logEndRef = useRef<HTMLDivElement>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);


  useEffect(() => {
    // --- Event Listeners ---
    EventsOn("qr:update", (code) => {
      setQrCode(code);
      setShowQr(true);
    });
    EventsOn("qr:hide", () => setShowQr(false));
    EventsOn("status:update", setStatus);
    EventsOn("log:new", (logLine) => setLogs(prev => [...prev, logLine]));
    EventsOn("chat:new", (msg) => {
      const timestamp = new Date().toLocaleTimeString();
      setChatMessages(prev => [...prev, { ...msg, timestamp }]);
    });
    EventsOn("show:toast", (message) => {
        toast({ title: "Bot Notification", description: message });
    });


    // --- Initial Data Fetch ---
    fetchAllowedUsers();
    GetEnv().then(setEnv);
  }, []);

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [chatMessages]);


  const fetchAllowedUsers = () => {
    GetAllowedUsers().then(setAllowedUsers);
  };

  const handleAddUser = async () => {
    try {
      await AddAllowedUser(newUser);
      setNewUser('');
      fetchAllowedUsers();
    } catch (error) {
      console.error(error);
      toast({ title: "Error", description: String(error), variant: "destructive" });
    }
  };

  return (
    <>
      <main className="container mx-auto p-4 grid grid-cols-1 lg:grid-cols-3 gap-4 h-screen">
        {/* Left Column: Status & QR */}
        <div className="flex flex-col gap-4">
          <Card>
            <CardHeader><CardTitle>Status</CardTitle></CardHeader>
            <CardContent>
              <p className="text-lg font-semibold">{status}</p>
            </CardContent>
          </Card>
          {showQr && qrCode && (
            <Card>
              <CardHeader><CardTitle>Scan to Login</CardTitle></CardHeader>
              <CardContent className="flex justify-center p-6 bg-white">
                <QRCode value={qrCode} />
              </CardContent>
            </Card>
          )}
        </div>

        {/* Middle Column: Chat Feed */}
        <Card className="flex flex-col">
          <CardHeader><CardTitle>Live Chat Feed</CardTitle></CardHeader>
          <CardContent className="flex-grow overflow-hidden">
            <ScrollArea className="h-[calc(100vh-12rem)] pr-4">
              {chatMessages.map((msg, index) => (
                <div key={index} className="mb-2 p-2 border rounded-md">
                  <p className="font-bold text-sm">{msg.sender}</p>
                  <p className="text-sm">{msg.message}</p>
                  <p className="text-xs text-muted-foreground text-right">{msg.timestamp}</p>
                </div>
              ))}
              <div ref={chatEndRef} />
            </ScrollArea>
          </CardContent>
        </Card>

        {/* Right Column: Controls & Logs */}
        <div className="flex flex-col gap-4">
          {env === 'dev' && (
             <Card>
                <CardHeader><CardTitle>Dev Allowed Users</CardTitle></CardHeader>
                <CardContent>
                    <ScrollArea className="h-32 mb-4 border rounded-md p-2">
                        {allowedUsers.length > 0 ? allowedUsers.map(u => <p key={u}>{u}</p>) : <p>No users specified.</p>}
                    </ScrollArea>
                    <div className="flex w-full max-w-sm items-center space-x-2">
                        <Input type="text" placeholder="Phone number" value={newUser} onChange={(e) => setNewUser(e.target.value)} />
                        <Button onClick={handleAddUser}>Add</Button>
                    </div>
                    <p className="text-xs text-muted-foreground mt-2">Add numbers like `51987654321`. Restart required.</p>
                </CardContent>
             </Card>
          )}
          <Card className="flex flex-col flex-grow">
            <CardHeader><CardTitle>Simplified Logs</CardTitle></CardHeader>
            <CardContent className="flex-grow overflow-hidden">
                <ScrollArea className="h-[calc(100vh-28rem)]">
                    <pre className="text-xs whitespace-pre-wrap">
                    {logs.join('')}
                    </pre>
                    <div ref={logEndRef} />
                </ScrollArea>
            </CardContent>
          </Card>
        </div>
      </main>
      <Toaster />
    </>
  );
}

export default App;
