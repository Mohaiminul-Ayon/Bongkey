const steps = ["Bag", "Details", "Confirmed"];

type CheckoutStepsProps = { current: 1 | 2 | 3 };

/** CheckoutSteps shows where the shopper is in the bag → details → confirmation flow. */
export function CheckoutSteps({ current }: CheckoutStepsProps) {
  return (
    <ol aria-label="Checkout progress" className="checkout-steps">
      {steps.map((label, index) => (
        <li key={label} aria-current={index + 1 === current ? "step" : undefined} data-done={index + 1 < current || undefined}>
          <span>{index + 1 < current ? "✓" : index + 1}</span>{label}
        </li>
      ))}
    </ol>
  );
}
