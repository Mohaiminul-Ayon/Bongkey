import type { Metadata } from "next";
import { Fraunces, Inter } from "next/font/google";
import { Footer } from "@/components/Footer";
import { Header } from "@/components/Header";
import { CartProvider } from "@/lib/cart";
import { BagPanel } from "@/components/BagPanel";
import { CustomerProvider } from "@/lib/customer";
import { WishlistProvider } from "@/lib/wishlist";
import "./globals.css";

const inter = Inter({ subsets: ["latin"], variable: "--font-sans" });
// Fraunces is the serif for large editorial headings only; body text stays Inter.
const fraunces = Fraunces({ subsets: ["latin"], style: ["normal", "italic"], variable: "--font-display" });

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
    <html lang="en" className={`${inter.variable} ${fraunces.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: "try{document.documentElement.dataset.theme=localStorage.getItem('bongkey-theme')==='dark'?'dark':'light'}catch{document.documentElement.dataset.theme='light'}" }} />
      </head>
      <body>
        <CartProvider>
          <WishlistProvider>
            <CustomerProvider>
              <Header />
              <main className="min-h-screen">{children}</main>
              <Footer />
              <BagPanel />
            </CustomerProvider>
          </WishlistProvider>
        </CartProvider>
      </body>
    </html>
  );
}
