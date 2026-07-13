"use client";

import { useState, type FormEvent } from "react";

type SubmitState =
  | { status: "idle" }
  | { status: "submitting" }
  | { status: "success"; requestId: string }
  | { status: "error"; message: string };

export default function HomePage() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [state, setState] = useState<SubmitState>({ status: "idle" });

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setState({ status: "submitting" });

    try {
      const res = await fetch("/api/subscribe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, email }),
      });

      if (res.status === 202) {
        const data = (await res.json()) as { requestId: string };
        setState({ status: "success", requestId: data.requestId });
        setName("");
        setEmail("");
        return;
      }

      const data = (await res.json().catch(() => null)) as {
        error?: string;
      } | null;
      setState({
        status: "error",
        message: data?.error ?? "Something went wrong. Please try again.",
      });
    } catch {
      setState({
        status: "error",
        message: "Could not reach the server. Please try again.",
      });
    }
  }

  const submitting = state.status === "submitting";

  return (
    <main className="flex min-h-screen items-center justify-center p-6">
      <div className="w-full max-w-sm rounded-xl border border-slate-200 bg-white p-8 shadow-sm">
        <h1 className="mb-1 text-xl font-semibold">Get a welcome email</h1>
        <p className="mb-6 text-sm text-slate-500">
          Submit your name and email — a worker will send you a welcome
          message shortly.
        </p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label
              htmlFor="name"
              className="mb-1 block text-sm font-medium text-slate-700"
            >
              Name
            </label>
            <input
              id="name"
              name="name"
              type="text"
              required
              minLength={1}
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={submitting}
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none disabled:bg-slate-100"
              placeholder="Ada Lovelace"
            />
          </div>

          <div>
            <label
              htmlFor="email"
              className="mb-1 block text-sm font-medium text-slate-700"
            >
              Email
            </label>
            <input
              id="email"
              name="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={submitting}
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none disabled:bg-slate-100"
              placeholder="ada@example.com"
            />
          </div>

          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:bg-slate-400"
          >
            {submitting ? "Submitting..." : "Submit"}
          </button>
        </form>

        {state.status === "success" && (
          <p className="mt-4 rounded-md bg-green-50 px-3 py-2 text-sm text-green-700">
            Thanks! Your request ({state.requestId.slice(0, 8)}...) was
            queued.
          </p>
        )}
        {state.status === "error" && (
          <p className="mt-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">
            {state.message}
          </p>
        )}
      </div>
    </main>
  );
}
