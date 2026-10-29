import Link from "next/link";

type CategoryCardProps = { title: string; text: string; href: string };

/** CategoryCard sends shoppers to one slice of the catalog. */
export function CategoryCard({ title, text, href }: CategoryCardProps) {
  return (
    <Link className="category-card" href={href}>
      <div><h3>{title}</h3><p>{text}</p></div>
      <span aria-hidden="true" className="category-arrow">→</span>
    </Link>
  );
}
