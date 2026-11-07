"use client";

import { useEffect, useRef } from "react";

type GoogleId = {
  initialize(options: { client_id: string; callback: (response: { credential: string }) => void }): void;
  renderButton(element: HTMLElement, options: Record<string, string | number>): void;
};

declare global {
  interface Window {
    google?: { accounts: { id: GoogleId } };
  }
}

const clientId = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID ?? "";
const scriptUrl = "https://accounts.google.com/gsi/client";

/** googleSignInEnabled is false until the owner sets NEXT_PUBLIC_GOOGLE_CLIENT_ID. */
export const googleSignInEnabled = clientId !== "";

/** loadGoogleScript adds Google's sign-in script once and resolves when it is ready. */
function loadGoogleScript() {
  return new Promise<void>((resolve, reject) => {
    if (window.google?.accounts) return resolve();
    let script = document.querySelector<HTMLScriptElement>(`script[src="${scriptUrl}"]`);
    if (!script) {
      script = Object.assign(document.createElement("script"), { src: scriptUrl, async: true });
      document.head.appendChild(script);
    }
    script.addEventListener("load", () => resolve(), { once: true });
    script.addEventListener("error", () => reject(new Error("Google sign-in could not load.")), { once: true });
  });
}

type Props = { onCredential: (credential: string) => void; onError: (message: string) => void };

/** GoogleSignInButton draws Google's own button and hands the ID token to onCredential. */
export function GoogleSignInButton({ onCredential, onError }: Props) {
  const box = useRef<HTMLDivElement>(null);
  // A ref keeps the latest callbacks without re-initialising Google's button on every render.
  const handlers = useRef({ onCredential, onError });
  useEffect(() => {
    handlers.current = { onCredential, onError };
  });

  useEffect(() => {
    if (!googleSignInEnabled) return;
    let cancelled = false;
    loadGoogleScript()
      .then(() => {
        if (cancelled || !box.current || !window.google) return;
        window.google.accounts.id.initialize({ client_id: clientId, callback: (response) => handlers.current.onCredential(response.credential) });
        window.google.accounts.id.renderButton(box.current, { theme: "outline", size: "large", text: "signin_with", shape: "pill" });
      })
      .catch((err: Error) => handlers.current.onError(err.message));
    return () => {
      cancelled = true;
    };
  }, []);

  return <div ref={box} />;
}
