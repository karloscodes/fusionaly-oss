import React, { useState, useEffect } from 'react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Alert, AlertDescription } from '../components/ui/alert';
import { Progress } from '../components/ui/progress';

interface OnboardingData {
  email?: string;
  password?: string;
}

type Step = 'user_account' | 'password' | 'completed';

const stepNames: Record<Step, string> = {
  user_account: 'User Account',
  password: 'Password Setup',
  completed: 'Complete'
};

const stepProgress: Record<Step, number> = {
  user_account: 33,
  password: 66,
  completed: 100
};

export default function Onboarding() {
  const [currentStep, setCurrentStep] = useState<Step>('user_account');
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [data, setData] = useState<OnboardingData>({});
  const [confirmPassword, setConfirmPassword] = useState('');

  // Start onboarding session on component mount
  useEffect(() => {
    // Clear any existing authentication cookies
    clearAuthCookies();
    // Set timezone cookie
    setTimezoneCookie();
    // Start onboarding process
    startOnboarding();
  }, []);

  const clearAuthCookies = () => {
    // Clear common authentication cookies
    const cookiesToClear = ['fusionaly_session', 'fusionaly_flash', 'fusionaly_onboarding_session', '_session', 'session_token', 'auth_token'];
    cookiesToClear.forEach(cookieName => {
      document.cookie = `${cookieName}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
      document.cookie = `${cookieName}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; domain=${window.location.hostname};`;
    });
  };

  const setTimezoneCookie = () => {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    document.cookie = `_tz=${timezone}; path=/; SameSite=Lax`;
  };

  const startOnboarding = async () => {
    try {
      setLoading(true);
      setError('');

      const response = await fetch('/api/onboarding/start', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ force: false }),
      });

      const result = await response.json();

      if (!response.ok) {
        throw new Error(result.error || 'Failed to start onboarding');
      }

      setSessionId(result.session_id);
      setCurrentStep(result.step);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  };

  const handleUserAccountSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!data.email?.trim()) {
      setError('Email is required');
      return;
    }

    // Validate email format
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(data.email.trim())) {
      setError('Please enter a valid email address');
      return;
    }

    try {
      setLoading(true);
      setError('');

      const response = await fetch('/api/onboarding/user', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          email: data.email,
        }),
      });

      const result = await response.json();

      if (!response.ok) {
        throw new Error(result.error || 'User account setup failed');
      }

      setCurrentStep(result.step);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  };

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!data.password?.trim()) {
      setError('Password is required');
      return;
    }

    if (data.password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    if (data.password.length < 8) {
      setError('Password must be at least 8 characters long');
      return;
    }

    try {
      setLoading(true);
      setError('');

      const response = await fetch('/api/onboarding/password', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          password: data.password,
          confirm_password: confirmPassword,
        }),
      });

      const result = await response.json();

      if (!response.ok) {
        throw new Error(result.error || 'Password setup failed');
      }

      setCurrentStep(result.step);

      // Redirect to websites page after successful completion
      setTimeout(() => {
        window.location.href = '/admin/websites/new';
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  };

  const renderUserAccountStep = () => (
    <form onSubmit={handleUserAccountSubmit} className="space-y-4">
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="email">Email Address</Label>
          <Input
            id="email"
            type="email"
            name="email"
            placeholder="Enter your email address"
            value={data.email || ''}
            onChange={(e) => setData({ ...data, email: e.target.value })}
            disabled={loading}
            required
          />
        </div>

        <p className="text-sm text-gray-600">
          This email will be used for your admin account login.
        </p>
      </div>

      <Button type="submit" disabled={loading} className="w-full">
        {loading ? 'Setting up...' : 'Continue'}
      </Button>
    </form>
  );

  const renderPasswordStep = () => (
    <form onSubmit={handlePasswordSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          name="password"
          placeholder="Enter password"
          value={data.password || ''}
          onChange={(e) => setData({ ...data, password: e.target.value })}
          disabled={loading}
          required
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="confirm_password">Confirm Password</Label>
        <Input
          id="confirm_password"
          type="password"
          name="confirm_password"
          placeholder="Confirm password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          disabled={loading}
          required
        />
      </div>

      <p className="text-sm text-gray-600">
        Password must be at least 8 characters long.
      </p>

      <Button type="submit" disabled={loading} className="w-full">
        {loading ? 'Creating account...' : 'Complete Setup'}
      </Button>
    </form>
  );

  const renderCompletedStep = () => (
    <div className="text-center space-y-4">
      <div className="mx-auto w-16 h-16 bg-green-100 rounded-full flex items-center justify-center">
        <svg className="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      </div>

      <h3 className="text-xl font-semibold text-green-900">Setup Complete!</h3>
      <p className="text-green-700">
        Your Fusionaly installation is ready. You have been logged in automatically.
      </p>
      <p className="text-sm text-gray-600">
        Redirecting you to create your first website...
      </p>
    </div>
  );

  const renderStepContent = () => {
    switch (currentStep) {
      case 'user_account':
        return renderUserAccountStep();
      case 'password':
        return renderPasswordStep();
      case 'completed':
        return renderCompletedStep();
      default:
        return <div>Unknown step</div>;
    }
  };

  const getStepNumber = () => {
    const steps: Step[] = ['user_account', 'password', 'completed'];
    return steps.indexOf(currentStep) + 1;
  };

  if (loading && !sessionId) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
          <p className="mt-2 text-gray-600">Starting setup...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md mx-auto">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Initial Setup</h1>
          <p className="mt-2 text-gray-600">Configure your Fusionaly installation</p>
        </div>

        <Card className="mb-6">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-lg">{stepNames[currentStep]}</CardTitle>
                <CardDescription>
                  Step {getStepNumber()} of 3
                </CardDescription>
              </div>
              <div className="text-right text-sm text-gray-500">
                {stepProgress[currentStep]}%
              </div>
            </div>
            <Progress value={stepProgress[currentStep]} className="mt-4" />
          </CardHeader>

          <CardContent>
            {error && (
              <Alert className="mb-4 border-red-200 bg-red-50">
                <AlertDescription className="text-red-800">{error}</AlertDescription>
              </Alert>
            )}

            {renderStepContent()}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
