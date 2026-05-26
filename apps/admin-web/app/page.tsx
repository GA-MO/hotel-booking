// Phase 1 will redirect to /dashboard once authed, /login otherwise.
export default function AdminHomePage() {
  return (
    <main className="mx-auto flex min-h-screen max-w-2xl flex-col items-center justify-center px-6 text-center">
      <h1 className="text-3xl font-semibold tracking-tight">Hotel Admin</h1>
      <p className="mt-3 text-neutral-600">Sign in to manage your property.</p>
    </main>
  );
}
