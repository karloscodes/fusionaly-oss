import { usePage } from "@inertiajs/react";
import { Dashboard, DashboardComponentProps } from "@/components/dashboard";
import { BarChart3, ExternalLink } from "lucide-react";

type PublicDashboardProps = DashboardComponentProps & {
  website_domain: string;
};

// Fusionaly Logo Component
const FusionalyLogo = ({ className = "" }: { className?: string }) => (
  <span className={`font-semibold font-mono ${className}`}>
    fusionaly<span className="text-[#00D678]">_</span>
  </span>
);

export default function PublicDashboard() {
  const { props } = usePage<{ props: PublicDashboardProps }>();
  const data = props as unknown as PublicDashboardProps;

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Top Banner */}
      <div className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-2 text-sm text-gray-600">
            <BarChart3 className="w-4 h-4 text-[#00D678]" />
            <FusionalyLogo className="text-sm text-gray-900" />
            <span className="hidden sm:inline text-gray-400">·</span>
            <span className="hidden sm:inline">Privacy-first analytics</span>
          </div>
          <a
            href="https://fusionaly.com"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium text-gray-700 hover:text-gray-900 flex items-center gap-1"
          >
            Get your own
            <ExternalLink className="w-3 h-3" />
          </a>
        </div>
      </div>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-4 bg-white min-h-screen">
        {/* Header with domain */}
        <div className="py-4 flex justify-between items-center">
          <h1 className="text-2xl font-bold text-gray-900">{data.website_domain}</h1>
          <span className="text-sm text-gray-500">Last 30 days</span>
        </div>

        {/* Reuse Dashboard component in public/read-only mode */}
        <Dashboard
          {...data}
          is_public_view={true}
        />
      </div>

      {/* Bottom CTA */}
      <div className="bg-gray-900 text-white">
        <div className="max-w-7xl mx-auto px-4 py-12 text-center">
          <h2 className="text-xl sm:text-2xl font-bold mb-3">
            Want analytics like this for your site?
          </h2>
          <p className="text-gray-400 mb-6 max-w-md mx-auto">
            <FusionalyLogo className="text-white" /> is open source, privacy-first analytics. Self-host for free or try Pro.
          </p>
          <a
            href="https://fusionaly.com"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block bg-white text-gray-900 font-semibold px-6 py-3 rounded-lg hover:bg-gray-100 transition-colors"
          >
            Get Started Free
          </a>
        </div>
      </div>
    </div>
  );
}
