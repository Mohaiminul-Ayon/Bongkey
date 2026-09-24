import type { Metadata } from "next";
import { Footer } from "@/components/Footer";
import { Header } from "@/components/Header";
import { FloatingCart } from "@/components/FloatingCart";
import { CartProvider } from "@/lib/cart";
import "./globals.css";

export const metadata: Metadata = {
  title: "Bongkey Eyewear",
  description: "Budget to premium eyewear in Bangladesh.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <CartProvider>
          <Header />
          <main className="min-h-screen">{children}</main>
          <FloatingCart />
          <Footer />
        </CartProvider>
      </body>
    </html>
  );
}
