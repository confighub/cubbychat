import ReactDOM from "react-dom/client";
import App from "./App";

// Strict Mode disabled in development due to WebSocket connection issues
// Re-enable for production builds if needed
ReactDOM.createRoot(document.getElementById("root")!).render(
  <App />
);
