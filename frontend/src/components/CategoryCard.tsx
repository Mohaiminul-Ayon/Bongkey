import Link from "next/link";

type CategoryCardProps = { title: string; text: string; index: number };

/** CategoryCard sends shoppers to the available catalog. */
export function CategoryCard({ title, text, index }: CategoryCardProps) {
  return (
    <Link className="category-card" href="/products">
      <span className="category-index">0{index} / Collection</span>
      <div><h3>{title}</h3><p>{text}</p></div>
      <span aria-hidden="true" className="category-arrow">↗</span>
    </Link>
  );
}
