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
          <p className="mt-2 text-muted-foreground">The requested page could not be found.</p>
          <a
            href="/login"
            className="mt-4 inline-block px-4 py-2 bg-primary text-primary-foreground rounded hover:bg-primary/90"
          >
            Go to Home
          </a>
        </div>
      </div>
    </div>
  );
}
