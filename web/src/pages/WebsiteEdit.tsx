import React from 'react';
import { usePage, useForm, router } from '@inertiajs/react';
import { PageHeader } from '@/components/ui/page-header';
import { FlashMessageDisplay } from '@/components/ui/flash-message';
import { Settings, Info } from 'lucide-react';
import type { FlashMessage } from '@/types';
import { AdminLayout } from "@/components/admin-layout";

interface Website {
  id: number;
  domain: string;
  created_at: string;
  conversion_goals?: string[];
  subdomain_tracking_enabled?: boolean;
  privacy_mode?: string;
}

interface Event {
  event_name: string;
  domain: string;
  website_id: number;
}

interface WebsiteEditProps {
  title: string;
  website: Website;
  all_distinct_events: Event[];
  conversion_goals: string[];
  subdomain_tracking_enabled: boolean;
  flash?: FlashMessage;
  error?: string;
  [key: string]: any;
}

// ConversionGoalsSelector component for handling the goals selection
const ConversionGoalsSelector: React.FC<{
  events: Event[];
  initialGoals: string[];
  onGoalsChange: (goals: string[]) => void;
}> = ({ events, initialGoals, onGoalsChange }) => {
  const [selectedGoals, setSelectedGoals] = React.useState<string[]>(initialGoals);
  const [searchTerm, setSearchTerm] = React.useState("");

  const handleGoalToggle = (eventName: string) => {
    setSelectedGoals(prev => {
      const newGoals = prev.includes(eventName)
        ? prev.filter(goal => goal !== eventName)
        : [...prev, eventName];

      // Notify parent component about the change
      onGoalsChange(newGoals);
      return newGoals;
    });
  };

  const filteredEvents = React.useMemo(() => {
    if (!searchTerm.trim()) {
      return events;
    }
    const lowerSearchTerm = searchTerm.toLowerCase();
    return events.filter(event =>
      event.event_name.toLowerCase().includes(lowerSearchTerm)
    );
  }, [events, searchTerm]);

  return (
    <div className="bg-white border rounded-lg shadow-sm overflow-hidden mt-4">
      {/* We don't need this hidden input as we'll set the value in the form submission handler */}

      {/* Search Bar */}
      <div className="p-4 border-b">
        <div className="relative">
          <input
            type="search"
            placeholder="Search by event name..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full border border-gray-300 focus:border-black focus:ring-black rounded-md pl-9 py-2 text-sm"
          />
          <div className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.3-4.3" />
            </svg>
          </div>
        </div>
      </div>

      {/* Events List */}
      <div className="max-h-72 overflow-y-auto">
        {filteredEvents.length > 0 ? (
          filteredEvents.map((event) => {
            const isChecked = selectedGoals.includes(event.event_name);
            return (
              <div
                key={event.event_name}
                className="flex items-center px-4 py-2 border-b last:border-0 hover:bg-gray-50"
              >
                <input
                  type="checkbox"
                  id={`goal-${event.event_name}`}
                  checked={isChecked}
                  onChange={() => handleGoalToggle(event.event_name)}
                  className="h-4 w-4 text-black focus:ring-black border-gray-300 rounded"
                />
                <label
                  htmlFor={`goal-${event.event_name}`}
                  className="ml-3 cursor-pointer flex-grow"
                >
                  <span className="font-medium text-gray-900">
                    {event.event_name}
                  </span>
                </label>
              </div>
            );
          })
        ) : (
          <div className="p-6 text-center">
            <p className="text-sm text-gray-600">
              No events found for this website{searchTerm ? " matching your search" : ""}.
            </p>
            <p className="text-xs text-gray-500 mt-1">
              Events will appear here once your website starts tracking user interactions.
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

const WebsiteEdit: React.FC = () => {
  const { props } = usePage<WebsiteEditProps>();
  const {
    website,
    all_distinct_events,
    conversion_goals,
    subdomain_tracking_enabled,
    flash,
    error
  } = props;

  const form = useForm({
    conversion_goals: JSON.stringify(conversion_goals || []),
    subdomain_tracking_enabled: (subdomain_tracking_enabled || false).toString(),
  });

  const [selectedGoals, setSelectedGoals] = React.useState<string[]>(conversion_goals || []);
  const [subdomainTrackingEnabled, setSubdomainTrackingEnabled] = React.useState<boolean>(
    subdomain_tracking_enabled || false
  );

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    // Clean up goals before submitting
    const cleanedGoals = selectedGoals.filter(goal => goal && goal.trim().length > 0)
      .map(goal => goal.trim());

    form.transform(() => ({
      conversion_goals: JSON.stringify(cleanedGoals),
      subdomain_tracking_enabled: subdomainTrackingEnabled.toString(),
    }));
    form.post(`/admin/websites/${website.id}`);
  };

  if (!website) {
    return (
      <AdminLayout currentPath="/admin">
        <div className="py-8">
          <div className="text-center text-red-600">Website not found</div>
        </div>
      </AdminLayout>
    );
  }

  return (
    <AdminLayout currentPath="/admin">
      <div className="py-6">
        <PageHeader
          title={`Settings of ${website.domain}`}
          icon={Settings}
          leftContent={
            <a href="/admin" className="text-gray-600 hover:text-gray-900 mr-4">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="m15 18-6-6 6-6" />
              </svg>
            </a>
          }
        />

        <FlashMessageDisplay
          flash={flash}
          error={error}
        />

        {/* Main Content */}
        <div className="bg-white border border-black shadow-sm rounded-lg overflow-hidden">
          <div className="p-6">
            <form
              className="space-y-6"
              id="website-edit-form"
              onSubmit={handleSubmit}
              data-website-id={website.id}
            >
              {/* Website Info Section */}
              <div>
                <h2 className="text-xl font-semibold flex items-center gap-2 mb-4">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <circle cx="12" cy="12" r="10" />
                    <line x1="2" x2="22" y1="12" y2="12" />
                    <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
                  </svg>
                  Website Settings
                </h2>
                <div className="space-y-4">
                  <div>
                    <label htmlFor="domain" className="block text-sm font-medium text-gray-700 mb-1">
                      Domain
                    </label>
                    <input
                      type="text"
                      name="domain"
                      id="domain"
                      value={website.domain}
                      readOnly
                      className="block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm bg-gray-50 text-gray-500 cursor-not-allowed sm:text-sm"
                      placeholder="example.com"
                    />
                    <p className="mt-1 text-xs text-gray-500">
                      Domain cannot be changed after website creation to preserve analytics data integrity.
                    </p>
                  </div>
                </div>
              </div>

              {/* Conversion Goals Section */}
              <div className="pt-6 border-t border-gray-200">
                <h2 className="text-xl font-semibold flex items-center gap-2 mb-4">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <circle cx="12" cy="12" r="10" />
                    <circle cx="12" cy="12" r="6" />
                    <circle cx="12" cy="12" r="2" />
                  </svg>
                  Conversion Goals
                </h2>
                <p className="text-sm text-gray-500 mb-4">
                  Select the events you want to track as conversion goals for this website.
                  These will be used in conversion rate calculations and funnel analysis.
                </p>

                <div className="bg-gray-50 border rounded-lg p-4 mb-4">
                  <div className="flex items-start gap-3">
                    <div className="shrink-0 mt-0.5">
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        className="text-gray-600"
                      >
                        <circle cx="12" cy="12" r="10" />
                        <line x1="12" y1="16" x2="12" y2="12" />
                        <line x1="12" y1="8" x2="12.01" y2="8" />
                      </svg>
                    </div>
                    <div className="text-sm text-gray-700">
                      <p>
                        Events are automatically collected when users interact with your website.
                        Choose which events represent important conversions for your business.
                      </p>
                    </div>
                  </div>
                </div>

                <ConversionGoalsSelector
                  events={all_distinct_events || []}
                  initialGoals={selectedGoals}
                  onGoalsChange={setSelectedGoals}
                />
              </div>

              {/* Privacy Mode Section - Hidden but functional */}
              {/* Uncomment to allow users to toggle between privacy and tracking modes
              <div className="pt-6 border-t border-gray-200">
                <h2 className="text-xl font-semibold flex items-center gap-2 mb-4">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="text-green-600"
                  >
                    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                  </svg>
                  Privacy & Tracking Settings
                </h2>
                <p className="text-sm text-gray-500 mb-4">
                  Choose how visitor tracking works for this website. Both modes are privacy-compliant (no IP storage).
                </p>

                <div className="space-y-4">
                  <div className="border rounded-lg p-4 bg-white">
                    <div className="flex items-start gap-3">
                      <div className="flex-1">
                        <h3 className="font-medium mb-2">Tracking Mode</h3>
                        <div className="space-y-3">
                          <label className="flex items-center gap-3 p-3 border rounded-lg cursor-pointer hover:bg-gray-50 transition-colors">
                            <input
                              type="radio"
                              name="privacy_mode_radio"
                              value="privacy"
                              checked={privacyMode === 'privacy'}
                              onChange={(e) => setPrivacyMode(e.target.value)}
                              className="w-4 h-4 text-black border-gray-300 focus:ring-black"
                            />
                            <div className="flex-1">
                              <div className="font-medium text-sm">Privacy Mode</div>
                              <div className="text-xs text-gray-600 mt-1">
                                ✓ Visitor IDs rotate daily<br/>
                                ✓ Aggregate analytics only<br/>
                                ✗ No multi-day user journeys
                              </div>
                            </div>
                          </label>
                          <label className="flex items-center gap-3 p-3 border rounded-lg cursor-pointer hover:bg-gray-50 transition-colors">
                            <input
                              type="radio"
                              name="privacy_mode_radio"
                              value="tracking"
                              checked={privacyMode === 'tracking'}
                              onChange={(e) => setPrivacyMode(e.target.value)}
                              className="w-4 h-4 text-black border-gray-300 focus:ring-black"
                            />
                            <div className="flex-1">
                              <div className="font-medium text-sm">Tracking Mode (Default)</div>
                              <div className="text-xs text-gray-600 mt-1">
                                ✓ Stable visitor IDs (cross-session)<br/>
                                ✓ Full journey tracking & cohorts<br/>
                                ✓ Still privacy-compliant (hashed IDs only)
                              </div>
                            </div>
                          </label>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                    <div className="flex items-start gap-3">
                      <Info className="h-5 w-5 text-blue-600 mt-0.5 shrink-0" />
                      <div className="text-sm text-blue-900">
                        <p className="font-medium mb-1">Privacy Guarantee</p>
                        <p className="text-xs">
                          Both modes are GDPR-compliant. We <strong>never store IP addresses</strong> – only anonymous hashed identifiers.
                          The difference is how long we can connect a visitor's sessions.
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              */}

              {/* Subdomain Tracking Section */}
              <div className="pt-6 border-t border-gray-200">
                <h2 className="text-xl font-semibold flex items-center gap-2 mb-4">
                  <Info className="w-5 h-5 text-blue-500" />
                  Subdomain Tracking
                </h2>
                <p className="text-sm text-gray-500 mb-4">
                  Configure whether subdomains should be tracked as part of this website or separately.
                  This affects how referrals between subdomains are treated.
                </p>

                <div className="bg-gray-50 border rounded-lg p-4 mb-4">
                  <div className="flex items-start gap-3">
                    <div className="shrink-0 mt-0.5">
                      <Info className="h-4 w-4 text-gray-600" />
                    </div>
                    <div className="text-sm text-gray-700">
                      <p className="mb-2">
                        <strong>Subdomain tracking behavior:</strong>
                      </p>
                      <ul className="space-y-1 text-xs list-disc list-inside">
                        <li><strong>Enabled:</strong> blog.{website.domain} → app.{website.domain} = self-referral (unified analytics)</li>
                        <li><strong>Disabled:</strong> blog.{website.domain} → app.{website.domain} = external referral (separate analytics)</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="border rounded-lg p-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <h3 className="font-medium">{website.domain}</h3>
                      <p className="text-sm text-gray-500">
                        Track all subdomains under {website.domain} as one website
                      </p>
                    </div>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input
                        type="checkbox"
                        className="sr-only peer"
                        checked={subdomainTrackingEnabled}
                        onChange={(e) => setSubdomainTrackingEnabled(e.target.checked)}
                      />
                      <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-gray-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-black"></div>
                    </label>
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="pt-6 border-t border-gray-200 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => router.visit('/admin')}
                  className="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-black"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={form.processing}
                  className="inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-black hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-black disabled:opacity-70 disabled:cursor-not-allowed"
                >
                  {form.processing ? 'Saving...' : 'Save Changes'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </AdminLayout>
  );
};

export default WebsiteEdit;
