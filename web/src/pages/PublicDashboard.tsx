import { usePage } from "@inertiajs/react";
import { Dashboard, DashboardComponentProps } from "@/components/dashboard";

type PublicDashboardProps = DashboardComponentProps & {
  website_domain: string;
};

// Fusionaly Logo Component
const FusionalyLogo = () => (
  <span className="text-lg font-semibold font-mono">
    fusionaly<span className="text-[#00D678]">_</span>
  </span>
);

export default function PublicDashboard() {
  const { props } = usePage<{ props: PublicDashboardProps }>();
  const data = props as unknown as PublicDashboardProps;

  return (
    <div className="min-h-screen bg-black/5">
      {/* Top Banner */}
      <div className="bg-white border-b border-black/10">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <a
              href="https://fusionaly.com"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:opacity-80 transition-opacity"
            >
              <FusionalyLogo />
            </a>
            <span className="text-black/30">·</span>
            <span className="text-sm text-black/50">Simple analytics</span>
          </div>
          <a
            href="https://fusionaly.com"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium text-black/60 hover:text-black transition-colors"
          >
            Get your own
          </a>
        </div>
      </div>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-4 bg-white min-h-screen">
        {/* Header with domain */}
        <div className="py-4 flex justify-between items-center">
          <h1 className="text-2xl font-bold text-black">{data.website_domain}</h1>
          <span className="text-sm text-black/50">Last 30 days</span>
        </div>

        {/* Reuse Dashboard component in public/read-only mode */}
        <Dashboard
          {...data}
          is_public_view={true}
        />
      </div>

      {/* Bottom CTA - subtle, not aggressive */}
      <div className="border-t border-black/10 bg-white">
        <div className="max-w-7xl mx-auto px-4 py-10 text-center">
          <p className="text-black/60 mb-4">
            Like what you see? Fusionaly is free and open source.
          </p>
          <a
            href="https://fusionaly.com"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-block text-sm font-medium text-black border border-black/20 px-5 py-2.5 rounded-lg hover:border-black/40 hover:bg-black/5 transition-colors"
          >
            Learn more
          </a>
        </div>
      </div>
    </div>
  );
}
