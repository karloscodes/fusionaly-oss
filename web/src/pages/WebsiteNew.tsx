import React from 'react';
import { usePage, useForm, router } from '@inertiajs/react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Globe, ArrowLeft, Plus } from 'lucide-react';
import { FlashMessageDisplay } from '../components/ui/flash-message';
import type { FlashMessage } from "@/types";
import { AdminLayout } from "@/components/admin-layout";

interface WebsiteNewProps {
  title: string;
  websites: any[];
  currentWebsiteId: number;
  flash?: FlashMessage;
  error?: string;
  [key: string]: any;
}

const WebsiteNew: React.FC = () => {
  const { props } = usePage<WebsiteNewProps>();
  const { flash, error } = props;

  const form = useForm({
    domain: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    form.post('/admin/websites');
  };

  const validateDomain = (value: string) => {
    // More permissive domain validation for real-world domains
    const domainRegex = /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/;
    return domainRegex.test(value);
  };

  const isValidDomain = form.data.domain.trim() === '' || form.data.domain.trim() === 'localhost' || validateDomain(form.data.domain.trim());

  return (
    <AdminLayout currentPath="/admin/websites/new">
      <div className="py-8">
        <FlashMessageDisplay flash={flash} error={error} />

        {/* Centered Card */}
        <div className="flex justify-center">
          <div className="w-full max-w-md">
            <div className="bg-white border border-black rounded-xl overflow-hidden">
              {/* Card Header */}
              <div className="px-6 py-5 border-b border-gray-200">
                <div className="flex items-center gap-3">
                  <div className="p-2 bg-black rounded-lg">
                    <Globe className="w-5 h-5 text-white" />
                  </div>
                  <div>
                    <h2 className="text-lg font-semibold text-gray-900">Add New Website</h2>
                    <p className="text-sm text-gray-500">Start tracking analytics for your site</p>
                  </div>
                </div>
              </div>

              {/* Card Body */}
              <div className="px-6 py-6">
                <form onSubmit={handleSubmit} className="space-y-5">

                  <div>
                    <label htmlFor="domain" className="block text-sm font-medium text-gray-900 mb-2">
                      Website Domain
                    </label>
                    <Input
                      id="domain"
                      name="domain"
                      type="text"
                      placeholder="example.com"
                      value={form.data.domain}
                      onChange={(e) => form.setData('domain', e.target.value)}
                      className={`w-full h-11 text-base ${!isValidDomain && form.data.domain.trim() !== '' ? 'border-red-500 focus:border-red-500 focus:ring-red-500' : ''}`}
                      required
                      autoFocus
                    />
                    {form.errors.domain && (
                      <p className="text-red-500 mt-2 text-sm">{form.errors.domain}</p>
                    )}
                    {form.data.domain.trim() !== '' && !isValidDomain ? (
                      <p className="text-red-500 mt-2 text-sm">
                        Please enter a valid domain name (e.g., example.com)
                      </p>
                    ) : (
                      <p className="text-gray-500 mt-2 text-xs">
                        Enter your website domain without http:// or https://
                      </p>
                    )}
                  </div>

                  <div className="flex gap-3 pt-2">
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => router.visit('/admin')}
                      className="flex-1 h-11"
                    >
                      <ArrowLeft className="w-4 h-4 mr-2" />
                      Cancel
                    </Button>
                    <Button
                      type="submit"
                      disabled={form.processing || !isValidDomain || form.data.domain.trim() === ''}
                      className="flex-1 h-11 bg-black hover:bg-gray-800"
                    >
                      {form.processing ? (
                        'Creating...'
                      ) : (
                        <>
                          <Plus className="w-4 h-4 mr-2" />
                          Create Website
                        </>
                      )}
                    </Button>
                  </div>
                </form>
              </div>

              {/* Card Footer */}
              <div className="px-6 py-4 border-t border-gray-200">
                <p className="text-xs text-gray-500 text-center">
                  After creation, you'll receive a tracking script to add to your website.
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AdminLayout>
  );
};

export default WebsiteNew;
