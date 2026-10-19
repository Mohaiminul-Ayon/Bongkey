type BrandLogoProps = {
  size?: "sm" | "lg";
};

/** BrandLogo shows the owner's original wordmark without altering its artwork. */
export function BrandLogo({ size = "sm" }: BrandLogoProps) {
  return (
    <span className={`brand-logo brand-logo--${size}`}>
      <img alt="Bongkey — see life clearly" src="/images/bongkey-logo.png" />
    </span>
  );
}
