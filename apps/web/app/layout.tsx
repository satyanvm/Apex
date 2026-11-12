import "./globals.css";
import type { Metadata } from "next";
import { GeistSans } from "geist/font/sans";
import { GeistMono } from "geist/font/mono";

export const metadata: Metadata = {
  title: "Apex: prompts in, durable workflows out",
  description:
    "Apex compiles a plain-English request into a validated workflow graph over 6,758 real integration actions, then runs it durably on Temporal.",
  metadataBase: new URL(process.env.SITE_URL || "http://localhost:3000"),
};

// Runs before paint to apply the saved/system theme with no flash of the
// wrong mode. Kept as a tiny string so it can be inlined in <head>.
const themeInit = `(function(){try{var t=localStorage.getItem('theme');var d=t?t==='dark':window.matchMedia('(prefers-color-scheme: dark)').matches;if(d)document.documentElement.classList.add('dark');}catch(e){}})();`;

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      className={`${GeistSans.variable} ${GeistMono.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInit }} />
      </head>
      <body className="min-h-screen antialiased">{children}</body>
    </html>
  );
}
