import "./index.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import ActivityTable from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ActivityTable />
  </StrictMode>,
);
