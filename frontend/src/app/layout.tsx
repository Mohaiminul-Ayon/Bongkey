import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { Footer } from "@/components/Footer";
import { Header } from "@/components/Header";
import { CartProvider } from "@/lib/cart";
import { CustomerProvider } from "@/lib/customer";
import { WishlistProvider } from "@/lib/wishlist";
import "./globals.css";

const inter = Inter({ subsets: ["latin"], variable: "--font-sans" });

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
    <html lang="en" className={inter.variable} suppressHydrationWarning>
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
            </CustomerProvider>
          </WishlistProvider>
        </CartProvider>
      </body>
    </html>
  );
}
