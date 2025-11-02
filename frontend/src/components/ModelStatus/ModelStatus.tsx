import React, { useState, useEffect } from "react";
import { Badge, Progress, Group, Loader } from "@mantine/core";

interface ModelStatusData {
  ready: boolean;
  status: string;
  model: string;
}

const ModelStatus: React.FC = () => {
  const [status, setStatus] = useState<ModelStatusData>({
    ready: false,
    status: "initializing",
    model: "unknown"
  });

  // Poll for model status updates
  useEffect(() => {
    const pollStatus = async () => {
      try {
        const response = await fetch("/api/model-status");
        const data = await response.json();
        setStatus(data);
      } catch (error) {
        console.error("Failed to fetch model status:", error);
      }
    };

    // Poll immediately and then every 2 seconds
    pollStatus();
    const interval = setInterval(pollStatus, 2000);

    return () => clearInterval(interval);
  }, []);

  const getStatusInfo = () => {
    switch (status.status) {
      case "initializing":
        return {
          color: "blue",
          text: "🚀 Initializing...",
          progress: 10,
          icon: <Loader size="xs" />
        };
      case "checking_models":
        return {
          color: "blue",
          text: "🔍 Checking available models...",
          progress: 20,
          icon: <Loader size="xs" />
        };
      case "model_found":
        return {
          color: "yellow",
          text: `📥 Found model: ${status.model}`,
          progress: 40,
          icon: <Loader size="xs" />
        };
      case "testing_generation":
        return {
          color: "orange",
          text: "🧪 Loading model into memory...",
          progress: 70,
          icon: <Loader size="xs" />
        };
      case "ready":
        return {
          color: "green",
          text: `✅ Ready: ${status.model}`,
          progress: 100,
          icon: null
        };
      case "retry_1":
      case "retry_2":
      case "retry_3":
      case "retry_4":
      case "retry_5":
        return {
          color: "yellow",
          text: "🔄 Retrying model loading...",
          progress: 50,
          icon: <Loader size="xs" />
        };
      case "error_connecting":
        return {
          color: "red",
          text: "❌ Connection error",
          progress: 0,
          icon: null
        };
      case "error_api":
        return {
          color: "red",
          text: "❌ API error",
          progress: 0,
          icon: null
        };
      case "error_generation":
        return {
          color: "red",
          text: "❌ Model generation failed",
          progress: 0,
          icon: null
        };
      case "no_models":
        return {
          color: "red",
          text: "❌ No models available",
          progress: 0,
          icon: null
        };
      case "failed":
        return {
          color: "red",
          text: "❌ Model loading failed",
          progress: 0,
          icon: null
        };
      default:
        return {
          color: "gray",
          text: `🔄 ${status.status}`,
          progress: 30,
          icon: <Loader size="xs" />
        };
    }
  };

  const statusInfo = getStatusInfo();

  return (
    <div style={{ marginTop: "8px" }}>
      <Group gap="xs" align="center">
        <Badge
          color={statusInfo.color}
          variant="light"
          size="sm"
          leftSection={statusInfo.icon}
        >
          {statusInfo.text}
        </Badge>
        {!status.ready && statusInfo.progress > 0 && (
          <Progress
            value={statusInfo.progress}
            size="xs"
            style={{ width: "100px" }}
            color={statusInfo.color}
            animated
          />
        )}
      </Group>
    </div>
  );
};

export default ModelStatus;