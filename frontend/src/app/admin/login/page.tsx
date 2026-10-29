"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/Button";
import { BrandLogo } from "@/components/BrandLogo";
import { loginAdmin, setAdminToken } from "@/lib/api";

// AdminLoginPage submits owner credentials without bundling demo credentials.
export default function AdminLoginPage() {
  const router = useRouter();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  // submit stores only a successful owner session in this browser tab.
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const result = await loginAdmin(email, password);
      setAdminToken(result.token);
      router.push("/admin/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="grid min-h-[calc(100vh-230px)] place-items-center px-4 py-12">
      <div className="grid w-full max-w-4xl overflow-hidden rounded-xl border border-black/10 bg-white shadow-sm md:grid-cols-[0.95fr_1.05fr]">
        <div className="hidden bg-neutral-950 p-8 text-white md:block">
          <span className="brand-on-dark"><BrandLogo size="lg" /></span>
          <p className="mt-8 text-sm font-black uppercase tracking-[0.22em] text-neutral-300">Bongkey Admin</p>
          <h1 className="mt-4 text-4xl font-black leading-tight">Manage eyewear orders with confidence.</h1>
          <div className="mt-8 grid gap-3 text-sm font-semibold text-white/80">
            <span>Product and stock management</span>
            <span>Cash on Delivery order review</span>
            <span>Invoice and prescription workflow</span>
          </div>
        </div>
        <div className="p-6 sm:p-8">
          <div className="mb-8">
            <p className="text-2xl font-black tracking-[0.16em] text-neutral-950">BONGKEY</p>
            <p className="mt-2 text-sm font-semibold text-neutral-500">Sign in to the admin dashboard</p>
          </div>
          <form className="grid gap-4" onSubmit={submit}>
            {error ? <p className="rounded-md bg-red-50 p-3 text-sm font-bold text-red-700">{error}</p> : null}
            <label className="grid gap-2 text-sm font-black">
              Email
              <input
                name="email"
                required
                className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]"
                placeholder="Owner email"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </label>
            <label className="grid gap-2 text-sm font-black">
              Password
              <input
                name="password"
                required
                className="rounded-md border border-neutral-300 px-4 py-3 font-normal outline-none focus:border-[#173c34]"
                placeholder="Password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>
            <Button className="mt-2 w-full" type="submit">{loading ? "Signing In..." : "Sign In"}</Button>
          </form>
        </div>
      </div>
    </section>
  );
}
