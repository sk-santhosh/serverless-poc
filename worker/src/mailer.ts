import nodemailer from "nodemailer";
import { requireEnv } from "./redis";

/**
 * Generic SMTP transport — works against SES SMTP, SendGrid SMTP, a local
 * maildev container, or any other provider. No provider-specific SDK, so
 * there's no vendor lock-in.
 */
function createTransport() {
  const host = requireEnv("SMTP_HOST");
  const port = Number(process.env.SMTP_PORT ?? "587");
  const user = process.env.SMTP_USER;
  const pass = process.env.SMTP_PASS;

  return nodemailer.createTransport({
    host,
    port,
    secure: port === 465,
    // Auth is optional so this also works against unauthenticated local
    // test servers like maildev.
    auth: user && pass ? { user, pass } : undefined,
  });
}

export async function sendWelcomeEmail(
  name: string,
  email: string
): Promise<void> {
  const fromEmail = requireEnv("FROM_EMAIL");
  const transport = createTransport();

  await transport.sendMail({
    from: fromEmail,
    to: email,
    subject: "Welcome!",
    text: `Hi ${name},\n\nThanks for signing up — we're glad you're here!\n\nBest,\nThe Team`,
  });
}
