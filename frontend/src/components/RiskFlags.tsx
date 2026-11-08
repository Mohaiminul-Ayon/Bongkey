/** flagText explains each checkout red flag in words staff can act on during the confirmation call. */
const flagText: Record<string, string> = {
  disposable_email: "Throwaway email address",
  short_address: "Very short or unclear address",
  odd_name: "Name looks odd (digits or too short)",
  repeat_phone: "Same phone: several orders today",
  phone_other_name: "Same phone used with a different name",
  high_value_cod: "High-value cash on delivery",
  fast_submit: "Form sent unusually fast",
};

/** RiskFlags shows an order's red flags in red; they are reasons to look twice, not reasons to refuse. */
export function RiskFlags({ flags }: { flags?: string[] | null }) {
  if (!flags?.length) return null;
  return (
    <ul aria-label="Red flags" className="mt-2 flex flex-wrap gap-2">
      {flags.map((flag) => (
        <li key={flag} className="rounded-full bg-red-50 px-3 py-1 text-xs font-bold text-red-700 ring-1 ring-red-200">⚠ {flagText[flag] ?? flag}</li>
      ))}
    </ul>
  );
}
