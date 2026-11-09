import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** cn joins class names and lets a later Tailwind class override an earlier one (shadcn/ui's merge helper). */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
