"use client";

import Link from "next/link";
import { type DragEvent, FormEvent, type ReactNode, useEffect, useMemo, useState } from "react";
import { AlertTriangle, ArrowLeftRight, CircleCheck, ImagePlus, Info, LoaderCircle, PackageX, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Textarea } from "@/components/ui/textarea";
import type { ReturnKind } from "@/lib/api";
import { maxPrescriptionBytes } from "@/lib/prescriptionFiles";
import { whatsAppLink, whatsAppNumber } from "@/lib/whatsapp";

const dayMs = 24 * 60 * 60 * 1000;
const maxPhotos = 3;
const maxMessage = 1000;
// Photos only (the server's private store also takes PDFs, but a return needs pictures of the frame).
const photoTypes = ["image/jpeg", "image/png", "image/heic"];
const photoAccept = ".jpg,.jpeg,.png,.heic,image/jpeg,image/png,image/heic";

// The same windows the server enforces (returns.Window); the server decides, this only hides closed choices.
const kinds: { kind: ReturnKind; label: string; hint: string; days: number; icon: typeof ArrowLeftRight }[] = [
  { kind: "exchange", label: "Exchange for another frame", hint: "Change of mind. A pricier frame means you pay the difference.", days: 3, icon: ArrowLeftRight },
  { kind: "defect", label: "Defect or colour problem", hint: "Something is broken, loose or not the colour shown.", days: 15, icon: AlertTriangle },
  { kind: "wrong_item", label: "Wrong item received", hint: "The parcel had a different frame from your order.", days: 15, icon: PackageX },
];

type ReturnRequestFormProps = {
  /** orderNumber is known on order pages; the standalone request page asks for it in children instead */
  orderNumber?: string;
  /** status and deliveredAt hide the form before delivery and hide choices whose time is over. Without them
   * (the standalone page) every choice shows and the server alone decides. */
  status?: string;
  deliveredAt?: string | null;
  /** send posts the form (kind, message, photos) through the page's own order access */
  send: (form: FormData) => Promise<{ id: number }>;
  /** children are extra fields at the top of the form (order number and phone on the request page) */
  children?: ReactNode;
};

/** ReturnRequestForm lets a customer open a return or exchange request after delivery. Staff then call or chat;
 * nothing is refunded or swapped automatically. */
export function ReturnRequestForm({ orderNumber, status, deliveredAt, send, children }: ReturnRequestFormProps) {
  const standalone = status === undefined;
  const [open, setOpen] = useState(standalone);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [sent, setSent] = useState<{ id: number; order: string } | null>(null);
  const [message, setMessage] = useState("");
  const [photos, setPhotos] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);
  // Read the clock once per render; the windows are days long, so seconds of drift do not matter.
  const [now] = useState(() => Date.now());
  const since = deliveredAt ? now - new Date(deliveredAt).getTime() : 0;
  const available = kinds.filter((item) => since <= item.days * dayMs);
  const [kind, setKind] = useState<ReturnKind>(available[0]?.kind ?? "exchange");

  // Preview addresses live only while their photo is listed; revoking them frees the browser's memory.
  const previews = useMemo(() => photos.map((file) => (file.type === "image/heic" ? "" : URL.createObjectURL(file))), [photos]);
  useEffect(() => () => previews.forEach((url) => url && URL.revokeObjectURL(url)), [previews]);

  if (!standalone && (status !== "delivered" || !deliveredAt)) return null;
  const policyLink = <Link href="/return-policy" className="font-medium text-foreground underline underline-offset-4">return &amp; exchange policy</Link>;

  if (available.length === 0) {
    return <p className="rounded-xl border border-border p-4 text-sm">The return time for this order is over (see our {policyLink}). Questions? {whatsAppNumber ? <a className="font-medium underline" target="_blank" rel="noopener noreferrer" href={whatsAppLink(`Hi Bongkey, I have a question about order ${orderNumber}.`)}>Message us on WhatsApp</a> : "Call us."}</p>;
  }

  /** addPhotos keeps up to three JPG, PNG or HEIC photos of 5 MB or less; the server checks the content again. */
  function addPhotos(files: File[]) {
    if (files.length === 0) return;
    if (files.some((file) => !photoTypes.includes(file.type) && !/\.(jpe?g|png|heic)$/i.test(file.name))) {
      setError("Photos must be JPG, PNG or HEIC.");
      return;
    }
    if (files.some((file) => file.size > maxPrescriptionBytes)) {
      setError("Each photo must be 5 MB or smaller.");
      return;
    }
    if (photos.length + files.length > maxPhotos) {
      setError(`Add up to ${maxPhotos} photos.`);
      return;
    }
    setError("");
    setPhotos((current) => [...current, ...files]);
  }

  function drop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    setDragging(false);
    addPhotos(Array.from(event.dataTransfer.files));
  }

  // submit sends the request with the chosen reason and photos; the server checks everything again.
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    form.set("kind", kind);
    form.set("message", message);
    form.delete("photos");
    photos.forEach((file) => form.append("photos", file));
    setBusy(true);
    setError("");
    try {
      const order = orderNumber ?? String(form.get("order_number") ?? "").trim();
      setSent({ id: (await send(form)).id, order });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not send your request.");
    } finally {
      setBusy(false);
    }
  }

  if (sent !== null) {
    return (
      <Card role="status" className="gap-3 text-sm">
        <CardHeader className="flex items-center gap-3">
          <CircleCheck aria-hidden="true" className="size-6 shrink-0 text-green-700" />
          <CardTitle className="text-base">Request #{sent.id} sent</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <p className="text-muted-foreground">We will call you to understand the problem and agree a solution.</p>
          {whatsAppNumber ? <Button asChild variant="outline" className="w-fit"><a target="_blank" rel="noopener noreferrer" href={whatsAppLink(`Hi Bongkey, I opened return request #${sent.id} for order ${sent.order}.`)}>Chat with us on WhatsApp</a></Button> : null}
        </CardContent>
      </Card>
    );
  }

  if (!open) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border p-4 text-sm">
        <span>Need an exchange or found a problem? See our {policyLink}.</span>
        <Button type="button" variant="outline" onClick={() => setOpen(true)}>Request a return or exchange</Button>
      </div>
    );
  }

  return (
    <Card>
      <form onSubmit={submit} className="grid gap-6">
        {standalone ? null : (
          <CardHeader>
            <CardTitle className="text-lg">Return or exchange request</CardTitle>
            <CardDescription>Tell us what happened. We call you to agree a solution.</CardDescription>
          </CardHeader>
        )}
        <CardContent className="grid gap-6">
          {children}

          <div className="grid gap-3">
            <Label id="return-kind-label">What do you need?</Label>
            <RadioGroup aria-labelledby="return-kind-label" value={kind} onValueChange={(value) => setKind(value as ReturnKind)} className="grid gap-3 sm:grid-cols-3">
              {available.map(({ kind: value, label, hint, days, icon: Icon }) => (
                <label key={value} htmlFor={`return-kind-${value}`}
                  className="flex cursor-pointer flex-col gap-2 rounded-lg border border-border p-4 text-sm transition-colors hover:bg-muted/50 has-[[data-state=checked]]:border-foreground has-[[data-state=checked]]:bg-muted/50 has-[:focus-visible]:ring-[3px] has-[:focus-visible]:ring-ring/50">
                  {/* The radio is hidden but still focusable; the card's border shows the choice. */}
                  <RadioGroupItem id={`return-kind-${value}`} value={value} className="sr-only" />
                  <Icon aria-hidden="true" className="size-5 text-muted-foreground" strokeWidth={1.6} />
                  <span className="font-medium leading-snug">{label}</span>
                  <span className="text-xs text-muted-foreground">{hint}</span>
                  <Badge variant="secondary" className="mt-auto">Within {days} days</Badge>
                </label>
              ))}
            </RadioGroup>
          </div>

          <div className="grid gap-2">
            <div className="flex items-baseline justify-between gap-2">
              <Label htmlFor="return-message">Tell us what happened</Label>
              <span className="text-xs text-muted-foreground" aria-live="polite">{message.length}/{maxMessage}</span>
            </div>
            <Textarea id="return-message" required minLength={5} maxLength={maxMessage} value={message} onChange={(event) => setMessage(event.target.value)}
              className="min-h-28" placeholder="e.g. The left hinge is loose. Or: I'd like the same frame in black." />
          </div>

          <div className="grid gap-2">
            <div className="flex items-baseline justify-between gap-2">
              <Label htmlFor="return-photos">Photos <span className="font-normal text-muted-foreground">(optional)</span></Label>
              <span className="text-xs text-muted-foreground">{photos.length}/{maxPhotos}</span>
            </div>
            {photos.length > 0 ? (
              <ul className="flex flex-wrap gap-3">
                {photos.map((file, index) => (
                  <li key={`${file.name}-${index}`} className="relative size-20 overflow-hidden rounded-md border border-border bg-muted">
                    {previews[index] ? <img src={previews[index]} alt={`Photo ${index + 1}: ${file.name}`} className="size-full object-cover" />
                      : <span className="grid size-full place-items-center p-1 text-center text-[10px] break-all text-muted-foreground">{file.name}</span>}
                    <button type="button" aria-label={`Remove photo ${index + 1}`} onClick={() => setPhotos((current) => current.filter((_, i) => i !== index))}
                      className="absolute top-1 right-1 grid size-6 place-items-center rounded-full bg-background/90 shadow-sm"><X aria-hidden="true" className="size-3.5" /></button>
                  </li>
                ))}
              </ul>
            ) : null}
            {photos.length < maxPhotos ? (
              <label htmlFor="return-photos" onDragOver={(event) => { event.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={drop}
                className={`flex cursor-pointer flex-col items-center gap-1 rounded-lg border-2 border-dashed p-5 text-center text-sm transition-colors focus-within:ring-[3px] focus-within:ring-ring/50 hover:bg-muted/50 ${dragging ? "border-foreground bg-muted/50" : "border-border"}`}>
                <ImagePlus aria-hidden="true" className="size-6 text-muted-foreground" strokeWidth={1.6} />
                <span className="font-medium">Add photos</span>
                <span className="text-xs text-muted-foreground">Tap to choose, or drag them here. JPG, PNG or HEIC, up to 5 MB each.</span>
                <input id="return-photos" type="file" multiple accept={photoAccept} className="sr-only"
                  onChange={(event) => { addPhotos(Array.from(event.target.files ?? [])); event.target.value = ""; }} />
              </label>
            ) : null}
          </div>

          <p className="flex gap-2 rounded-lg bg-muted p-3 text-sm text-muted-foreground">
            <Info aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>Lenses are made for you and are not refunded. After the delivery person leaves, the courier charge for sending it back is yours. Full rules: {policyLink}.</span>
          </p>
          {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
        <CardFooter className="flex flex-wrap gap-2">
          <Button type="submit" size="lg" disabled={busy} className="w-full sm:w-auto">
            {busy ? <><LoaderCircle aria-hidden="true" className="animate-spin" />Sending...</> : "Send request"}
          </Button>
          {standalone ? null : <Button type="button" variant="ghost" size="lg" onClick={() => setOpen(false)}>Cancel</Button>}
        </CardFooter>
      </form>
    </Card>
  );
}
