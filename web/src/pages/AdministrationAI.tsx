import { useForm } from "@inertiajs/react"
import { AdministrationLayout } from "@/components/administration-layout"
import { Card, CardHeader, CardTitle, CardContent, CardDescription, CardFooter } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { FlashMessageDisplay } from "@/components/ui/flash-message"
import { Key, ExternalLink, CheckCircle2 } from "lucide-react"
import type { FlashMessage } from "@/types"

interface Setting {
  key: string
  value: string
}

interface AdministrationAIProps {
  settings?: Setting[]
  available_models?: string[]
  flash?: FlashMessage
  error?: string
}

export function AdministrationAI({ settings, available_models, flash, error }: AdministrationAIProps) {
  const openaiSetting = settings?.find((s) => s.key === "openai_api_key")
  const initialApiKey = openaiSetting?.value || ""
  const hasApiKey = initialApiKey.trim().startsWith("sk-") || initialApiKey.startsWith("*")

  const models = available_models ?? []
  const initialModel = settings?.find((s) => s.key === "ai_model")?.value || ""

  const settingsForm = useForm({
    openai_api_key: initialApiKey,
    ai_model: initialModel,
  })

  const currentHasApiKey = settingsForm.data.openai_api_key.trim().startsWith("sk-") ||
    settingsForm.data.openai_api_key.startsWith("*") ||
    (hasApiKey && settingsForm.data.openai_api_key === initialApiKey)

  const handleSettingsSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    settingsForm.post("/admin/administration/ai", {
      preserveScroll: true,
    })
  }

  return (
    <AdministrationLayout currentPage="ai">
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-black">AI Settings</h1>
          <p className="text-black/60 mt-1">
            Connect OpenRouter to enable Ask.
          </p>
        </div>

        <FlashMessageDisplay flash={flash} error={error} />

        <form onSubmit={handleSettingsSubmit}>
          <Card className="border-black shadow-sm">
            <CardHeader className="pb-4">
              <div className="flex justify-between items-center">
                <CardTitle className="text-lg flex items-center gap-2">
                  <Key className="h-5 w-5" /> OpenRouter API key
                </CardTitle>
              </div>
              <CardDescription>
                Configure your OpenRouter API key for Ask and Alerts features.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="bg-black/5 p-4 rounded-lg flex items-start gap-3 border">
                <div className="shrink-0 mt-0.5">
                  <Key className="h-4 w-4 text-black/60" />
                </div>
                <div className="text-sm text-black/70">
                  <p>
                    An OpenRouter API key is required to use Ask and generate alerts.
                    Get one from OpenRouter.
                  </p>
                  <a
                    href="https://openrouter.ai/keys"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm font-medium text-black/70 hover:text-black mt-2"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    Get your API key
                  </a>
                </div>
              </div>
              <div>
                <label
                  htmlFor="openai_api_key"
                  className="block text-sm font-medium mb-1.5"
                >
                  OpenRouter API key
                  {currentHasApiKey && (
                    <span className="inline-flex items-center ml-2 text-xs text-green-700 bg-green-100 px-1.5 py-0.5 rounded-md font-medium">
                      <CheckCircle2 className="h-3 w-3 mr-1" />
                      Key Set
                    </span>
                  )}
                </label>
                <div className="relative">
                  <Input
                    id="openai_api_key"
                    name="openai_api_key"
                    type="password"
                    placeholder={hasApiKey ? "Enter new key to replace existing" : "sk-or-..."}
                    value={settingsForm.data.openai_api_key}
                    onChange={(e) => settingsForm.setData("openai_api_key", e.target.value)}
                    disabled={settingsForm.processing}
                    className="w-full border-black/20 focus:border-black focus:ring-black rounded-md"
                  />
                </div>
                <p className="text-xs text-black/50 mt-1.5">
                  Your API key is stored securely and only used for AI features.
                </p>
                <p className="text-xs text-black/50 mt-1.5">
                  Ask AI is optional and uses your own OpenRouter key. It never
                  sends your visitors' data — only your database schema and the
                  questions you type are sent to OpenRouter (and the model
                  provider you choose there).
                </p>
              </div>
              <div>
                <label
                  htmlFor="ai_model"
                  className="block text-sm font-medium mb-1.5"
                >
                  Model
                </label>
                <Select
                  value={settingsForm.data.ai_model}
                  onValueChange={(value) => settingsForm.setData("ai_model", value)}
                  disabled={settingsForm.processing}
                >
                  <SelectTrigger
                    id="ai_model"
                    name="ai_model"
                    className="w-full border-black/20 focus:border-black focus:ring-black rounded-md"
                  >
                    <SelectValue placeholder="openai/gpt-4o-mini" />
                  </SelectTrigger>
                  <SelectContent>
                    {models.map((model) => (
                      <SelectItem key={model} value={model}>
                        {model}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </CardContent>
            <CardFooter className="flex justify-end border-t pt-4">
              <Button
                type="submit"
                disabled={settingsForm.processing}
                className="bg-black hover:bg-black/80 text-white rounded-md min-w-[140px]"
              >
                {settingsForm.processing ? "Saving..." : "Save API Key"}
              </Button>
            </CardFooter>
          </Card>
        </form>
      </div>
    </AdministrationLayout>
  )
}

export default AdministrationAI
