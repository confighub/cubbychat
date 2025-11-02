import React from "react";
import Chat from "./components/Chat/Chat";
import { MantineProvider } from "@mantine/core";
import '@mantine/core/styles.css';

const App: React.FC = () => {
  return (
    <MantineProvider>
      <Chat />
    </MantineProvider>
  );
};

export default App;
