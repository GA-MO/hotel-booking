"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { Auth } from "@/app/lib/api";
import { clearAuth, getRefreshToken } from "@/app/lib/auth";

export default function LogoutPage() {
  const router = useRouter();
  useEffect(() => {
    (async () => {
      const rt = getRefreshToken();
      if (rt) {
        try {
          await Auth.logout(rt);
        } catch {
          // best-effort
        }
      }
      clearAuth();
      router.replace("/login");
    })();
  }, [router]);

  return (
    <main className="flex min-h-screen items-center justify-center">
      <p className="text-sm text-neutral-500">…</p>
    </main>
  );
}
