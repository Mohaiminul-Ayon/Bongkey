import type { OrderTracking } from "@/lib/api";

type OrderTimelineProps = {
  status: string;
  tracking: OrderTracking;
};

// The order's steps in the owner's status order; "preparing" is shown to customers as "Packed".
const steps = [
  ["pending", "Order placed", "placed_at"],
  ["confirmed", "Confirmed", "confirmed_at"],
  ["preparing", "Packed", "preparing_at"],
  ["shipped", "Shipped", "shipped_at"],
  ["delivered", "Delivered", "delivered_at"],
] as const;

/** formatStepDate shows a short Bangladesh date and time, or nothing for older orders without a saved time. */
function formatStepDate(value: string | null) {
  return value ? new Date(value).toLocaleString("en-BD", { day: "numeric", month: "short", hour: "numeric", minute: "2-digit" }) : "";
}

/** OrderTimeline shows how far an order has come, when each step happened, and the courier tracking. */
export function OrderTimeline({ status, tracking }: OrderTimelineProps) {
  if (status === "cancelled") {
    return <p className="rounded-md bg-red-50 p-4 font-bold text-red-700">This order was cancelled{tracking.cancelled_at ? ` on ${formatStepDate(tracking.cancelled_at)}` : ""}.</p>;
  }
  const reached = steps.findIndex(([value]) => value === status);
  return (
    <div className="grid gap-4">
      <ol className="grid grid-cols-5 gap-1 text-center text-xs" aria-label="Order progress">
        {steps.map(([value, label, field], index) => {
          const done = index <= reached;
          return (
            <li key={value} aria-current={index === reached ? "step" : undefined} className="grid justify-items-center gap-1">
              <span className={`grid h-8 w-8 place-items-center rounded-full border-2 text-sm font-black ${done ? "border-[#173c34] bg-[#173c34] text-white" : "border-neutral-300 text-neutral-400"}`}>{done ? "✓" : index + 1}</span>
              <span className={done ? "font-bold" : "text-neutral-500"}>{label}</span>
              <span className="text-neutral-500">{done ? formatStepDate(tracking[field]) : ""}</span>
            </li>
          );
        })}
      </ol>
      {tracking.courier || tracking.tracking_code ? (
        <p className="rounded-md bg-[#f0f7f4] p-3 text-sm text-[#173c34]">
          Courier: <strong>{tracking.courier || "—"}</strong>
          {tracking.tracking_code ? <> · Tracking code: <strong className="font-mono">{tracking.tracking_code}</strong></> : null}
          {tracking.tracking_url ? <> · <a className="font-bold underline" href={tracking.tracking_url} target="_blank" rel="noopener noreferrer">Track with the courier</a></> : null}
        </p>
      ) : null}
    </div>
  );
}
