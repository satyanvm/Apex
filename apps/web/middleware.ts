import { NextResponse, type NextRequest } from "next/server";

/**
 * Access control for a hosted deployment.
 *
 * - Anyone can open the landing page, the playground (which compiles through
 *   /api/public/*) and the sign-in pages.
 * - The console needs an account. Without a session cookie, its pages redirect
 *   to /signin?next=…; the Go API checks the session itself on every console
 *   API call, so a forged or expired cookie gets no further than a 401.
 * - AGENTLY_API_TOKEN is attached server-side to every proxied console /api
 *   request so the Go API accepts it. The browser never sees the token.
 *
 * Webhook ingress (/api/hooks/*) stays open: third-party providers call it.
 */
const PUBLIC_PAGES = new Set(["/", "/playground", "/signin", "/signup", "/icon.svg"]);
const SESSION_COOKIE = "apex_session";

function isPublicApi(pathname: string): boolean {
  return (
    pathname === "/api/health" ||
    pathname.startsWith("/api/public/") ||
    pathname.startsWith("/api/auth/") ||
    pathname.startsWith("/api/hooks/")
  );
}

export function middleware(req: NextRequest) {
  const { pathname, search } = req.nextUrl;

  if (!pathname.startsWith("/api/")) {
    if (PUBLIC_PAGES.has(pathname) || req.cookies.has(SESSION_COOKIE)) return NextResponse.next();
    const url = req.nextUrl.clone();
    url.pathname = "/signin";
    url.search = `?next=${encodeURIComponent(pathname + search)}`;
    return NextResponse.redirect(url);
  }

  // Public routes get no API token, so a path that slips past isPublicApi still
  // meets the API's own checks.
  const headers = new Headers(req.headers);
  const token = process.env.AGENTLY_API_TOKEN;
  if (token && !isPublicApi(pathname)) headers.set("authorization", `Bearer ${token}`);
  else headers.delete("authorization");
  return NextResponse.next({ request: { headers } });
}

export const config = {
  runtime: "nodejs",
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
