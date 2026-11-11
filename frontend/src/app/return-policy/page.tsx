import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Return & exchange policy · Bongkey" };

// The rules here must match the server's returns package (3 and 15 days, refunds for problems only).
const sections: { title: string; points: string[] }[] = [
  {
    title: "Check your parcel at delivery",
    points: [
      "Please open the parcel in front of the delivery person.",
      "If something is wrong, hand it back right away. You pay nothing more, because you already paid for delivery once.",
    ],
  },
  {
    title: "Exchange a frame (change of mind)",
    points: [
      "You can exchange a frame for another one within 3 days of delivery.",
      "Same price: nothing more to pay. A pricier frame: you pay the difference.",
    ],
  },
  {
    title: "Defect, wrong colour or wrong item",
    points: [
      "Tell us within 15 days of delivery. We replace the frame, or refund it if we can't.",
      "A refund is paid by bKash within 15 days of your request.",
    ],
  },
  {
    title: "Glasses with lenses",
    points: [
      "Lenses are made to your prescription, so they are not refunded or exchanged.",
      "The frame follows the rules above. If you return the frame, the lens price is taken off the refund, and a new lens for an exchanged frame is paid again.",
    ],
  },
  {
    title: "Courier charge",
    points: [
      "After the delivery person has left, the courier charge for sending the item back is yours. It is taken off a refund or paid with an exchange.",
      "The delivery charge you paid for the original order is not refunded.",
    ],
  },
  {
    title: "How to ask",
    points: [
      "Open your order (or find it on the Track page with your order number and phone) and press \"Request a return or exchange\".",
      "Tell us what happened and add photos if you can. We call you to agree a solution, and you can chat with us on WhatsApp.",
    ],
  },
];

/** ReturnPolicyPage is the plain-language return, exchange and refund policy linked from the footer, account,
 * checkout and payment pages. */
export default function ReturnPolicyPage() {
  return (
    <section className="mx-auto grid max-w-3xl gap-8 px-4 py-12 md:px-6">
      <div className="grid gap-3">
        <p className="kicker">Help</p>
        <h1 className="display text-4xl">Return &amp; exchange policy</h1>
        <p className="text-[var(--muted)]">Short and simple: what you can exchange or return, for how long, and who pays the courier.</p>
      </div>
      {sections.map((section) => (
        <div key={section.title} className="grid gap-2 border-t border-[var(--border)] pt-6">
          <h2 className="text-lg font-semibold">{section.title}</h2>
          <ul className="grid list-disc gap-1.5 pl-5 text-[15px] leading-7">
            {section.points.map((point) => <li key={point}>{point}</li>)}
          </ul>
        </div>
      ))}
      <p className="border-t border-[var(--border)] pt-6 text-sm text-[var(--muted)]">Find your order on the <Link className="font-semibold text-[var(--foreground)] underline underline-offset-4" href="/track">Track page</Link> to start a request.</p>
    </section>
  );
}
