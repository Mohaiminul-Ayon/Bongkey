"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/Button";
import { loginAdmin, setAdminToken } from "@/lib/api";

export default function AdminLoginPage() {
  const router = useRouter();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [email, setEmail] = useState("admin@bongkey.com");
  const [password, setPassword] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const result = await loginAdmin(email, password);
      setAdminToken(result.token);
      router.push("/admin/dashboard");
    } catch (err) {
      setError(
        err instanceof Error
          ? `${err.message}. Make sure the backend is running on port 8080 and PostgreSQL is started.`
          : "Login failed. Make sure the backend and PostgreSQL are running."
      );
    } finally {
      setLoading(false);
    }
  }

  function fillDefaultCredentials() {
    setEmail("admin@bongkey.com");
    setPassword("admin12345");
    setError("");
  }

  return (
    <section className="grid min-h-[calc(100vh-230px)] place-items-center px-4 py-12">
      <div className="grid w-full max-w-4xl overflow-hidden rounded-xl border border-black/10 bg-white shadow-sm md:grid-cols-[0.95fr_1.05fr]">
        <div className="hidden bg-[#173c34] p-8 text-white md:block">
          <p className="text-sm font-black uppercase tracking-[0.22em] text-white/70">Bongkey Admin</p>
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
                placeholder="admin@bongkey.com"
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
                placeholder="admin12345"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>
            <Button className="mt-2 w-full" type="submit">{loading ? "Signing In..." : "Sign In"}</Button>
            <button
              className="rounded-md border border-[#173c34] bg-[#f0f7f4] px-5 py-3 text-sm font-black text-[#173c34]"
              onClick={fillDefaultCredentials}
              type="button"
            >
              Fill Local MVP Login
            </button>
          </form>
          <div className="mt-5 rounded-md bg-[#f0f7f4] p-4 text-sm font-semibold leading-6 text-[#173c34]">
            <p>Local MVP admin login:</p>
            <p>Email: admin@bongkey.com</p>
            <p>Password: admin12345</p>
          </div>
        </div>
      </div>
    </section>
  );
}
