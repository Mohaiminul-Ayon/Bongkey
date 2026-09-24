import { Button } from "@/components/Button";
import { CategoryCard } from "@/components/CategoryCard";
import { ProductCard } from "@/components/ProductCard";
import { ProductVisual } from "@/components/ProductVisual";
import { TrustBadge } from "@/components/TrustBadge";
import { featuredProducts, products } from "@/lib/products";

const categories = [
  {
    title: "Blue-Cut Glasses",
    text: "Comfortable lenses for long office hours, study sessions, and phone-heavy days.",
  },
  {
    title: "Sunglasses",
    text: "Clean everyday sun protection for commutes, travel, and weekends.",
  },
  {
    title: "Men's Frames",
    text: "Structured silhouettes with lightweight materials and confident finishes.",
  },
  {
    title: "Women's Frames",
    text: "Soft, refined, and modern shapes made for daily wear.",
  },
  {
    title: "Premium Collection",
    text: "Better materials, polished details, and elevated Bongkey styling.",
  },
  {
    title: "Budget Picks",
    text: "Good-looking frames with practical prices and Cash on Delivery.",
  },
];

const trustItems = [
  {
    title: "Cash on Delivery",
    text: "Order first, pay when your frame arrives at your doorstep.",
  },
  {
    title: "Easy Exchange",
    text: "Size or style concerns are handled through a simple exchange process.",
  },
  {
    title: "Prescription Upload",
    text: "Share your prescription during checkout for lens-ready orders.",
  },
  {
    title: "Invoice Provided",
    text: "Every confirmed order includes a proper invoice for your records.",
  },
];

export default function Home() {
  const heroProduct = products[2];

  return (
    <>
      <section className="bg-white">
        <div className="mx-auto grid max-w-7xl gap-10 px-4 py-12 md:grid-cols-[1.05fr_0.95fr] md:items-center md:px-6 lg:py-18">
          <div>
            <p className="mb-4 text-sm font-black uppercase tracking-[0.22em] text-[#9a6a24]">
              Bongkey Eyewear Bangladesh
            </p>
            <h1 className="max-w-4xl text-4xl font-black leading-tight text-neutral-950 md:text-6xl">
              Affordable Premium Eyewear in Bangladesh
            </h1>
            <p className="mt-5 max-w-2xl text-lg leading-8 text-neutral-700">
              Shop blue-cut glasses, sunglasses, and prescription-ready frames
              with modern styling, practical pricing, and Cash on Delivery across Bangladesh.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <Button href="/products">Shop Now</Button>
              <Button href="/products/bk-blue-101" variant="secondary">
                Try On Frames
              </Button>
            </div>
            <div className="mt-8 grid grid-cols-2 gap-3 text-sm font-bold text-neutral-700 sm:grid-cols-4">
              {["COD across BD", "Invoice included", "Blue-cut options", "Try-on ready"].map((item) => (
                <div key={item} className="rounded-md border border-black/10 bg-[#f7f5ef] px-3 py-3">
                  {item}
                </div>
              ))}
            </div>
          </div>
          <div className="rounded-xl border border-black/10 bg-[#f7f5ef] p-4 shadow-sm">
            <ProductVisual product={heroProduct} />
            <div className="mt-4 flex items-center justify-between gap-4 rounded-lg bg-white p-4">
              <div>
                <p className="text-xs font-black uppercase tracking-[0.18em] text-[#9a6a24]">Featured frame</p>
                <h2 className="mt-1 font-black text-neutral-950">{heroProduct.name}</h2>
              </div>
              <span className="rounded-full bg-[#f0f7f4] px-3 py-1 text-xs font-black text-[#173c34]">
                Try online
              </span>
            </div>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-4 py-12 md:px-6">
        <div className="mb-7 flex items-end justify-between gap-4">
          <div>
            <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Shop categories</p>
            <h2 className="mt-2 text-3xl font-black text-neutral-950">Find frames by your daily need</h2>
          </div>
          <Button className="hidden sm:inline-flex" href="/products" variant="ghost">
            View All
          </Button>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {categories.map((category) => (
            <CategoryCard key={category.title} title={category.title} text={category.text} />
          ))}
        </div>
      </section>

      <section className="bg-white py-12">
        <div className="mx-auto max-w-7xl px-4 md:px-6">
          <div className="mb-7 flex items-end justify-between gap-4">
            <div>
              <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Featured products</p>
              <h2 className="mt-2 text-3xl font-black text-neutral-950">Frames customers can order today</h2>
            </div>
            <Button href="/products" variant="ghost">
              Browse Products
            </Button>
          </div>
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {featuredProducts.map((product) => (
              <ProductCard key={product.slug} product={product} />
            ))}
          </div>
        </div>
      </section>

      <section className="mx-auto grid max-w-7xl gap-8 px-4 py-12 md:grid-cols-[0.9fr_1.1fr] md:items-center md:px-6">
        <div className="rounded-xl border border-black/10 bg-white p-4 shadow-sm">
          <ProductVisual product={products[0]} />
        </div>
        <div>
          <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Virtual try-on</p>
          <h2 className="mt-2 text-3xl font-black leading-tight text-neutral-950 md:text-4xl">
            Try frames online before placing your Cash on Delivery order.
          </h2>
          <p className="mt-4 max-w-2xl text-base leading-7 text-neutral-700">
            Upload or take a photo, place a frame on your face, adjust the fit,
            and choose with more confidence before checkout.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <Button href="/products/bk-blue-101" variant="secondary">
              Try On Frames
            </Button>
            <Button href="/products" variant="ghost">
              Explore Catalog
            </Button>
          </div>
        </div>
      </section>

      <section className="bg-[#f0f7f4] py-12">
        <div className="mx-auto max-w-7xl px-4 md:px-6">
          <div className="mb-7">
            <p className="text-sm font-black uppercase tracking-[0.2em] text-[#9a6a24]">Why Bongkey</p>
            <h2 className="mt-2 text-3xl font-black text-neutral-950">Built for confident eyewear shopping</h2>
          </div>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {trustItems.map((item) => (
              <TrustBadge key={item.title} title={item.title} text={item.text} />
            ))}
          </div>
        </div>
      </section>
    </>
  );
}
