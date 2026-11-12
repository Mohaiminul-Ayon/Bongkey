"use client";

export const orderStatusLabel: Record<string, string> = {
  pending: "Pending",
  confirmed: "Confirmed",
  preparing: "Preparing",
  shipped: "Shipped",
  delivered: "Delivered",
  cancelled: "Cancelled",
};

// nextStatuses mirrors the backend state machine; the server still checks every change.
const nextStatuses: Record<string, string[]> = {
  pending: ["confirmed", "cancelled"],
  confirmed: ["preparing", "cancelled"],
  preparing: ["shipped"],
  shipped: ["delivered"],
};

type Props = {
  order: { order_number: string; order_status: string; payment_status: string };
  busy: boolean;
  /** onChange performs the status change; the parent owns the request and the error message. */
  onChange: (status: string) => void;
};

/** OrderStatusActions shows the next fulfilment steps for one order, asking first before a cancel.
 * Confirm waits while a bKash payment is due, so staff don't press a button the server will refuse. */
export function OrderStatusActions({ order, busy, onChange }: Props) {
  const waitingForPayment = order.payment_status === "advance_due";
  const transitions = (nextStatuses[order.order_status] ?? []).filter((status) => !(status === "confirmed" && waitingForPayment));

  // choose asks before a cancel because it restores stock and cannot be undone.
  function choose(status: string) {
    if (status === "cancelled" && !window.confirm(`Cancel ${order.order_number} and restore its stock?`)) return;
    onChange(status);
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      {transitions.map((status) => (
        <button key={status} type="button" disabled={busy} onClick={() => choose(status)}
          className={status === "cancelled" ? "rounded-md border border-red-300 px-3 py-2 text-sm font-bold text-red-700 disabled:opacity-50" : "rounded-md bg-neutral-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"}>
          {status === "cancelled" ? "Cancel order" : `Mark ${orderStatusLabel[status]}`}
        </button>
      ))}
      {order.order_status === "pending" && waitingForPayment ? <span className="text-sm font-semibold text-[#7b531c]">Confirm after the bKash payment is approved</span> : null}
      {!nextStatuses[order.order_status] ? <span className="text-sm font-semibold text-neutral-500">No further fulfillment action</span> : null}
    </div>
  );
}
