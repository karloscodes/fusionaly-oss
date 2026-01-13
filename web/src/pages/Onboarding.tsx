import { useEffect } from 'react';
import { usePage } from '@inertiajs/react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Alert, AlertDescription } from '../components/ui/alert';
import { Progress } from '../components/ui/progress';

interface OnboardingProps {
  step: string;
  email?: string;
  flash?: {
    error?: string;
    success?: string;
    info?: string;
  };
  [key: string]: unknown;
}

type Step = 'user_account' | 'password' | 'geolite' | 'completed';

const stepNames: Record<Step, string> = {
  user_account: 'User Account',
  password: 'Password Setup',
  geolite: 'Location Data',
  completed: 'Complete'
};

const stepProgress: Record<Step, number> = {
  user_account: 25,
  password: 50,
  geolite: 75,
  completed: 100
};

export default function Onboarding() {
  const { props } = usePage<OnboardingProps>();
  const currentStep = (props.step || 'user_account') as Step;
  const email = props.email || '';
  const flash = props.flash || {};

  // Set timezone cookie on mount
  useEffect(() => {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    document.cookie = `_tz=${timezone}; path=/; SameSite=Lax`;
  }, []);

  const renderUserAccountStep = () => (
    <form action="/setup/user" method="POST" className="space-y-4">
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="email">Email Address</Label>
          <Input
            id="email"
            type="email"
            name="email"
            placeholder="Enter your email address"
            defaultValue={email}
            required
          />
        </div>

        <p className="text-sm text-gray-600">
          This email will be used for your admin account login.
        </p>
      </div>

      <Button type="submit" className="w-full">
        Continue
      </Button>
    </form>
  );

  const renderPasswordStep = () => (
    <form action="/setup/password" method="POST" className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          name="password"
          placeholder="Enter password"
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
          required
        />
      </div>

      <p className="text-sm text-gray-600">
        Password must be at least 8 characters long.
      </p>

      <Button type="submit" className="w-full">
        Continue
      </Button>
    </form>
  );

  const renderGeoLiteStep = () => (
    <form action="/setup/geolite" method="POST" className="space-y-4">
      <Alert className="border-amber-200 bg-amber-50">
        <AlertDescription className="text-amber-800">
          <strong>GeoLite Database Required for Event Processing</strong>
        </AlertDescription>
      </Alert>

      <div className="space-y-3 text-sm text-gray-600">
        <p>
          Fusionaly uses MaxMind's GeoLite2 database to detect visitor locations (country, city).
          <strong className="text-gray-900"> Without it, events will be queued but not processed.</strong>
        </p>

        <div className="bg-gray-50 p-3 rounded-md border">
          <p className="font-medium text-gray-900 mb-2">To enable event processing:</p>
          <ol className="list-decimal list-inside space-y-1">
            <li>Register at <a href="https://www.maxmind.com/en/geolite2/signup" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">MaxMind</a> (free account)</li>
            <li>Download GeoLite2-City.mmdb</li>
            <li>Go to <strong>Administration &rarr; System</strong> to configure the path</li>
          </ol>
        </div>

        <p className="text-gray-500">
          You can complete setup now and configure GeoLite later. Events will queue until GeoLite is configured.
        </p>
      </div>

      <Button type="submit" className="w-full">
        Complete Setup
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
      case 'geolite':
        return renderGeoLiteStep();
      case 'completed':
        return renderCompletedStep();
      default:
        return <div>Unknown step</div>;
    }
  };

  const getStepNumber = () => {
    const steps: Step[] = ['user_account', 'password', 'geolite', 'completed'];
    return steps.indexOf(currentStep) + 1;
  };

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
                  Step {getStepNumber()} of 4
                </CardDescription>
              </div>
              <div className="text-right text-sm text-gray-500">
                {stepProgress[currentStep]}%
              </div>
            </div>
            <Progress value={stepProgress[currentStep]} className="mt-4" />
          </CardHeader>

          <CardContent>
            {flash.error && (
              <Alert className="mb-4 border-red-200 bg-red-50">
                <AlertDescription className="text-red-800">{flash.error}</AlertDescription>
              </Alert>
            )}
            {flash.success && (
              <Alert className="mb-4 border-green-200 bg-green-50">
                <AlertDescription className="text-green-800">{flash.success}</AlertDescription>
              </Alert>
            )}
            {flash.info && (
              <Alert className="mb-4 border-blue-200 bg-blue-50">
                <AlertDescription className="text-blue-800">{flash.info}</AlertDescription>
              </Alert>
            )}

            {renderStepContent()}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
