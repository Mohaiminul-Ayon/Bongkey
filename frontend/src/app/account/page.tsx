"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Heart, LogOut, Package, ShieldCheck, Truck } from "lucide-react";
import { GoogleSignInButton, googleSignInEnabled } from "@/components/GoogleSignInButton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  fetchCustomerOrders,
  fetchLinkCandidates,
  linkEarlierOrders,
  signInWithGoogle,
  signOutCustomer,
  type Customer,
  type CustomerOrder,
} from "@/lib/api";
import { initials, useCustomer } from "@/lib/customer";
import { formatPrice } from "@/lib/products";

const statusNames: Record<string, string> = {
  pending: "Order placed",
  confirmed: "Confirmed",
  preparing: "Packed",
  shipped: "Shipped",
  delivered: "Delivered",
  cancelled: "Cancelled",
};

const perks = [
  { icon: Package, text: "All your orders and their status in one place" },
  { icon: Truck, text: "New orders are saved to your profile automatically" },
  { icon: Heart, text: "Faster checkout next time" },
];

/** AccountPage shows sign-in (and sign-up, which is the same Google step) when signed out, and the profile when signed in. */
export default function AccountPage() {
  const { customer, ready, refresh, signedOut } = useCustomer();
  const [error, setError] = useState("");

  /** signIn sends Google's ID token to the backend, which sets the session cookie; the shared state then updates the header too. */
  async function signIn(credential: string) {
    setError("");
    try {
      await signInWithGoogle(credential);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Google sign-in failed.");
    }
  }

  if (!ready) return <section className="mx-auto max-w-4xl px-4 py-16 text-center text-muted-foreground">Loading…</section>;
  if (customer) return <Profile customer={customer} onSignedOut={signedOut} />;

  return (
    <section className="mx-auto max-w-4xl px-4 py-10 md:py-16">
      <Card className="grid gap-0 overflow-hidden p-0 md:grid-cols-2">
        <div className="hidden flex-col justify-between bg-primary p-10 text-primary-foreground md:flex">
          <div>
            <p className="text-sm font-semibold tracking-[0.2em] uppercase opacity-70">Bongkey</p>
            <h2 className="mt-4 text-3xl font-bold leading-tight">See life clearly, and keep your orders close.</h2>
          </div>
          <ul className="grid gap-4">
            {perks.map(({ icon: Icon, text }) => (
              <li key={text} className="flex items-start gap-3 text-sm">
                <Icon aria-hidden="true" className="mt-0.5 size-5 shrink-0 opacity-80" />
                <span>{text}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="p-6 sm:p-10">
          <Tabs defaultValue="signin" className="gap-6">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="signin">Sign in</TabsTrigger>
              <TabsTrigger value="signup">Create account</TabsTrigger>
            </TabsList>
            <TabsContent value="signin" className="grid gap-6">
              <div className="grid gap-1.5 text-center">
                <h1 className="text-2xl font-bold">Welcome back</h1>
                <p className="text-sm text-muted-foreground">Sign in to see your orders and checkout faster.</p>
              </div>
              <SignInMethod text="signin_with" onCredential={signIn} onError={setError} />
            </TabsContent>
            <TabsContent value="signup" className="grid gap-6">
              <div className="grid gap-1.5 text-center">
                <h1 className="text-2xl font-bold">Create your account</h1>
                <p className="text-sm text-muted-foreground">One click with Google. No new password to remember.</p>
              </div>
              <SignInMethod text="signup_with" onCredential={signIn} onError={setError} />
            </TabsContent>
          </Tabs>

          {error ? <p role="alert" className="mt-4 text-center text-sm text-destructive">{error}</p> : null}

          <div className="my-6 flex items-center gap-3 text-xs text-muted-foreground uppercase">
            <Separator className="flex-1" />
            or
            <Separator className="flex-1" />
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            <Button asChild variant="outline"><Link href="/products">Continue as guest</Link></Button>
            <Button asChild variant="outline"><Link href="/track">Track an order</Link></Button>
          </div>
          <p className="mt-6 flex items-center justify-center gap-1.5 text-center text-xs text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="size-4" />
            We only receive your name and email from Google.
          </p>
        </div>
      </Card>
    </section>
  );
}

/** SignInMethod shows Google's button, or a plain notice while the owner has not set up Google sign-in. */
function SignInMethod({ text, onCredential, onError }: { text: "signin_with" | "signup_with"; onCredential: (credential: string) => void; onError: (message: string) => void }) {
  if (!googleSignInEnabled) {
    return <p className="text-center text-sm text-muted-foreground">Sign-in is not available yet. You can still order as a guest.</p>;
  }
  return <GoogleSignInButton text={text} onCredential={onCredential} onError={onError} />;
}

/** Profile is the signed-in view: who you are, earlier orders to add, and My orders. */
function Profile({ customer, onSignedOut }: { customer: Customer; onSignedOut: () => void }) {
  const [orders, setOrders] = useState<CustomerOrder[] | null>(null);
  const [candidates, setCandidates] = useState(0);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  /** load reads the linked orders and how many earlier guest orders share this email. */
  const load = useCallback(async () => {
    try {
      const [list, link] = await Promise.all([fetchCustomerOrders(), fetchLinkCandidates()]);
      setOrders(list);
      setCandidates(link.count);
    } catch (err) {
      setOrders([]);
      setError(err instanceof Error ? err.message : "Could not load your orders.");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  /** signOut ends the session on the server, not just in this tab. */
  async function signOut() {
    await signOutCustomer().catch(() => undefined);
    onSignedOut();
  }

  /** link sends the typed phone; only orders with this email and this phone are added. */
  async function link(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const phone = String(new FormData(event.currentTarget).get("phone") ?? "");
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const { linked } = await linkEarlierOrders(phone);
      setMessage(linked > 0 ? `Added ${linked} order${linked === 1 ? "" : "s"} to your profile.` : "No orders matched this phone number.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add the orders.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="mx-auto grid max-w-4xl gap-6 px-4 py-10 md:py-16">
      <Card>
        <CardContent className="flex flex-wrap items-center gap-4">
          <span aria-hidden="true" className="grid size-16 place-items-center rounded-full bg-primary text-xl font-bold text-primary-foreground">{initials(customer)}</span>
          <div className="min-w-0 flex-1">
            <h1 className="truncate text-2xl font-bold">{customer.name || "Your profile"}</h1>
            <p className="truncate text-sm text-muted-foreground">{customer.email}</p>
            <Badge variant="secondary" className="mt-2">Signed in with Google</Badge>
          </div>
          <Button variant="outline" onClick={signOut}><LogOut aria-hidden="true" />Sign out</Button>
        </CardContent>
      </Card>

      {candidates > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>We found {candidates} earlier order{candidates === 1 ? "" : "s"} with your email</CardTitle>
            <CardDescription>Type the phone number you used on them. Only orders with both your email and this phone are added.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={link} className="flex flex-wrap gap-3">
              <Input name="phone" required inputMode="tel" placeholder="01XXXXXXXXX" aria-label="Phone used on the orders" className="max-w-xs" />
              <Button type="submit" disabled={busy}>{busy ? "Checking…" : "Add my orders"}</Button>
            </form>
          </CardContent>
        </Card>
      ) : null}
      {message ? <p role="status" className="text-sm font-semibold text-green-700 dark:text-green-400">{message}</p> : null}
      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}

      <Card>
        <CardHeader>
          <CardTitle>My orders</CardTitle>
          <CardDescription>For delivery details, open <Link className="underline underline-offset-4" href="/track">Track order</Link> with the order number.</CardDescription>
        </CardHeader>
        <CardContent>
          {orders === null ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : orders.length === 0 ? (
            <div className="grid justify-items-center gap-3 py-8 text-center">
              <Package aria-hidden="true" className="size-8 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">No orders yet.</p>
              <Button asChild><Link href="/products">Start shopping</Link></Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Order</TableHead>
                  <TableHead>Date</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((order) => (
                  <TableRow key={order.id}>
                    <TableCell className="font-medium">{order.order_number}</TableCell>
                    <TableCell>{new Date(order.created_at).toLocaleDateString("en-BD", { day: "numeric", month: "short", year: "numeric" })}</TableCell>
                    <TableCell>
                      <Badge variant={order.order_status === "cancelled" ? "outline" : "secondary"}>{statusNames[order.order_status] ?? order.order_status}</Badge>
                    </TableCell>
                    <TableCell className="text-right font-medium">{formatPrice(order.total)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </section>
  );
}
