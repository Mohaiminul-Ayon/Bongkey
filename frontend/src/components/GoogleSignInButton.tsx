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

type Handlers = { onCredential: (credential: string) => void; onError: (message: string) => void };
type Props = Handlers & { text?: "signin_with" | "signup_with" | "continue_with" };

// Google allows one initialize per page, so the latest mounted button's handlers receive the credential.
const active: { current: Handlers | null; initialized: boolean } = { current: null, initialized: false };

/** GoogleSignInButton draws Google's own button and hands the ID token to onCredential. */
export function GoogleSignInButton({ onCredential, onError, text = "signin_with" }: Props) {
  const box = useRef<HTMLDivElement>(null);
  // A ref keeps the latest callbacks without re-initialising Google's button on every render.
  const handlers = useRef({ onCredential, onError });
  useEffect(() => {
    handlers.current = { onCredential, onError };
    active.current = handlers.current;
  });

  useEffect(() => {
    if (!googleSignInEnabled) return;
    let cancelled = false;
    // draw renders the button at the form's width (Google allows up to 400 px) in the site's current theme;
    // locale "en" keeps it English whatever the visitor's browser language is.
    const draw = () => {
      if (cancelled || !box.current || !window.google) return;
      const dark = document.documentElement.dataset.theme === "dark";
      window.google.accounts.id.renderButton(box.current, {
        theme: dark ? "filled_black" : "outline", size: "large", text, shape: "rectangular",
        width: Math.min(400, box.current.offsetWidth || 320), locale: "en", logo_alignment: "center",
      });
    };
    // The theme toggle only changes data-theme on <html>, so redraw when it does.
    const themeWatcher = new MutationObserver(draw);
    loadGoogleScript()
      .then(() => {
        if (cancelled || !window.google) return;
        if (!active.initialized) {
          window.google.accounts.id.initialize({ client_id: clientId, callback: (response) => active.current?.onCredential(response.credential) });
          active.initialized = true;
        }
        draw();
        themeWatcher.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
      })
      .catch((err: Error) => handlers.current.onError(err.message));
    return () => {
      cancelled = true;
      themeWatcher.disconnect();
    };
  }, [text]);

  return <div ref={box} className="flex min-h-11 w-full justify-center" />;
}
