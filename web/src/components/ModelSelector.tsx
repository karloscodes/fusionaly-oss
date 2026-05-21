import React from "react";
import { Sparkles, Zap, Brain } from "lucide-react";
import { cn } from "@/lib/utils";

// Available AI models (Jan 2026)
export const AI_MODELS = [
  { id: "gpt-4.1", label: "Fast", icon: Zap },
  { id: "gpt-5.2", label: "Smart", icon: Sparkles },
  { id: "gpt-5.2-thinking", label: "Deep", icon: Brain },
] as const;

export type ModelId = typeof AI_MODELS[number]["id"];

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
    <div className="flex items-center gap-1 p-0.5 bg-black/5 rounded-lg w-fit">
      {AI_MODELS.map((m) => {
        const Icon = m.icon;
        const isSelected = value === m.id;
        return (
          <button
            key={m.id}
            type="button"
            onClick={() => onChange(m.id)}
            disabled={disabled}
            className={cn(
              "flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all",
              isSelected
                ? "bg-white text-black shadow-sm"
                : "text-black/60 hover:text-black",
              disabled && "opacity-50 cursor-not-allowed"
            )}
          >
            <Icon className="h-3 w-3" />
            <span>{m.label}</span>
          </button>
        );
      })}
    </div>
  );
};
