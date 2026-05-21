import React from "react";

// Static fallback list of OpenRouter model ids. The live catalog is fetched on
// the server (internal/ai) and threaded in via the `models` prop, so this only
// shows up when that fetch fails (or in tests). Keep in sync with
// internal/ai/ai.go AvailableModels.
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

export type ModelId = string;

export const DEFAULT_MODEL: ModelId = "openai/gpt-4o-mini";

interface ModelSelectorProps {
  value: string;
  onChange: (model: ModelId) => void;
  disabled?: boolean;
  /** Live OpenRouter model ids from the server. Falls back to AI_MODELS. */
  models?: string[];
}

// Use the live OpenRouter catalog when provided, otherwise the static fallback.
// An input + <datalist> means the list is always current AND the user can type
// any exact id OpenRouter offers (e.g. a specific kimi/claude version).
export const ModelSelector: React.FC<ModelSelectorProps> = ({
  value,
  onChange,
  disabled = false,
  models,
}) => {
  const options = models && models.length > 0 ? models : [...AI_MODELS];
  const listId = React.useId();

  return (
    <>
      <input
        type="text"
        list={listId}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        placeholder={DEFAULT_MODEL}
        spellCheck={false}
        autoComplete="off"
        className="h-8 w-[220px] text-xs border border-black/20 focus:border-black focus:ring-1 focus:ring-black focus:outline-none rounded-md px-2 bg-white disabled:bg-black/5 disabled:cursor-not-allowed"
      />
      <datalist id={listId}>
        {options.map((model) => (
          <option key={model} value={model} />
        ))}
      </datalist>
    </>
  );
};
