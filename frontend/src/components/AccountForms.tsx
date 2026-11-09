"use client";

import { FormEvent, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { requestEmailCode, setCustomerPassword, signInWithPassword, updateCustomerProfile, verifyEmailCode, type Customer } from "@/lib/api";

const message = (err: unknown, fallback: string) => (err instanceof Error ? err.message : fallback);

/** PasswordField is a password input with a show/hide button. */
function PasswordField({ id, autoComplete, label = "Password" }: { id: string; autoComplete: "current-password" | "new-password"; label?: string }) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Input id={id} name="password" type={visible ? "text" : "password"} required minLength={8} maxLength={72} autoComplete={autoComplete} className="pr-10" />
        <button type="button" onClick={() => setVisible(!visible)} aria-label={visible ? "Hide password" : "Show password"}
          className="absolute inset-y-0 right-0 grid w-10 place-items-center text-muted-foreground hover:text-foreground">
          {visible ? <EyeOff aria-hidden="true" className="size-4" /> : <Eye aria-hidden="true" className="size-4" />}
        </button>
      </div>
    </div>
  );
}

/** PasswordSignInForm signs in with email and password; the parent refreshes the shared sign-in state. */
export function PasswordSignInForm({ onSignedIn, onForgot }: { onSignedIn: () => Promise<unknown>; onForgot: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      await signInWithPassword(String(form.get("email") ?? ""), String(form.get("password") ?? ""));
      await onSignedIn();
    } catch (err) {
      setError(message(err, "Could not sign in."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="signin-email">Email</Label>
        <Input id="signin-email" name="email" type="email" required autoComplete="email" placeholder="you@gmail.com" />
      </div>
      <PasswordField id="signin-password" autoComplete="current-password" />
      <button type="button" onClick={onForgot} className="-mt-2 justify-self-end text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline">
        Forgot password?
      </button>
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
      <Button type="submit" size="lg" disabled={busy}>{busy ? "Signing in…" : "Sign in"}</Button>
    </form>
  );
}

/** EmailCodePasswordForm is "create account" and "reset password": prove the email with a 6-digit code, then set a password. */
export function EmailCodePasswordForm({ mode, onSignedIn }: { mode: "signup" | "reset"; onSignedIn: () => Promise<unknown> }) {
  const [email, setEmail] = useState("");
  const [codeSent, setCodeSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  /** sendCode asks the backend to email a code; the same answer is shown whether or not an account exists. */
  async function sendCode(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    setBusy(true);
    setError("");
    try {
      await requestEmailCode(email.trim());
      setCodeSent(true);
      setNotice(`We sent a 6-digit code to ${email.trim()}. It expires in 5 minutes.`);
    } catch (err) {
      setError(message(err, "Could not send the code."));
    } finally {
      setBusy(false);
    }
  }

  /** finish trades the code for a short-lived email proof, then sets the password and signs in. */
  async function finish(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      const { email_proof } = await verifyEmailCode(email.trim(), String(form.get("code") ?? "").trim());
      await setCustomerPassword(email_proof, String(form.get("password") ?? ""));
      await onSignedIn();
    } catch (err) {
      setError(message(err, "Could not finish. Check the code and try again."));
    } finally {
      setBusy(false);
    }
  }

  if (!codeSent) {
    return (
      <form onSubmit={sendCode} className="grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor={`${mode}-email`}>Email</Label>
          <Input id={`${mode}-email`} type="email" required autoComplete="email" placeholder="you@gmail.com" value={email} onChange={(event) => setEmail(event.target.value)} />
        </div>
        {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
        <Button type="submit" size="lg" disabled={busy}>{busy ? "Sending…" : "Send code"}</Button>
      </form>
    );
  }

  return (
    <form onSubmit={finish} className="grid gap-4">
      <p role="status" className="rounded-md bg-muted p-3 text-sm">{notice}</p>
      <div className="grid gap-2">
        <Label htmlFor={`${mode}-code`}>6-digit code</Label>
        <Input id={`${mode}-code`} name="code" required inputMode="numeric" pattern="[0-9]{6}" maxLength={6} autoComplete="one-time-code" placeholder="123456" />
      </div>
      <PasswordField id={`${mode}-password`} autoComplete="new-password" label={mode === "signup" ? "Choose a password" : "New password"} />
      <p className="-mt-2 text-xs text-muted-foreground">At least 8 characters. A short sentence is easy to remember.</p>
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
      <Button type="submit" size="lg" disabled={busy}>{busy ? "Saving…" : mode === "signup" ? "Create account" : "Save new password"}</Button>
      <div className="flex justify-between text-sm text-muted-foreground">
        <button type="button" onClick={() => { setCodeSent(false); setError(""); }} className="underline-offset-4 hover:text-foreground hover:underline">Change email</button>
        <button type="button" disabled={busy} onClick={() => void sendCode()} className="underline-offset-4 hover:text-foreground hover:underline">Send a new code</button>
      </div>
    </form>
  );
}

/** CompleteProfileForm asks for the required name and mobile number right after the first sign-in. */
export function CompleteProfileForm({ customer, onSaved }: { customer: Customer; onSaved: () => Promise<unknown> }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      await updateCustomerProfile(String(form.get("name") ?? ""), String(form.get("phone") ?? ""));
      await onSaved();
    } catch (err) {
      setError(message(err, "Could not save your profile."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="profile-name">Full name</Label>
        <Input id="profile-name" name="name" required minLength={2} maxLength={80} autoComplete="name" defaultValue={customer.name} placeholder="Your full name" />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="profile-phone">Mobile number</Label>
        <Input id="profile-phone" name="phone" required inputMode="tel" autoComplete="tel" defaultValue={customer.phone} placeholder="01XXXXXXXXX" />
        <p className="text-xs text-muted-foreground">We call this number to confirm your orders.</p>
      </div>
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
      <Button type="submit" size="lg" disabled={busy}>{busy ? "Saving…" : "Save and continue"}</Button>
    </form>
  );
}
