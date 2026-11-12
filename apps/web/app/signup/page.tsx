import type { Metadata } from "next";
import { AuthForm } from "@/components/auth-form";
import { SiteNav } from "@/components/playground/site-nav";

export const metadata: Metadata = { title: "Create an account · Apex" };

export default function SignUpPage() {
  return (
    <div className="min-h-dvh bg-bg">
      <SiteNav cta={false} account={false} />
      <main>
        <AuthForm mode="signup" />
      </main>
    </div>
  );
}
