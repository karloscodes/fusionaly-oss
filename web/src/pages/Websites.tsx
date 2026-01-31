import React, { useState, useEffect, useMemo } from 'react';
import { usePage, router, Link } from '@inertiajs/react';
import { Button } from '../components/ui/button';
import {
  Globe,
  Plus,
  Settings,
  Trash2,
  Code,
  Copy,
  Check,
  TrendingUp,
  Zap,
  ArrowRight,
  Clock,
  AlertTriangle
} from 'lucide-react';
import { FlashMessageDisplay } from '../components/ui/flash-message';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter
} from '../components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../components/ui/tooltip';
import type { FlashMessage } from "@/types";
import { AdminLayout } from "@/components/admin-layout";
import { formatDistanceToNow } from 'date-fns';

interface Website {
  id: number;
  domain: string;
  created_at: string;
  event_count?: number;
}

interface WebsiteInsight {
  website_id: number;
  website_domain: string;
  title: string;
  description: string;
  severity: 'high' | 'medium' | 'low' | 'info';
}

interface WebsitesProps {
  websites: Website[];
  insights?: WebsiteInsight[];
  flash?: FlashMessage;
  error?: string;
  total_events?: number;
  [key: string]: any;
}

const RECENT_WEBSITES_KEY = 'fusionaly_recent_websites';
const MAX_RECENT_WEBSITES = 3;

interface RecentWebsiteAccess {
  id: number;
  accessedAt: number;
}

// Helper to get recent website IDs from localStorage
const getRecentWebsiteAccesses = (): RecentWebsiteAccess[] => {
  try {
    const stored = localStorage.getItem(RECENT_WEBSITES_KEY);
    if (stored) {
      return JSON.parse(stored);
    }
  } catch (e) {
    console.error('Error reading recent websites from localStorage:', e);
  }
  return [];
};

// Helper to save a website access to localStorage
export const saveRecentWebsiteAccess = (websiteId: number): void => {
  try {
    const accesses = getRecentWebsiteAccesses();
    // Remove existing entry for this website if present
    const filtered = accesses.filter(a => a.id !== websiteId);
    // Add new entry at the beginning
    filtered.unshift({ id: websiteId, accessedAt: Date.now() });
    // Keep only the most recent ones
    const trimmed = filtered.slice(0, MAX_RECENT_WEBSITES);
    localStorage.setItem(RECENT_WEBSITES_KEY, JSON.stringify(trimmed));
  } catch (e) {
    console.error('Error saving recent website to localStorage:', e);
  }
};

const Websites: React.FC = () => {
  const { props } = usePage<WebsitesProps>();
  const { websites: websitesData, insights: insightsData, flash, error, total_events } = props;

  // Process websites data
  let websites: Website[] = [];
  if (Array.isArray(websitesData)) {
    websites = (websitesData as unknown as Record<string, unknown>[]).map(site => ({
      id: Number(site.id || 0),
      domain: String(site.domain || ''),
      created_at: String(site.created_at || ''),
      event_count: Number(site.event_count || 0)
    } as Website));
    // Sort by creation date (newest first)
    websites.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  }

  const [recentAccesses, setRecentAccesses] = useState<RecentWebsiteAccess[]>([]);
  const [websiteToDelete, setWebsiteToDelete] = useState<Website | null>(null);
  const [showIntegrationHelp, setShowIntegrationHelp] = useState(false);
  const [selectedWebsiteForIntegration, setSelectedWebsiteForIntegration] = useState<Website | null>(null);
  const [copiedScript, setCopiedScript] = useState(false);

  // Load recent website accesses from localStorage
  useEffect(() => {
    setRecentAccesses(getRecentWebsiteAccesses());
  }, []);

  // Get recently accessed websites (that still exist)
  const recentWebsites = useMemo(() => {
    return recentAccesses
      .map(access => websites.find(w => w.id === access.id))
      .filter((w): w is Website => w !== undefined);
  }, [recentAccesses, websites]);

  const handleDeleteWebsite = () => {
    if (!websiteToDelete) return;
    router.post(`/admin/websites/${websiteToDelete.id}/delete`, {}, {
      onSuccess: () => setWebsiteToDelete(null),
      onError: (errors) => console.error('Failed to delete website:', errors)
    });
  };

  const formatCreationDate = (dateString: string): string => {
    if (!dateString || dateString === '' || dateString === 'null') return 'Unknown';
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return 'Unknown';
      return formatDistanceToNow(date, { addSuffix: true });
    } catch {
      return 'Unknown';
    }
  };

  // Calculate totals
  const totalEvents = total_events || websites.reduce((sum, w) => sum + (w.event_count || 0), 0);
  const activeWebsites = websites.filter(w => (w.event_count || 0) > 0).length;

  // Process insights data and create a map by website_id for quick lookup
  const insights: WebsiteInsight[] = useMemo(() => {
    if (insightsData && Array.isArray(insightsData)) {
      return insightsData;
    }
    return [];
  }, [insightsData]);

  // Create a map of website_id to their insights for quick lookup
  const insightsByWebsite = useMemo(() => {
    const map = new Map<number, WebsiteInsight[]>();
    insights.forEach(insight => {
      const existing = map.get(insight.website_id) || [];
      existing.push(insight);
      map.set(insight.website_id, existing);
    });
    return map;
  }, [insights]);

  return (
    <AdminLayout currentPath="/admin">
      <div className="py-6">
        <FlashMessageDisplay flash={flash} error={error} />

        {/* Hero Section */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-2">
            <h1 className="text-2xl font-bold text-black">Welcome back</h1>
            <Button
              onClick={() => window.location.href = '/admin/websites/new'}
              className="bg-black hover:bg-black/80"
            >
              <Plus className="h-4 w-4 mr-2" />
              Add Website
            </Button>
          </div>
          <p className="text-black/60">
            {websites.length === 0
              ? "Get started by adding your first website to track"
              : `Managing ${websites.length} website${websites.length !== 1 ? 's' : ''}`
            }
          </p>
        </div>

        {/* Stats Cards */}
        {websites.length > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
            <div className="bg-white border border-black/10 rounded-xl p-5">
              <div className="flex items-center gap-3">
                <div className="p-2.5 bg-blue-50 rounded-lg">
                  <Globe className="h-5 w-5 text-blue-600" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-black">{websites.length}</p>
                  <p className="text-sm text-black/60">Total Websites</p>
                </div>
              </div>
            </div>
            <div className="bg-white border border-black/10 rounded-xl p-5">
              <div className="flex items-center gap-3">
                <div className="p-2.5 bg-green-50 rounded-lg">
                  <TrendingUp className="h-5 w-5 text-green-600" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-black">{totalEvents.toLocaleString()}</p>
                  <p className="text-sm text-black/60">Total Events</p>
                </div>
              </div>
            </div>
            <div className="bg-white border border-black/10 rounded-xl p-5">
              <div className="flex items-center gap-3">
                <div className="p-2.5 bg-purple-50 rounded-lg">
                  <Zap className="h-5 w-5 text-purple-600" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-black">{activeWebsites}</p>
                  <p className="text-sm text-black/60">Collecting Data</p>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Recently Accessed Section */}
        {recentWebsites.length > 0 && (
          <div className="mb-8">
            <div className="flex items-center gap-2 mb-4">
              <Clock className="h-4 w-4 text-black/50" />
              <h2 className="text-sm font-medium text-black/70 uppercase tracking-wide">Recently Accessed</h2>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {recentWebsites.map((website) => (
                <Link
                  key={website.id}
                  href={`/admin/websites/${website.id}/dashboard`}
                  className="bg-white border border-black/10 rounded-xl p-4 hover:border-black hover:shadow-sm transition-all group"
                >
                  <div className="flex items-center gap-3">
                    <img
                      src={`https://www.google.com/s2/favicons?domain=${website.domain}&sz=32`}
                      alt=""
                      className="h-8 w-8 rounded-lg bg-black/5 p-1"
                      onError={(e) => {
                        const target = e.target as HTMLImageElement;
                        target.style.display = 'none';
                      }}
                    />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-semibold text-black group-hover:text-black truncate">
                        {website.domain}
                      </p>
                      <p className="text-xs text-black/50">
                        {(website.event_count || 0).toLocaleString()} events
                      </p>
                    </div>
                    <ArrowRight className="h-4 w-4 text-black/40 group-hover:text-black transition-colors" />
                  </div>
                </Link>
              ))}
            </div>
          </div>
        )}

        {/* Empty State */}
        {websites.length === 0 ? (
          <div className="bg-white border border-black/10 rounded-xl p-12 text-center">
            <div className="max-w-md mx-auto">
              <div className="w-16 h-16 bg-blue-50 rounded-full flex items-center justify-center mx-auto mb-6">
                <Globe className="h-8 w-8 text-blue-600" />
              </div>
              <h2 className="text-xl font-semibold text-black mb-2">No websites yet</h2>
              <p className="text-black/60 mb-6">
                Add your first website to start collecting analytics data. It only takes a minute to set up.
              </p>
              <Button
                onClick={() => window.location.href = '/admin/websites/new'}
                className="bg-black hover:bg-black/80"
              >
                <Plus className="h-4 w-4 mr-2" />
                Add Your First Website
              </Button>
            </div>
          </div>
        ) : (
          /* Websites List */
          <div className="bg-white border border-black/10 rounded-xl overflow-hidden">
            <div className="px-5 py-4 border-b border-black/10 flex items-center justify-between">
              <h2 className="font-semibold text-black">Your Websites</h2>
              <span className="text-sm text-black/60">{websites.length} site{websites.length !== 1 ? 's' : ''}</span>
            </div>
            <div className="divide-y divide-black/10">
              {websites.map((website) => {
                const websiteInsights = insightsByWebsite.get(website.id) || [];
                const hasHighSeverity = websiteInsights.some(i => i.severity === 'high');
                const hasMediumSeverity = websiteInsights.some(i => i.severity === 'medium');
                const topInsight = websiteInsights[0];

                return (
                  <div
                    key={website.id}
                    className="px-5 py-4 hover:bg-black/5 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      {/* Left: Website info */}
                      <div className="flex items-center gap-4 flex-1 min-w-0">
                        <div className="flex-shrink-0">
                          <img
                            src={`https://www.google.com/s2/favicons?domain=${website.domain}&sz=32`}
                            alt=""
                            className="h-8 w-8 rounded-lg bg-black/5 p-1"
                            onError={(e) => {
                              const target = e.target as HTMLImageElement;
                              target.style.display = 'none';
                            }}
                          />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <Link
                              href={`/admin/websites/${website.id}/dashboard`}
                              className="text-sm font-semibold text-black hover:text-black transition-colors truncate"
                            >
                              {website.domain}
                            </Link>
                            {topInsight && (
                              <TooltipProvider>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span
                                      data-insight-badge="true"
                                      className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium ${
                                      hasHighSeverity ? 'bg-red-50 text-red-700' :
                                      hasMediumSeverity ? 'bg-amber-50 text-amber-700' :
                                      'bg-blue-50 text-blue-700'
                                    }`}>
                                      <AlertTriangle className="h-3 w-3" />
                                      {websiteInsights.length > 1 ? `${websiteInsights.length} issues` : '1 issue'}
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent side="right" className="max-w-xs">
                                    <div className="space-y-1">
                                      {websiteInsights.slice(0, 3).map((insight, idx) => (
                                        <p key={idx} className="text-xs">{insight.title}</p>
                                      ))}
                                      {websiteInsights.length > 3 && (
                                        <p className="text-xs text-black/40">+{websiteInsights.length - 3} more</p>
                                      )}
                                    </div>
                                  </TooltipContent>
                                </Tooltip>
                              </TooltipProvider>
                            )}
                          </div>
                          <p className="text-xs text-black/60">
                            Added {formatCreationDate(website.created_at)}
                          </p>
                        </div>
                      </div>

                      {/* Center: Stats */}
                      <div className="hidden md:flex items-center gap-6 px-4">
                        <div className="text-right">
                          <p className="text-sm font-medium text-black">
                            {(website.event_count || 0).toLocaleString()}
                          </p>
                          <p className="text-xs text-black/60">events</p>
                        </div>
                        <div className="flex items-center gap-1.5">
                          <span className={`w-2 h-2 rounded-full ${(website.event_count || 0) > 0 ? 'bg-green-500' : 'bg-black/40'}`} />
                          <span className={`text-xs font-medium ${(website.event_count || 0) > 0 ? 'text-green-600' : 'text-black/60'}`}>
                            {(website.event_count || 0) > 0 ? 'Active' : 'Pending'}
                          </span>
                        </div>
                      </div>

                      {/* Right: Actions */}
                      <div className="flex items-center gap-1">
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => {
                                  setSelectedWebsiteForIntegration(website);
                                  setShowIntegrationHelp(true);
                                }}
                                className="h-8 w-8 p-0 text-black/50 hover:text-black/70"
                              >
                                <Code className="h-4 w-4" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Get tracking script</TooltipContent>
                          </Tooltip>
                        </TooltipProvider>

                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => window.location.href = `/admin/websites/${website.id}/edit`}
                                className="h-8 w-8 p-0 text-black/50 hover:text-black/70"
                              >
                                <Settings className="h-4 w-4" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Settings</TooltipContent>
                          </Tooltip>
                        </TooltipProvider>

                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setWebsiteToDelete(website)}
                                className="h-8 w-8 p-0 text-black/50 hover:text-red-600"
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Delete</TooltipContent>
                          </Tooltip>
                        </TooltipProvider>

                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => window.location.href = `/admin/websites/${website.id}/dashboard`}
                          className="ml-2 text-black/50 hover:text-black/70"
                        >
                          <ArrowRight className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Delete Dialog */}
        <Dialog open={websiteToDelete !== null} onOpenChange={(open) => !open && setWebsiteToDelete(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delete Website</DialogTitle>
              <DialogDescription>
                Are you sure you want to delete {websiteToDelete?.domain}? This action cannot be undone and all associated data will be permanently removed.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setWebsiteToDelete(null)}>
                Cancel
              </Button>
              <Button variant="destructive" onClick={handleDeleteWebsite}>
                Delete Website
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Integration Help Dialog */}
        <Dialog open={showIntegrationHelp} onOpenChange={setShowIntegrationHelp}>
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Code className="h-5 w-5" />
                Add Tracking Script
              </DialogTitle>
              <DialogDescription>
                Add this script to {selectedWebsiteForIntegration?.domain} to start collecting data
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <p className="text-sm text-black/70 mb-3">
                  Copy and paste this script into your website's HTML, preferably in the <code className="bg-black/5 px-1 rounded">&lt;head&gt;</code> section:
                </p>
                <div className="bg-black p-4 rounded-lg font-mono text-sm overflow-x-auto">
                  <code className="text-green-400">
                    {`<script defer src="${window.location.origin}/y/api/v1/sdk.js" data-website-id="${selectedWebsiteForIntegration?.id}"></script>`}
                  </code>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-3"
                  onClick={() => {
                    const script = `<script defer src="${window.location.origin}/y/api/v1/sdk.js" data-website-id="${selectedWebsiteForIntegration?.id}"></script>`;
                    navigator.clipboard.writeText(script);
                    setCopiedScript(true);
                    setTimeout(() => setCopiedScript(false), 2000);
                  }}
                >
                  {copiedScript ? (
                    <>
                      <Check className="h-4 w-4 mr-2" />
                      Copied!
                    </>
                  ) : (
                    <>
                      <Copy className="h-4 w-4 mr-2" />
                      Copy Script
                    </>
                  )}
                </Button>
              </div>
              <div className="bg-blue-50 border border-blue-100 rounded-lg p-4">
                <h4 className="font-medium text-blue-900 mb-2">What happens next?</h4>
                <ul className="text-sm text-blue-800 space-y-1">
                  <li>• Events will start appearing within minutes</li>
                  <li>• Check the Events page to verify data is flowing</li>
                  <li>• View analytics on the Dashboard</li>
                </ul>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </AdminLayout>
  );
};

export default Websites;
