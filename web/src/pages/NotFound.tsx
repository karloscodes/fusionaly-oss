import { PageHeader } from '@/components/ui/page-header';
import { AlertTriangle } from 'lucide-react';

export function NotFound() {
  return (
    <div className="min-h-screen bg-white py-4">
      <div className="flex flex-col gap-4 max-w-7xl mx-auto">
        <PageHeader
          title="Page Not Found"
          icon={AlertTriangle}
        />

        <div className="text-center py-12">
          <p className="mt-2 text-black/50">The requested page could not be found.</p>
          <a
            href="/login"
            className="mt-4 inline-block px-4 py-2 bg-black text-white rounded hover:bg-black/80"
          >
            Go to Home
          </a>
        </div>
      </div>
    </div>
  );
}
