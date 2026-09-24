import { AdminGuard } from "@/components/AdminGuard";
import { AdminNav } from "@/components/AdminNav";
import { ProductForm } from "@/components/ProductForm";

export default function NewProductPage() {
  return <AdminGuard><section className="mx-auto max-w-5xl px-4 py-8 md:px-6"><AdminNav /><h1 className="mb-6 text-3xl font-black">Add new product</h1><ProductForm /></section></AdminGuard>;
}
