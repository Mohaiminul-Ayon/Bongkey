// whatsAppNumber is the shop's public chat number in the international form wa.me needs (8801XXXXXXXXX).
// A local Bangladeshi number (01XXXXXXXXX, with or without +, spaces or dashes) is converted, so a typo-prone format still works.
const digits = (process.env.NEXT_PUBLIC_WHATSAPP_NUMBER ?? "").replace(/\D/g, "");
export const whatsAppNumber = digits.startsWith("0") ? `88${digits}` : digits;

/** whatsAppLink opens a chat with the shop with the given first message already typed. */
export function whatsAppLink(text: string) {
  return `https://wa.me/${whatsAppNumber}?text=${encodeURIComponent(text)}`;
}

/** whatsAppLinkTo opens a chat with a customer's local number (01XXXXXXXXX), for staff replying to a request. */
export function whatsAppLinkTo(localNumber: string, text: string) {
  return `https://wa.me/88${localNumber.replace(/\D/g, "")}?text=${encodeURIComponent(text)}`;
}
