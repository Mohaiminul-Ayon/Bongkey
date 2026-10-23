"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { adminRequest, clearAdminToken, getAdminToken } from "@/lib/api";

// AdminGuard renders owner pages only after the backend accepts the current tab session.
export function AdminGuard({ children }: { children: React.ReactNode }) {
  const [allowed, setAllowed] = useState(false);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    let active = true;
    if (!getAdminToken()) {
      setChecked(true);
      return;
    }
    adminRequest<{ authenticated: boolean }>("/api/admin/me")
      .then(({ authenticated }) => {
        if (active) setAllowed(authenticated);
      })
      .catch(() => {
        clearAdminToken();
        if (active) setAllowed(false);
      })
      .finally(() => {
        if (active) setChecked(true);
      });
    return () => { active = false; };
  }, []);

  if (!checked) return <div className="p-8">Checking admin session...</div>;
  if (!allowed) {
    return (
      <div className="mx-auto max-w-xl px-4 py-12 text-center">
        <h1 className="text-3xl font-black">Admin login required</h1>
        <Link className="mt-5 inline-flex rounded-md bg-neutral-950 px-5 py-3 font-black text-white" href="/admin/login">Go to Login</Link>
      </div>
    );
  }
  return <>{children}</>;
}
