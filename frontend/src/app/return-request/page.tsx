"use client";

import Link from "next/link";
import { ReturnRequestForm } from "@/components/ReturnRequestForm";
import { requestReturnFromTrack } from "@/lib/api";
import { whatsAppLink, whatsAppNumber } from "@/lib/whatsapp";

const linkClass = "font-semibold text-[var(--foreground)] underline underline-offset-4";

/** ReturnRequestPage lets a customer open a return or exchange request with only the order number and the phone
 * used at checkout (the same proof as the Track page); staff then call them. */
export default function ReturnRequestPage() {
  return (
    <section className="mx-auto grid max-w-2xl gap-6 px-4 py-12 md:px-6">
      <div className="grid gap-3">
        <p className="kicker">Help</p>
        <h1 className="display text-4xl">Return or exchange request</h1>
        <p className="text-[var(--muted)]">Tell us what is wrong and we will call you to sort it out. Please read our <Link className={linkClass} href="/return-policy">return &amp; exchange policy</Link> first.</p>
      </div>
      <ReturnRequestForm send={requestReturnFromTrack}>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="grid gap-1 font-semibold">Order number
            <input name="order_number" required maxLength={20} autoComplete="off" placeholder="ORD-000123" className="product-select font-normal" />
          </label>
          <label className="grid gap-1 font-semibold">Phone you ordered with
            <input name="phone" required inputMode="tel" autoComplete="tel" placeholder="01XXXXXXXXX" className="product-select font-normal" />
          </label>
        </div>
      </ReturnRequestForm>
      <p className="text-sm text-[var(--muted)]">
        Lost your order number? It is on your order slip and in our order email, or find it on the <Link className={linkClass} href="/track">Track page</Link>.
        {whatsAppNumber ? <> You can also <a className={linkClass} target="_blank" rel="noopener noreferrer" href={whatsAppLink("Hi Bongkey, I need help with a return or exchange.")}>message us on WhatsApp</a>.</> : null}
      </p>
    </section>
  );
}
