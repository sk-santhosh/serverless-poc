import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Welcome Email Signup",
  description: "Queue-driven, pod-per-message serverless demo",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-50 text-slate-900">
        {children}
      </body>
    </html>
  );
}
