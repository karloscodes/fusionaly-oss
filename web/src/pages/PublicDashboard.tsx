import { usePage } from "@inertiajs/react";
import { Dashboard, DashboardComponentProps } from "@/components/dashboard";

type PublicDashboardProps = DashboardComponentProps & {
  website_domain: string;
};

const GITHUB_URL = "https://github.com/karloscodes/fusionaly-oss";

// Quiet wordmark — attribution, not promotion.
const FusionalyWordmark = () => (
  <span className="font-mono">
    fusionaly<span className="text-[#00D678]">_</span>
  </span>
);

export default function PublicDashboard() {
  const { props } = usePage<{ props: PublicDashboardProps }>();
  const data = props as unknown as PublicDashboardProps;

  return (
    <div className="min-h-screen bg-white flex flex-col">
      {/* Header — same slim chrome as the in-app dashboard. The dashboard is
          the subject; Fusionaly is a quiet GitHub attribution, not a pitch. */}
      <header className="border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex h-14 items-center justify-between gap-4">
            <div className="flex items-center gap-2 min-w-0">
              <h1 className="text-sm font-semibold text-gray-900 truncate">
                {data.website_domain}
              </h1>
              <span className="text-gray-300">·</span>
              <span className="text-sm text-gray-500 whitespace-nowrap">Last 30 days</span>
            </div>
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              title="Fusionaly on GitHub"
              className="text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors whitespace-nowrap"
            >
              <FusionalyWordmark />
            </a>
          </div>
        </div>
      </header>

      {/* Read-only dashboard — same component as in-app */}
      <main className="flex-1">
        <div className="max-w-7xl mx-auto px-4">
          <Dashboard {...data} is_public_view={true} />
        </div>
      </main>

      {/* Footer — quiet attribution, no call to action */}
      <footer className="border-t border-gray-200">
        <div className="max-w-7xl mx-auto px-4 py-6">
          <a
            href="https://fusionaly.com"
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-gray-500 hover:text-gray-800 transition-colors"
          >
            Self-hosted with <FusionalyWordmark />
          </a>
        </div>
      </footer>
    </div>
  );
}
