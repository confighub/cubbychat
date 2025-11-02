import React, { useState, useEffect, useRef } from "react";
import { Button, TextInput, ScrollArea, Paper, Text } from "@mantine/core";
import ReactMarkdown from "react-markdown";
import ModelStatus from "../ModelStatus/ModelStatus";

// Use relative URLs - Vite proxy handles routing to backend in dev, nginx in production
const WS_URL = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/api/ws`;
const HISTORY_URL = "/api/history";
const CONFIG_URL = "/api/config";

const Chat: React.FC = () => {
  const [messages, setMessages] = useState<{ sender: string; text: string }[]>([
    { sender: "AI", text: "Hello! I'm Cubby 🧸, your friendly chat assistant. How can I help you today?" }
  ]);
  const [input, setInput] = useState("");
  const ws = useRef<WebSocket | null>(null);
  const isConnecting = useRef(false);
  const [title, setTitle] = useState("🧸 Cubby Chat"); // Default title with mascot
  const [config, setConfig] = useState<{
    model: string;
    version: string;
    region: string;
    role: string;
  }>({
    model: "unknown",
    version: "unknown",
    region: "unknown",
    role: "unknown"
  });

  // Ref for scrolling to bottom
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  
  // Fetch config from backend
  useEffect(() => {
    fetch(CONFIG_URL)
      .then((res) => res.json())
      .then((data) => {
        setTitle("🧸 " + (data.title || "Cubby Chat"));
        setConfig({
          model: data.model || "unknown",
          version: data.version || "unknown",
          region: data.region || "unknown",
          role: data.role || "unknown"
        });
      })
      .catch((err) => console.error("❌ Failed to fetch config:", err));
  }, []);

  useEffect(() => {
    // Prevent duplicate connections in React Strict Mode
    if (isConnecting.current) return;
    isConnecting.current = true;

    console.log("Connecting to WebSocket:", WS_URL);
    ws.current = new WebSocket(WS_URL);

    ws.current.onopen = () => {
      console.log("✅ WebSocket connection opened");
    };

    ws.current.onmessage = (event) => {
      console.log("📩 Streaming token received:", event.data);

      setMessages((prevMessages) => {
        let lastMessage = prevMessages[prevMessages.length - 1];

        if (lastMessage?.sender === "AI") {
          lastMessage.text += event.data;
          return [...prevMessages.slice(0, -1), lastMessage];
        } else {
          return [...prevMessages, { sender: "AI", text: event.data }];
        }
      });
    };

    ws.current.onerror = (error) => {
      console.error("❌ WebSocket error:", error);
    };

    ws.current.onclose = (event) => {
      console.warn("⚠️ WebSocket closed:", event.code, event.reason);
      isConnecting.current = false;
    };

    return () => {
      console.log("🔌 Closing WebSocket connection");
      ws.current?.close();
      isConnecting.current = false;
    };
  }, []);

  const sendMessage = () => {
    if (input.trim() && ws.current) {
      console.log("📤 Sending message:", input);
      setMessages((prev) => [...prev, { sender: "You", text: input }, { sender: "AI", text: "" }]);
      ws.current.send(input);
      setInput("");
    }
  };

  const loadChatHistory = async () => {
    try {
      const response = await fetch(HISTORY_URL);
      if (!response.ok) throw new Error("Failed to fetch chat history");

      const history = await response.json();
      setMessages(history.map((msg: any) => ({ sender: msg.sender, text: msg.message })));
      console.log("✅ Chat history loaded");
    } catch (error) {
      console.error("❌ Error loading chat history:", error);
    }
  };


  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  return (
    <Paper shadow="xs" p="md" style={{ maxWidth: 600, margin: "auto", marginTop: 50 }}>
      <h1>{title}</h1>
      <div style={{ marginBottom: "1rem" }}>
        <Text size="sm" c="dimmed">
          Region: {config.region} | Role: {config.role}
        </Text>
        
        {/* Model Status Indicator */}
        <ModelStatus />
        
        <div style={{
          display: "inline-block",
          padding: "4px 12px",
          borderRadius: "20px",
          background: "linear-gradient(90deg, #ec4899, #f43f5e)",
          color: "white",
          fontSize: "12px",
          fontWeight: "bold",
          marginTop: "8px"
        }}>
          {config.version} 🎉
        </div>
      </div>
      <Button onClick={loadChatHistory} mb="md" fullWidth>
        Load Chat History
      </Button>

      <ScrollArea style={{ height: 400, border: "1px solid #ccc", padding: 10 }}>
        {messages.map((msg, index) => (
          <div key={index} style={{ marginBottom: "0.5rem" }}>
            <Text
              fw={700}
              style={{
                background: msg.sender === "You"
                  ? "linear-gradient(90deg, #3b82f6, #2563eb)"
                  : "linear-gradient(90deg, #a855f7, #9333ea)",
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
                backgroundClip: "text",
                display: "inline-block"
              }}
            >
              {msg.sender}:
            </Text>
            <div className="chat-message">
              <ReactMarkdown>{msg.text}</ReactMarkdown>
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </ScrollArea>

      <TextInput
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="Type a message..."
        onKeyPress={(e) => e.key === "Enter" && sendMessage()}
        mt="md"
      />
      <Button onClick={sendMessage} mt="md" fullWidth className="send-button">
        Send 🚀
      </Button>
    </Paper>
  );
};

export default Chat;