import React from "react";
import { Check, ChevronDown } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

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
  /** Model ids from the server. Falls back to AI_MODELS. */
  models?: string[];
}

// Looks like the app's Select trigger, but uses a DropdownMenu with
// modal={false}. Radix Select has no way to disable its scroll-lock, which
// shifts centered page content sideways on scrollable pages; the non-modal
// DropdownMenu (same as the home feed) avoids that.
export const ModelSelector: React.FC<ModelSelectorProps> = ({
  value,
  onChange,
  disabled = false,
  models,
}) => {
  const options = models && models.length > 0 ? models : [...AI_MODELS];
  const selected = value || DEFAULT_MODEL;

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild disabled={disabled}>
        <button
          type="button"
          disabled={disabled}
          className="flex h-8 w-[220px] items-center justify-between rounded-md border border-black/20 bg-white px-3 text-sm focus:border-black focus:outline-none disabled:opacity-50"
        >
          <span className="truncate">{selected}</span>
          <ChevronDown className="h-4 w-4 opacity-50 shrink-0" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="w-[220px] max-h-72 overflow-y-auto border-black"
      >
        {options.map((model) => (
          <DropdownMenuItem
            key={model}
            onClick={() => onChange(model)}
            className="flex items-center justify-between text-sm"
          >
            <span className="truncate">{model}</span>
            {model === selected && <Check className="h-4 w-4 shrink-0" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
