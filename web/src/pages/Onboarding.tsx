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

        <p className="text-sm text-black/60">
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

      <p className="text-sm text-black/60">
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
          <strong>GeoLite Database for Location Data</strong>
        </AlertDescription>
      </Alert>

      <div className="space-y-3 text-sm text-black/60">
        <p>
          Fusionaly uses MaxMind's GeoLite2 database to detect visitor locations (country, city).
          Enter your MaxMind credentials to enable automatic database downloads.
        </p>

        <div className="bg-black/5 p-3 rounded-md border">
          <p className="font-medium text-black mb-2">Get your free credentials:</p>
          <ol className="list-decimal list-inside space-y-1">
            <li>Register at <a href="https://www.maxmind.com/en/geolite2/signup" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">MaxMind</a> (free)</li>
            <li>Go to Account &rarr; Manage License Keys</li>
            <li>Generate a new license key</li>
          </ol>
        </div>
      </div>

      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="geolite_account_id">Account ID</Label>
          <Input
            id="geolite_account_id"
            type="text"
            name="geolite_account_id"
            placeholder="e.g., 123456"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="geolite_license_key">License Key</Label>
          <Input
            id="geolite_license_key"
            type="password"
            name="geolite_license_key"
            placeholder="Your MaxMind license key"
          />
        </div>
      </div>

      <p className="text-xs text-black/50">
        This step is optional. You can configure GeoLite later in Administration &rarr; System.
      </p>

      <div className="flex gap-2">
        <Button type="submit" name="action" value="skip" variant="outline" className="flex-1">
          Skip for Now
        </Button>
        <Button type="submit" name="action" value="save" className="flex-1">
          Save & Complete
        </Button>
      </div>
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
      <p className="text-sm text-black/60">
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
    <div className="min-h-screen bg-black/5 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md mx-auto">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-black">Initial Setup</h1>
          <p className="mt-2 text-black/60">Configure your Fusionaly installation</p>
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
              <div className="text-right text-sm text-black/50">
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
