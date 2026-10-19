type TrustBadgeProps = { title: string; text: string; index: number };

/** TrustBadge explains one step of the shopping experience. */
export function TrustBadge({ title, text, index }: TrustBadgeProps) {
  return (
    <div className="shop-note">
      <span>0{index}</span>
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  );
}
