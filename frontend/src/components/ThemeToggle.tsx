"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";

/** ThemeToggle persists the visitor's light or dark display choice. */
export function ThemeToggle() {
  const [theme, setTheme] = useState<"light" | "dark">("light");

  useEffect(() => {
    setTheme(document.documentElement.dataset.theme === "dark" ? "dark" : "light");
  }, []);

  /** toggleTheme updates the live palette and saves the choice when storage is available. */
  function toggleTheme() {
    const next = theme === "light" ? "dark" : "light";
    document.documentElement.dataset.theme = next;
    setTheme(next);
    try {
      localStorage.setItem("bongkey-theme", next);
    } catch {
      // Theme switching still works when browser storage is disabled.
    }
  }

  return (
    <button aria-label={`Switch to ${theme === "light" ? "dark" : "light"} mode`} className="icon-link" onClick={toggleTheme} type="button">
      {theme === "light" ? <Moon aria-hidden="true" /> : <Sun aria-hidden="true" />}
    </button>
  );
}
