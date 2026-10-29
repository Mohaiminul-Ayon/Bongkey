type BrandLogoProps = {
  size?: "sm" | "lg";
};

/** BrandLogo draws the Bongkey monogram and wordmark in the current text color so it follows light and dark mode. */
export function BrandLogo({ size = "sm" }: BrandLogoProps) {
  return (
    <svg aria-label="Bongkey — see life clearly" className={`brand-logo brand-logo--${size}`} role="img" viewBox="0 0 220 64">
      <circle cx="30" cy="32" r="26" fill="currentColor" />
      <text className="brand-logo-mark" x="30" y="43" textAnchor="middle">b</text>
      <text className="brand-logo-name" x="66" y="32">BONGKEY</text>
      <text className="brand-logo-tagline" x="67" y="50">see life clearly</text>
    </svg>
  );
}
