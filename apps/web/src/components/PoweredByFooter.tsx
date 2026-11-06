export function PoweredByFooter({ className = '' }: { className?: string }) {
  return (
    <footer
      className={`flex items-center justify-center gap-2 border-t border-zinc-100 px-4 py-3 text-sm font-medium text-zinc-500 dark:border-zinc-800 dark:text-zinc-400 ${className}`}
    >
      <img src="/web-app-manifest-192x192.png" alt="" className="h-6 w-6 rounded-md" />
      <span>
        Powered by{' '}
        <a href="https://infaira.ai/" target="_blank" rel="noopener noreferrer" className="hover:underline">
          Inf<span className="text-brand-500 dark:text-brand-400">AI</span>ra
        </a>
      </span>
    </footer>
  );
}
