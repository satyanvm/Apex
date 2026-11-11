import type { Metadata } from "next";
import { AuthForm } from "@/components/auth-form";
import { SiteNav } from "@/components/playground/site-nav";

export const metadata: Metadata = { title: "Sign in · Apex" };

export default function SignInPage() {
  return (
    <div className="min-h-dvh bg-bg">
      <SiteNav cta={false} account={false} />
      <main>
        <AuthForm mode="signin" />
      </main>
    </div>
  );
}
