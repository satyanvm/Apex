import type { Metadata } from "next";
import { Playground } from "@/components/playground/playground";
import { SiteNav } from "@/components/playground/site-nav";

export const metadata: Metadata = {
  title: "Compiler playground · Apex",
  description: "Type an automation in plain English and watch Apex compile it into a validated workflow graph.",
};

export default function PlaygroundPage() {
  return (
    <div className="min-h-dvh bg-bg">
      <SiteNav cta={false} />
      <main>
        <h1 className="sr-only">Apex compiler playground</h1>
        <Playground />
      </main>
    </div>
  );
}
