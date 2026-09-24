import type { Product } from "@/lib/products";
import { API_URL } from "@/lib/api";

type ProductVisualProps = {
  product: Product;
  compact?: boolean;
};

export function ProductVisual({ product, compact = false }: ProductVisualProps) {
  const imageSrc = toImageSrc(product.imageUrl);

  return (
    <div
      className={`relative overflow-hidden rounded-lg bg-gradient-to-br ${product.imageTone} ${
        compact ? "aspect-[4/3]" : "aspect-[5/4]"
      }`}
    >
      {imageSrc ? (
        <img alt={product.name} className="h-full w-full object-contain p-4" src={imageSrc} />
      ) : (
        <>
          <div className="absolute inset-x-5 top-5 flex items-center justify-between text-[10px] font-black uppercase tracking-[0.16em] text-neutral-500">
            <span>{product.shape}</span>
            <span>{product.finish}</span>
          </div>
          <div className="absolute left-1/2 top-[52%] h-16 w-[76%] -translate-x-1/2 -translate-y-1/2">
            <div className="absolute left-0 top-1/2 h-16 w-[42%] -translate-y-1/2 rounded-[999px] border-[10px] border-neutral-950/85 bg-white/15 shadow-sm" />
            <div className="absolute right-0 top-1/2 h-16 w-[42%] -translate-y-1/2 rounded-[999px] border-[10px] border-neutral-950/85 bg-white/15 shadow-sm" />
            <div className="absolute left-1/2 top-1/2 h-6 w-[18%] -translate-x-1/2 -translate-y-1/2 border-t-[8px] border-neutral-950/85" />
            <div className="absolute -left-6 top-1/2 h-1 w-12 -translate-y-1/2 rounded-full bg-neutral-950/80" />
            <div className="absolute -right-6 top-1/2 h-1 w-12 -translate-y-1/2 rounded-full bg-neutral-950/80" />
          </div>
        </>
      )}
      <div className="absolute bottom-4 left-4 rounded-full bg-white/90 px-3 py-1 text-xs font-black text-[#173c34] shadow-sm">
        COD available
      </div>
    </div>
  );
}

function toImageSrc(url?: string) {
  if (!url) return "";
  if (url.startsWith("http")) return url;
  return `${API_URL}${url}`;
}
