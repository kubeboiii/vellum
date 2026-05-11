// Phase 1 placeholder page.
// Phase 5 replaces this with the live incident feed (FR-7.1 in 00-master-prd).
export default function Home() {
  return (
    <main className="min-h-screen flex flex-col items-center justify-center p-8 gap-4 font-mono">
      <h1 className="text-3xl font-bold">IMS — Incident Management System</h1>
      <p className="text-sm text-gray-600 dark:text-gray-400">
        Phase 1 (Foundation). Live feed arrives in Phase 5.
      </p>
      <a
        href="http://localhost:8080/health"
        className="text-sm underline text-blue-600 dark:text-blue-400"
      >
        Backend /health →
      </a>
    </main>
  );
}
