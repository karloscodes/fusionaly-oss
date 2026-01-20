import { usePage } from "@inertiajs/react";
import { Dashboard, DashboardComponentProps } from "@/components/dashboard";

type PublicDashboardProps = DashboardComponentProps & {
  website_domain: string;
};

export default function PublicDashboard() {
  const { props } = usePage<{ props: PublicDashboardProps }>();
  const data = props as unknown as PublicDashboardProps;

  return (
    <div className="max-w-7xl mx-auto px-4">
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

      {/* Footer */}
      <div className="text-center py-8 border-t border-gray-200 mt-8">
        <a
          href="https://fusionaly.com"
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          Powered by Fusionaly
        </a>
      </div>
    </div>
  );
}
