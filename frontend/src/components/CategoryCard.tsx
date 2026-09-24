import Link from "next/link";

type CategoryCardProps = {
  title: string;
  text: string;
};

export function CategoryCard({ title, text }: CategoryCardProps) {
  return (
    <Link
      href="/products"
      className="group rounded-lg border border-black/10 bg-white p-5 shadow-sm transition hover:-translate-y-0.5 hover:border-[#173c34]"
    >
      <div className="mb-5 aspect-[5/3] rounded-md bg-[linear-gradient(135deg,#f7f5ef,#ffffff_48%,#d8b267)]" />
      <h3 className="text-lg font-black text-neutral-950 group-hover:text-[#173c34]">{title}</h3>
      <p className="mt-2 text-sm leading-6 text-neutral-600">{text}</p>
    </Link>
  );
}
