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
  { title: "Eyeglasses", text: "Everyday frames with a clean, considered fit." },
  { title: "Sunglasses", text: "Easy silhouettes for bright days." },
  { title: "Find your frame", text: "Explore the full Bongkey collection." },
];

const shopNotes = [
  { title: "Easy to explore", text: "Clear product details and pricing before you add a frame to your bag." },
  { title: "Try it your way", text: "Use the available try-on preview to get a feel for a frame." },
  { title: "Order with confidence", text: "Verify your email and review your order before checkout." },
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
          <p className="eyebrow">Bongkey eyewear / Bangladesh</p>
          <h1>See life <em>clearly.</em><br />Look entirely yourself.</h1>
          <p className="hero-description">Eyewear made for the way you live. Find a frame that feels right, with a simple path from browsing to checkout.</p>
          <div className="hero-actions">
            <Button href="/products">Explore the collection <span aria-hidden="true">↗</span></Button>
            <Button href="/products" variant="secondary">Shop all frames</Button>
          </div>
          <p className="hero-caption">Thoughtful style. Clear view. Everyday wear.</p>
        </div>
        <div className="home-hero-media">
          <Image alt="Woman and man wearing modern eyeglasses" fill priority sizes="(max-width: 1000px) 100vw, 55vw" src="/images/eyewear-editorial.png" />
          <span className="hero-image-note">A better way to see your day.</span>
        </div>
      </section>

      <section className="home-section">
        <div className="section-heading">
          <div><p className="eyebrow">01 / Discover</p><h2>Find your next frame.</h2></div>
          <p>Simple choices, made for every day.</p>
        </div>
        <div className="category-grid">
          {categories.map((category, index) => <CategoryCard key={category.title} index={index + 1} {...category} />)}
        </div>
      </section>

      <section className="home-section featured-section" id="featured">
        <div className="section-heading">
          <div><p className="eyebrow">02 / Selected for you</p><h2>Featured eyewear.</h2></div>
          <Button href="/products" variant="ghost">View all <span aria-hidden="true">↗</span></Button>
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
        <Button href="/products" variant="secondary">Discover frames <span aria-hidden="true">↗</span></Button>
      </section>

      <section className="home-section" id="how-it-works">
        <div className="section-heading"><div><p className="eyebrow">03 / How it works</p><h2>Shop with clarity.</h2></div></div>
        <div className="notes-grid">{shopNotes.map((item, index) => <TrustBadge key={item.title} index={index + 1} {...item} />)}</div>
      </section>
    </>
  );
}
