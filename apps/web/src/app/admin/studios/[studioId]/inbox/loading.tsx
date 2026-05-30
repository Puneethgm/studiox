export default function InboxLoading() {
  return (
    <div className="space-y-4 animate-pulse">
      {/* Compact glass header */}
      <div className="relative overflow-hidden rounded-[22px] border border-white/20 bg-white/30 px-5 py-3.5 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl bg-zinc-200 dark:bg-neutral-800" />
            <div className="space-y-1.5">
              <div className="h-4.5 w-16 rounded bg-zinc-300 dark:bg-neutral-700" />
              <div className="h-3 w-28 rounded bg-zinc-200 dark:bg-neutral-800" />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <div className="h-6 w-14 rounded-full bg-zinc-200 dark:bg-neutral-800" />
            <div className="h-6 w-16 rounded-full bg-zinc-200 dark:bg-neutral-800" />
          </div>
        </div>
      </div>

      {/* Main Inbox Interface Grid Mock */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-5 h-[calc(100vh-13rem)] lg:h-[calc(100vh-15.5rem)] min-h-[500px]">
        {/* Left side list pane */}
        <div className="lg:col-span-4 rounded-[24px] border border-white/20 bg-white/30 p-4 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 flex flex-col space-y-4 h-full">
          <div className="h-10 rounded-xl bg-zinc-200 dark:bg-neutral-800 w-full" />
          <div className="flex-1 overflow-y-auto space-y-3.5">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="flex gap-3 p-3 rounded-2xl border border-white/10 bg-white/10 dark:bg-neutral-800/10">
                <div className="h-9 w-9 rounded-full bg-zinc-200 dark:bg-neutral-800 shrink-0" />
                <div className="flex-1 space-y-1.5 min-w-0">
                  <div className="flex justify-between items-center">
                    <div className="h-3.5 w-24 rounded bg-zinc-300 dark:bg-neutral-700" />
                    <div className="h-2.5 w-8 rounded bg-zinc-200 dark:bg-neutral-800" />
                  </div>
                  <div className="h-3 w-32 rounded bg-zinc-200 dark:bg-neutral-800" />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Right side chat pane */}
        <div className="lg:col-span-8 rounded-[24px] border border-white/20 bg-white/30 p-5 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 flex flex-col h-full">
          <div className="flex items-center justify-between pb-4 border-b border-white/10 mb-4">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-full bg-zinc-200 dark:bg-neutral-800" />
              <div className="space-y-1">
                <div className="h-4 w-28 rounded bg-zinc-300 dark:bg-neutral-700" />
                <div className="h-3 w-16 rounded bg-zinc-200 dark:bg-neutral-800" />
              </div>
            </div>
            <div className="h-8 w-24 rounded-full bg-zinc-200 dark:bg-neutral-800" />
          </div>
          <div className="flex-1 space-y-4 py-4 overflow-y-auto">
            <div className="flex justify-start">
              <div className="h-10 w-44 rounded-2xl bg-zinc-200 dark:bg-neutral-800" />
            </div>
            <div className="flex justify-end">
              <div className="h-10 w-36 rounded-2xl bg-zinc-300 dark:bg-neutral-700" />
            </div>
            <div className="flex justify-start">
              <div className="h-14 w-60 rounded-2xl bg-zinc-200 dark:bg-neutral-800" />
            </div>
          </div>
          <div className="mt-4 pt-4 border-t border-white/10 flex gap-2">
            <div className="h-11 rounded-2xl bg-zinc-200 dark:bg-neutral-800 flex-1" />
            <div className="h-11 w-11 rounded-2xl bg-zinc-300 dark:bg-neutral-700 shrink-0" />
          </div>
        </div>
      </div>
    </div>
  );
}
