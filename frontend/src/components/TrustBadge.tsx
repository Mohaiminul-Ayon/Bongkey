type TrustBadgeProps = {
  title: string;
  text: string;
};

export function TrustBadge({ title, text }: TrustBadgeProps) {
  return (
    <div className="rounded-lg border border-black/10 bg-white p-5 shadow-sm">
      <div className="mb-4 h-10 w-10 rounded-full bg-[#f0f7f4]" />
      <h3 className="font-black text-neutral-950">{title}</h3>
      <p className="mt-2 text-sm leading-6 text-neutral-600">{text}</p>
    </div>
  );
}
