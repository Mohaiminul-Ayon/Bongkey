"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { ArrowLeft, Heart, LogOut, Package, Pencil, ShieldCheck, Truck } from "lucide-react";
import { CompleteProfileForm, EmailCodePasswordForm, PasswordSignInForm } from "@/components/AccountForms";
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

/** AccountPage shows sign-in / create account / reset password when signed out, asks for name and mobile
 * until the profile is complete, and then shows the profile. */
export default function AccountPage() {
  const { customer, ready, refresh, signedOut } = useCustomer();
  const [error, setError] = useState("");
  const [resetting, setResetting] = useState(false);

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
  if (customer && (!customer.name || !customer.phone)) {
    return (
      <section className="mx-auto max-w-md px-4 py-10 md:py-16">
        <Card>
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Finish your profile</CardTitle>
            <CardDescription>Your name and mobile number help us deliver and confirm your orders.</CardDescription>
          </CardHeader>
          <CardContent><CompleteProfileForm customer={customer} onSaved={refresh} /></CardContent>
        </Card>
      </section>
    );
  }
  if (customer) return <Profile customer={customer} onSignedOut={signedOut} onSaved={refresh} />;

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
          {resetting ? (
            <div className="grid gap-6">
              <button type="button" onClick={() => setResetting(false)} className="flex items-center gap-1 justify-self-start text-sm text-muted-foreground hover:text-foreground">
                <ArrowLeft aria-hidden="true" className="size-4" /> Back to sign in
              </button>
              <div className="grid gap-1.5 text-center">
                <h1 className="text-2xl font-bold">Reset your password</h1>
                <p className="text-sm text-muted-foreground">We will email you a code to prove it is you.</p>
              </div>
              <EmailCodePasswordForm mode="reset" onSignedIn={refresh} />
            </div>
          ) : (
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
              <OrDivider label="or with email" />
              <PasswordSignInForm onSignedIn={refresh} onForgot={() => setResetting(true)} />
            </TabsContent>
            <TabsContent value="signup" className="grid gap-6">
              <div className="grid gap-1.5 text-center">
                <h1 className="text-2xl font-bold">Create your account</h1>
                <p className="text-sm text-muted-foreground">Use Google, or your email and a password.</p>
              </div>
              <SignInMethod text="signup_with" onCredential={signIn} onError={setError} />
              <OrDivider label="or with email" />
              <EmailCodePasswordForm mode="signup" onSignedIn={refresh} />
            </TabsContent>
          </Tabs>
          )}

          {error ? <p role="alert" className="mt-4 text-center text-sm text-destructive">{error}</p> : null}

          <OrDivider label="or" />
          <div className="grid gap-2 sm:grid-cols-2">
            <Button asChild variant="outline"><Link href="/products">Continue as guest</Link></Button>
            <Button asChild variant="outline"><Link href="/track">Track an order</Link></Button>
          </div>
          <p className="mt-6 flex items-center justify-center gap-1.5 text-center text-xs text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="size-4" />
            We never store your password itself, only a secure hash. From Google we only receive your name and email.
          </p>
        </div>
      </Card>
    </section>
  );
}

/** OrDivider is the thin "or" line between sign-in choices. */
function OrDivider({ label }: { label: string }) {
  return (
    <div className="my-6 flex items-center gap-3 text-xs text-muted-foreground uppercase first:mt-0">
      <Separator className="flex-1" />
      {label}
      <Separator className="flex-1" />
    </div>
  );
}

/** SignInMethod shows Google's button, or a plain notice while the owner has not set up Google sign-in. */
function SignInMethod({ text, onCredential, onError }: { text: "signin_with" | "signup_with"; onCredential: (credential: string) => void; onError: (message: string) => void }) {
  if (!googleSignInEnabled) {
    return <p className="text-center text-sm text-muted-foreground">Google sign-in is not available yet.</p>;
  }
  return <GoogleSignInButton text={text} onCredential={onCredential} onError={onError} />;
}

/** Profile is the signed-in view: who you are, earlier orders to add, and My orders. */
function Profile({ customer, onSignedOut, onSaved }: { customer: Customer; onSignedOut: () => void; onSaved: () => Promise<unknown> }) {
  const [editing, setEditing] = useState(false);
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
            <p className="text-sm text-muted-foreground">{customer.phone}</p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setEditing(!editing)}><Pencil aria-hidden="true" />Edit</Button>
            <Button variant="outline" onClick={signOut}><LogOut aria-hidden="true" />Sign out</Button>
          </div>
        </CardContent>
        {editing ? (
          <CardContent className="border-t pt-6">
            <CompleteProfileForm customer={customer} onSaved={async () => { await onSaved(); setEditing(false); }} />
          </CardContent>
        ) : null}
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
