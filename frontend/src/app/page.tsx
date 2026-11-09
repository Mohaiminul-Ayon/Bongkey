"use client";

import { useEffect, useState, type ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import { ProductCard } from "@/components/ProductCard";
import { assetUrl, fetchFeaturedProducts, fetchProducts } from "@/lib/api";
import { type Product } from "@/lib/products";

type Shelf = "featured" | "eyeglasses" | "sunglasses";

const shelves: { key: Shelf; label: string; href: string; link: string }[] = [
  { key: "featured", label: "Featured", href: "/products", link: "Shop all frames" },
  { key: "eyeglasses", label: "Eyeglasses", href: "/products?type=eyeglasses", link: "All eyeglasses" },
  { key: "sunglasses", label: "Sunglasses", href: "/products?type=sunglasses", link: "All sunglasses" },
];

const steps = [
  { title: "Pick your frame", text: "Clear photos, sizes and prices before anything goes in your bag." },
  { title: "We call to confirm", text: "We phone you to check the frame, lenses and address before it ships." },
  { title: "Pay your way", text: "Cash on delivery, with a small bKash advance when an order needs one." },
];

/** TextLink is the quiet underlined call to action used across the home page. */
function TextLink({ href, children }: { href: string; children: ReactNode }) {
  return <Link className="text-link" href={href}>{children} <span aria-hidden="true">→</span></Link>;
}

/** Home is an editorial storefront: a full-width hero, one shelf with Featured / Eyeglasses / Sunglasses, a photo story and the ordering steps. */
export default function Home() {
  const [featured, setFeatured] = useState<Product[]>([]);
  const [catalog, setCatalog] = useState<Product[]>([]);
  const [shelf, setShelf] = useState<Shelf>("featured");

  useEffect(() => {
    fetchFeaturedProducts().then(setFeatured).catch(() => setFeatured([]));
    fetchProducts().then(setCatalog).catch(() => setCatalog([]));
  }, []);

  // Every non-sunglass frame is an eyeglass, the same split the catalog page uses.
  const shown = (shelf === "featured" ? (featured.length ? featured : catalog)
    : catalog.filter((product) => (product.shape === "sunglass") === (shelf === "sunglasses"))).slice(0, 8);
  const active = shelves.find((item) => item.key === shelf) ?? shelves[0];
  const story = catalog.find((product) => product.brandTier === "Premium" && product.imageUrl) ?? catalog.find((product) => product.imageUrl);
  const storyImage = assetUrl(story?.imageUrl);

  return (
    <>
      <section className="home-hero" aria-labelledby="home-title">
        <Image alt="Woman and man wearing modern eyeglasses" fill priority sizes="100vw" src="/images/eyewear-editorial.png" />
        <div className="home-hero-copy">
          <p className="kicker">Bongkey eyewear · Bangladesh</p>
          <h1 id="home-title" className="display">See life <em>clearly.</em></h1>
          <p className="hero-description">Frames for the way you live, delivered across Bangladesh.</p>
          <div className="hero-actions">
            <Link className="hero-button" href="/products?type=eyeglasses">Shop eyeglasses</Link>
            <TextLink href="/products?type=sunglasses">Shop sunglasses</TextLink>
          </div>
        </div>
      </section>
      <ul className="perk-strip" aria-label="Why shop with Bongkey">
        <li>Cash on delivery</li><li>Delivery across Bangladesh</li><li>Invoice with every order</li>
      </ul>

      <section className="home-section" aria-labelledby="shelf-title">
        <div className="shelf-head">
          <div>
            <p className="kicker">The collection</p>
            <h2 id="shelf-title" className="display">Find your <em>frame</em></h2>
          </div>
          <div className="shelf-tabs" role="group" aria-label="Show frames">
            {shelves.map((item) => (
              <button key={item.key} type="button" aria-pressed={shelf === item.key} onClick={() => setShelf(item.key)}>{item.label}</button>
            ))}
          </div>
        </div>
        <div className="product-grid" aria-live="polite">
          {shown.map((product) => <ProductCard key={product.slug} product={product} />)}
        </div>
        {shown.length === 0 ? <p className="empty-note">Frames will appear here soon.</p> : null}
        <div className="shelf-foot"><TextLink href={active.href}>{active.link}</TextLink></div>
      </section>

      <section className="home-story" aria-labelledby="story-title">
        <div className="home-story-media">
          {storyImage ? <img alt={story?.name ?? ""} src={storyImage} /> : null}
        </div>
        <div className="home-story-copy">
          <p className="kicker">The Bongkey approach</p>
          <h2 id="story-title" className="display">Good glasses make the everyday feel <em>different.</em></h2>
          <p>From your first look to the final fit, choosing a frame should feel clear and unhurried. Every order is checked by a person before it leaves us.</p>
          <TextLink href={story ? `/products/${story.slug}` : "/products"}>{story ? `Discover ${story.name}` : "Discover frames"}</TextLink>
        </div>
      </section>

      <section className="home-section" aria-labelledby="steps-title">
        <p className="kicker">How ordering works</p>
        <h2 id="steps-title" className="display">Simple from <em>start to finish</em></h2>
        <ol className="home-steps">
          {steps.map((step) => <li key={step.title}><h3>{step.title}</h3><p>{step.text}</p></li>)}
        </ol>
      </section>
    </>
  );
}
