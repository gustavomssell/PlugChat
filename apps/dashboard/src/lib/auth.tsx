import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { AuthResult, Availability } from "@chatfree/shared";
import { api } from "@/lib/api";
import { clearToken, getToken, setToken } from "@/lib/session";

interface AuthContextValue {
  session: AuthResult | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (input: {
    name: string;
    email: string;
    password: string;
    workspaceName: string;
  }) => Promise<void>;
  logout: () => void;
  refresh: () => Promise<void>;
  setAvailability: (availability: Availability) => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<AuthResult | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setSession(null);
      setLoading(false);
      return;
    }

    try {
      const result = await api.me();
      setSession(result);
    } catch {
      clearToken();
      setSession(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    const handleUnauthorized = () => {
      clearToken();
      setSession(null);
    };
    window.addEventListener("chatfree:unauthorized", handleUnauthorized);
    return () => window.removeEventListener("chatfree:unauthorized", handleUnauthorized);
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const result = await api.login(email, password);
    setToken(result.token);
    const { token: _token, ...rest } = result;
    setSession(rest);
  }, []);

  const register = useCallback(
    async (input: { name: string; email: string; password: string; workspaceName: string }) => {
      const result = await api.register(input);
      setToken(result.token);
      const { token: _token, ...rest } = result;
      setSession(rest);
    },
    [],
  );

  const logout = useCallback(() => {
    clearToken();
    setSession(null);
  }, []);

  const setAvailability = useCallback(async (availability: Availability) => {
    await api.setAvailability(availability);
    setSession((current) =>
      current
        ? { ...current, membership: { ...current.membership, availability } }
        : current,
    );
  }, []);

  const value = useMemo(
    () => ({ session, loading, login, register, logout, refresh, setAvailability }),
    [session, loading, login, register, logout, refresh, setAvailability],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth deve ser usado dentro de AuthProvider");
  }
  return context;
}
