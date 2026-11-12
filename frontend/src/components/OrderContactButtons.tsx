import { whatsAppLinkTo } from "@/lib/whatsapp";

/** OrderContactButtons lets staff call or WhatsApp the customer about one order.
 * The ready message names only the order, never prescription or payment details. */
export function OrderContactButtons({ name, phone, orderNumber }: { name: string; phone: string; orderNumber: string }) {
  return (
    <div className="flex flex-wrap gap-2">
      <a className="rounded-md border border-neutral-300 px-3 py-2 text-sm font-bold" href={`tel:${phone}`}>Call</a>
      <a className="rounded-md border border-neutral-300 px-3 py-2 text-sm font-bold" target="_blank" rel="noopener noreferrer"
        href={whatsAppLinkTo(phone, `Hi ${name}, this is Bongkey about your order ${orderNumber}.`)}>WhatsApp</a>
    </div>
  );
}
