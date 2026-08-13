"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api, Principal } from "@/lib/api";

type AuthContextValue = { principal: Principal | null; loading: boolean; reload: () => Promise<void>; logout: () => Promise<void> };
const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [principal, setPrincipal] = useState<Principal | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();
  const reload = async () => {
    try { setPrincipal(await api<Principal>("/me")); }
    catch { setPrincipal(null); if (pathname !== "/login") router.replace(`/login?return_to=${encodeURIComponent(pathname)}`); }
    finally { setLoading(false); }
  };
  useEffect(() => {
    let active = true;
    void api<Principal>("/me").then((value) => { if (active) setPrincipal(value); }).catch(() => {
      if (!active) return;
      setPrincipal(null);
      if (pathname !== "/login") router.replace(`/login?return_to=${encodeURIComponent(pathname)}`);
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [pathname, router]);
  const value = useMemo<AuthContextValue>(() => ({ principal, loading, reload, logout: async () => { await api("/auth/logout", { method: "POST" }); setPrincipal(null); router.replace("/login"); } }), [principal, loading]); // eslint-disable-line react-hooks/exhaustive-deps
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used inside AuthProvider");
  return context;
}
