import React from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

// Available AI models for Ask (Lens). These are OpenRouter model ids and must
// match internal/ai/ai.go AvailableModels. They change over time — verify
// against https://openrouter.ai/models.
export const AI_MODELS = [
  "openai/gpt-4o-mini",
  "openai/gpt-4.1-mini",
  "openai/gpt-4o",
  "anthropic/claude-3.5-haiku",
  "anthropic/claude-3.7-sonnet",
  "anthropic/claude-sonnet-4",
  "deepseek/deepseek-chat",
  "deepseek/deepseek-r1",
  "moonshotai/kimi-k2",
  "minimax/minimax-01",
] as const;

export type ModelId = (typeof AI_MODELS)[number];

export const DEFAULT_MODEL: ModelId = "openai/gpt-4o-mini";

interface ModelSelectorProps {
  value: string;
  onChange: (model: ModelId) => void;
  disabled?: boolean;
}

export const ModelSelector: React.FC<ModelSelectorProps> = ({
  value,
  onChange,
  disabled = false,
}) => {
  return (
    <Select
      value={value}
      onValueChange={(v) => onChange(v as ModelId)}
      disabled={disabled}
    >
      <SelectTrigger className="h-8 w-[220px] text-xs border-black/20 focus:border-black focus:ring-black rounded-md">
        <SelectValue placeholder={DEFAULT_MODEL} />
      </SelectTrigger>
      <SelectContent>
        {AI_MODELS.map((model) => (
          <SelectItem key={model} value={model} className="text-xs">
            {model}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
};
