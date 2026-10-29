"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import { Button } from "@/components/Button";
import { CategoryCard } from "@/components/CategoryCard";
import { ProductCard } from "@/components/ProductCard";
import { TrustBadge } from "@/components/TrustBadge";
import { fetchFeaturedProducts } from "@/lib/api";
import { type Product } from "@/lib/products";

const categories = [
  { title: "Eyeglasses", text: "Everyday frames with a clean, considered fit.", href: "/products?type=eyeglasses" },
  { title: "Sunglasses", text: "Easy silhouettes for bright days.", href: "/products?type=sunglasses" },
  { title: "Shop all", text: "Browse the full Bongkey collection.", href: "/products" },
];

const shopNotes = [
  { title: "Pick your frame", text: "Clear product details and pricing before you add a frame to your bag." },
  { title: "Verify your email", text: "A one-time code keeps your order and invoice private to you." },
  { title: "Pay on delivery", text: "Pay cash when your frame arrives. Your invoice is ready to download." },
];

/** Home presents the live featured catalog with an original editorial hero image. */
export default function Home() {
  const [featured, setFeatured] = useState<Product[]>([]);

  useEffect(() => {
    fetchFeaturedProducts().then(setFeatured).catch(() => setFeatured([]));
  }, []);

  return (
    <>
      <section className="home-hero">
        <div className="home-hero-copy">
          <p className="eyebrow">Bongkey eyewear · Bangladesh</p>
          <h1>See life <em>clearly.</em><br />Look entirely yourself.</h1>
          <p className="hero-description">Eyewear made for the way you live. Find a frame that feels right, with a simple path from browsing to checkout.</p>
          <div className="hero-actions">
            <Button href="/products?type=eyeglasses">Shop eyeglasses</Button>
            <Button href="/products?type=sunglasses" variant="secondary">Shop sunglasses</Button>
          </div>
          <ul className="hero-perks"><li>Cash on delivery</li><li>Delivery across Bangladesh</li><li>Invoice with every order</li></ul>
        </div>
        <div className="home-hero-media">
          <Image alt="Woman and man wearing modern eyeglasses" fill priority sizes="(max-width: 1000px) 100vw, 55vw" src="/images/eyewear-editorial.png" />
        </div>
      </section>

      <section className="home-section">
        <div className="section-heading">
          <h2>Find your next frame</h2>
          <p>Simple choices, made for every day.</p>
        </div>
        <div className="category-grid">
          {categories.map((category) => <CategoryCard key={category.title} {...category} />)}
        </div>
      </section>

      <section className="home-section featured-section" id="featured">
        <div className="section-heading">
          <h2>Featured frames</h2>
          <Button href="/products" variant="ghost">View all</Button>
        </div>
        <div className="product-grid">
          {featured.map((product) => <ProductCard key={product.slug} product={product} />)}
        </div>
        {featured.length === 0 ? <p className="empty-note">Featured frames will appear here when available.</p> : null}
      </section>

      <section className="editorial-band">
        <div>
          <p className="eyebrow">The Bongkey approach</p>
          <h2>Good glasses make the everyday feel different.</h2>
        </div>
        <p>From your first look to the final fit, choosing a frame should feel clear and uncomplicated.</p>
        <Button href="/products" variant="secondary">Discover frames</Button>
      </section>

      <section className="home-section" id="how-it-works">
        <div className="section-heading"><h2>How ordering works</h2></div>
        <div className="notes-grid">{shopNotes.map((item, index) => <TrustBadge key={item.title} index={index + 1} {...item} />)}</div>
      </section>
    </>
  );
}
