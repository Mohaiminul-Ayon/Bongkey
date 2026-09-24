import Link from "next/link";

export function Footer() {
  return (
    <footer className="bg-neutral-950 px-4 py-12 text-white md:px-6">
      <div className="mx-auto grid max-w-7xl gap-8 sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <h2 className="text-2xl font-black tracking-[0.16em]">BONGKEY</h2>
          <p className="mt-4 max-w-xs text-sm leading-6 text-neutral-300">
            Affordable premium eyewear for Bangladesh, with Cash on Delivery and invoice support.
          </p>
        </div>
        <div>
          <h3 className="font-black">Shop</h3>
          <div className="mt-4 grid gap-3 text-sm text-neutral-300">
            <Link href="/products">Blue-cut glasses</Link>
            <Link href="/products">Sunglasses</Link>
            <Link href="/products">Prescription-ready frames</Link>
          </div>
        </div>
        <div>
          <h3 className="font-black">Support</h3>
          <div className="mt-4 grid gap-3 text-sm text-neutral-300">
            <span>7-day exchange support</span>
            <span>Prescription upload guidance</span>
            <span>Invoice and order confirmation</span>
          </div>
        </div>
        <div>
          <h3 className="font-black">Contact</h3>
          <div className="mt-4 grid gap-3 text-sm text-neutral-300">
            <span>WhatsApp: 01700-000000</span>
            <span>Email: care@bongkey.com</span>
            <span>@bongkeybd on Facebook, Instagram, and TikTok</span>
          </div>
        </div>
      </div>
      <div className="mx-auto mt-10 max-w-7xl border-t border-white/10 pt-6 text-sm text-neutral-400">
        © 2026 Bongkey. Built for eyewear shoppers in Bangladesh.
      </div>
    </footer>
  );
}
