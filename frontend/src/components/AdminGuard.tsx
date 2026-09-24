"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { getAdminToken } from "@/lib/api";

export function AdminGuard({ children }: { children: React.ReactNode }) {
  const [allowed, setAllowed] = useState(false);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    setAllowed(Boolean(getAdminToken()));
    setChecked(true);
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
