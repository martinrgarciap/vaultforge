import { BrowserRouter } from "react-router";

import { AppRoutes } from "./AppRoutes";

export function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  );
}
