"use client";

import Link from "next/link";
import type { ReactNode } from "react";

type ButtonProps = {
  children: ReactNode;
  href?: string;
  variant?: "primary" | "secondary" | "ghost";
  className?: string;
  type?: "button" | "submit";
  onClick?: () => void;
};

const variants = {
  primary: "bg-neutral-950 text-white hover:bg-[#173c34]",
  secondary: "border border-[#173c34] bg-[#f0f7f4] text-[#173c34] hover:bg-[#e4f1ec]",
  ghost: "border border-neutral-200 bg-white text-neutral-950 hover:border-[#173c34]",
};

export function Button({
  children,
  href,
  variant = "primary",
  className = "",
  type = "button",
  onClick,
}: ButtonProps) {
  const classes = `inline-flex min-h-11 items-center justify-center rounded-md px-5 py-3 text-sm font-black transition ${variants[variant]} ${className}`;

  if (href) {
    return (
      <Link className={classes} href={href}>
        {children}
      </Link>
    );
  }

  return (
    <button className={classes} onClick={onClick} type={type}>
      {children}
    </button>
  );
}
