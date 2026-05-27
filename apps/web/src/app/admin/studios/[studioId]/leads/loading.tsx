export default function LeadsLoading() {
  return (
    <div className="space-y-5 pb-10 animate-pulse">
      {/* Skeleton Header */}
      <div className="relative overflow-hidden rounded-[24px] border border-white/20 bg-white/20 px-6 py-4 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/20">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="h-10 w-10 rounded-2xl bg-zinc-200 dark:bg-neutral-800" />
            <div className="space-y-1.5">
              <div className="h-5 w-24 rounded bg-zinc-300 dark:bg-neutral-700" />
              <div className="h-3.5 w-36 rounded bg-zinc-200 dark:bg-neutral-800" />
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="h-6 w-16 rounded-full bg-zinc-200 dark:bg-neutral-800" />
            <div className="h-9 w-28 rounded-full bg-zinc-200 dark:bg-neutral-800" />
            <div className="h-9 w-32 rounded-full bg-zinc-200 dark:bg-neutral-800" />
          </div>
        </div>
      </div>

      {/* Skeleton Filters */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5 rounded-[22px] border border-white/20 bg-white/10 p-3 dark:border-white/5 dark:bg-neutral-900/10">
        <div className="h-10 rounded-xl bg-zinc-200 dark:bg-neutral-800" />
        <div className="h-10 rounded-xl bg-zinc-200 dark:bg-neutral-800" />
        <div className="h-10 rounded-xl bg-zinc-200 dark:bg-neutral-800" />
        <div className="h-10 rounded-xl bg-zinc-200 dark:bg-neutral-800" />
        <div className="h-10 rounded-xl bg-zinc-200 dark:bg-neutral-800" />
      </div>

      {/* Skeleton List Items */}
      <div className="rounded-[24px] border border-white/20 bg-white/20 p-5 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/20 space-y-4">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="flex flex-col md:flex-row md:items-center justify-between gap-4 p-4 rounded-2xl border border-white/10 bg-white/10 dark:bg-neutral-800/10">
            <div className="flex items-center gap-3.5">
              <div className="h-9 w-9 rounded-full bg-zinc-200 dark:bg-neutral-800" />
              <div className="space-y-1.5">
                <div className="h-4 w-32 rounded bg-zinc-300 dark:bg-neutral-700" />
                <div className="h-3 w-44 rounded bg-zinc-200 dark:bg-neutral-800" />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="h-6 w-20 rounded-full bg-zinc-200 dark:bg-neutral-800" />
              <div className="h-6 w-24 rounded-full bg-zinc-200 dark:bg-neutral-800" />
              <div className="h-6 w-16 rounded-full bg-zinc-200 dark:bg-neutral-800" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
