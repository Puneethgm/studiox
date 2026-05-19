import { Card } from '@/components/ui/Card';

export default function NotFound() {
  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 px-4">
      <div className="w-full max-w-md">
        <Card>
          <div className="py-8 text-center">
            <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Link not found</h1>
            <p className="mt-2 text-slate-600">
              This campaign may have ended or the link is incorrect.
            </p>
          </div>
        </Card>
      </div>
    </main>
  );
}
