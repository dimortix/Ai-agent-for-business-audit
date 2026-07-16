import { Navigate, Route, Routes } from "react-router-dom";
import { loadUser } from "./api/client";
import Layout from "./components/Layout";
import Admin from "./pages/Admin";
import Advice from "./pages/Advice";
import Analytics from "./pages/Analytics";
import Dashboard from "./pages/Dashboard";
import Expenses from "./pages/Expenses";
import Login from "./pages/Login";

function Protected({ children }: { children: React.ReactNode }) {
  if (!loadUser()) return <Navigate to="/login" replace />;
  return <Layout>{children}</Layout>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/admin" element={<Admin />} />
      <Route path="/" element={<Protected><Dashboard /></Protected>} />
      <Route path="/analytics" element={<Protected><Analytics /></Protected>} />
      <Route path="/expenses" element={<Protected><Expenses /></Protected>} />
      <Route path="/advice" element={<Protected><Advice /></Protected>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
