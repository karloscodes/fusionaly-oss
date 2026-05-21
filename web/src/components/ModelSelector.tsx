import React from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

// Curated short list of OpenRouter model ids. The server (internal/ai) threads
// the same curated list in via the `models` prop; this acts as the fallback.
// Keep in sync with internal/ai/ai.go AvailableModels.
export const AI_MODELS = [
  "openai/gpt-5.4-mini",
  "openai/gpt-5.5",
  "anthropic/claude-sonnet-4.6",
  "anthropic/claude-opus-4.7",
  "google/gemini-3.5-flash",
  "deepseek/deepseek-v4-pro",
  "moonshotai/kimi-k2.6",
  "minimax/minimax-m2.7",
] as const;

export type ModelId = string;

export const DEFAULT_MODEL: ModelId = "openai/gpt-5.4-mini";

interface ModelSelectorProps {
  value: string;
  onChange: (model: ModelId) => void;
  disabled?: boolean;
  /** Live OpenRouter model ids from the server. Falls back to AI_MODELS. */
  models?: string[];
}

// Use the live OpenRouter catalog when provided, otherwise the static fallback.
// Renders the project's standard shadcn Select so the picker is consistent with
// the rest of the app's dropdowns.
export const ModelSelector: React.FC<ModelSelectorProps> = ({
  value,
  onChange,
  disabled = false,
  models,
}) => {
  const options = models && models.length > 0 ? models : [...AI_MODELS];
  const selected = value || DEFAULT_MODEL;

  return (
    <Select value={selected} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger className="h-8 w-[220px] text-sm border-black/20 focus:border-black">
        <SelectValue placeholder={DEFAULT_MODEL} />
      </SelectTrigger>
      <SelectContent className="border-black">
        {options.map((model) => (
          <SelectItem key={model} value={model} className="text-sm">
            {model}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
};
