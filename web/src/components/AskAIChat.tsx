import React, { useState, FormEvent, useRef, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Send, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { ModelSelector, ModelId } from "./ModelSelector";

interface AskAIChatProps {
  onSubmit: (question: string, websiteId: number, model: string) => void | Promise<void>;
  isLoading: boolean;
  websiteId: number;
  selectedModel?: string;
  onModelChange?: (model: string) => void;
  aiResult?: {
    question: string;
    query: string;
    results: Record<string, unknown>[];
    queryType: string;
    vegaSpec?: string;
    summary?: string;
    followUps?: string[];
    websiteId: number;
  } | null;
  onSaveResult?: () => void | Promise<void>;
  isSavingResult?: boolean;
  renderResults?: (results: Record<string, unknown>[], queryType: string, vegaSpec?: string) => React.ReactNode;
  /** External question to set in the input field (for help examples) */
  externalQuestion?: string;
  /** Callback when the external question has been consumed */
  onExternalQuestionConsumed?: () => void;
  /** Whether to clear the input field */
  shouldClearInput?: boolean;
  /** Callback when the input has been cleared */
  onInputCleared?: () => void;
  /** Callback when the input changes */
  onInputChange?: () => void;
}

const EXAMPLE_QUESTIONS = [
  "Compare this week vs last week",
  "Where is my traffic coming from?",
  "Which pages are losing visitors?",
  "Show traffic by day of week",
];

// Simple SQL formatter for readability
function formatSQL(sql: string): string {
  const keywords = ['SELECT', 'FROM', 'WHERE', 'GROUP BY', 'ORDER BY', 'HAVING', 'LIMIT', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'JOIN', 'AND', 'OR'];
  let formatted = sql.trim();

  // Add newlines before major keywords
  keywords.forEach(keyword => {
    const regex = new RegExp(`\\s+${keyword}\\s+`, 'gi');
    formatted = formatted.replace(regex, `\n${keyword} `);
  });

  // Handle comma-separated columns (add newline + indent after SELECT)
  formatted = formatted.replace(/^SELECT\s+/i, 'SELECT\n  ');
  formatted = formatted.replace(/,\s*(?=[a-zA-Z_])/g, ',\n  ');

  return formatted.trim();
}

export const AskAIChat: React.FC<AskAIChatProps> = ({
  onSubmit,
  isLoading,
  websiteId,
  selectedModel = "gpt-5.2",
  onModelChange,
  aiResult,
  onSaveResult,
  isSavingResult,
  renderResults,
  externalQuestion,
  onExternalQuestionConsumed,
  shouldClearInput,
  onInputCleared,
  onInputChange,
}) => {
  const [question, setQuestion] = useState("");
  const [model, setModel] = useState<ModelId>(selectedModel as ModelId);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Sync external model changes
  useEffect(() => {
    if (selectedModel && selectedModel !== model) {
      setModel(selectedModel as ModelId);
    }
  }, [selectedModel]);

  const handleModelChange = (newModel: ModelId) => {
    setModel(newModel);
    onModelChange?.(newModel);
  };

  // Auto-resize textarea
  useEffect(() => {
    const textarea = textareaRef.current;
    if (textarea) {
      textarea.style.height = "auto";
      textarea.style.height = `${textarea.scrollHeight}px`;
    }
  }, [question]);

  // Handle external question (from help examples)
  useEffect(() => {
    if (externalQuestion && externalQuestion !== question) {
      setQuestion(externalQuestion);
      onExternalQuestionConsumed?.();
    }
  }, [externalQuestion, onExternalQuestionConsumed]);

  // Handle clear input signal (after saving)
  useEffect(() => {
    if (shouldClearInput) {
      setQuestion("");
      onInputCleared?.();
    }
  }, [shouldClearInput, onInputCleared]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!question.trim() || isLoading) return;

    await onSubmit(question.trim(), websiteId, model);
    // Keep the text in textarea after submit
  };

  const handleExampleClick = async (exampleQuestion: string) => {
    if (isLoading) return;
    setQuestion(exampleQuestion);
    await onSubmit(exampleQuestion, websiteId, model);
  };

  return (
    <div className={cn("w-full", aiResult ? "mb-6" : "")}>
      <form onSubmit={handleSubmit} className={cn(aiResult ? "mb-0" : "mb-6")}>
        {/* Chat input card */}
        <div className={cn(
          "border border-black bg-white shadow-sm overflow-hidden",
          aiResult ? "rounded-t-lg" : "rounded-lg"
        )}>
          {/* Question input - more space */}
          <div className="px-6 pt-4 pb-3">
            <textarea
              ref={textareaRef}
              value={question}
              onChange={(e) => {
                setQuestion(e.target.value);
                onInputChange?.();
              }}
              placeholder="Ask questions in natural language • ⌘/Ctrl + Enter to send"
              className="w-full border-0 focus:ring-0 focus:outline-none resize-none text-base placeholder:text-black/40 bg-transparent min-h-[50px] max-h-[200px]"
              disabled={isLoading}
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault();
                  handleSubmit(e);
                }
              }}
            />
          </div>

          {/* Gutter: Model selector + Ask button */}
          <div className="flex items-center justify-between px-6 py-3 border-t border-black/10">
            <ModelSelector
              value={model}
              onChange={handleModelChange}
              disabled={isLoading}
            />

            <Button
              type="submit"
              disabled={!question.trim() || isLoading}
              className="bg-black hover:bg-black/80 text-white font-medium px-5 py-1.5 rounded-md text-sm disabled:bg-black/20 disabled:text-black/40 disabled:cursor-not-allowed"
            >
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                  Analyzing...
                </>
              ) : (
                <>
                  <Send className="mr-2 h-3.5 w-3.5" />
                  Ask
                </>
              )}
            </Button>
          </div>
        </div>
      </form>

      {/* AI Result Display - Integrated into chat */}
      {aiResult && renderResults && (
        <div className="border-x border-b border-black rounded-b-lg bg-white overflow-hidden">
          {/* Summary - Plain English explanation */}
          {aiResult.summary && (
            <div className="px-6 pt-5 pb-2">
              <p className="text-black text-sm leading-relaxed">{aiResult.summary}</p>
            </div>
          )}
          {/* Results Display */}
          <div className="p-6 border-t border-black/10">
            {renderResults(aiResult.results, aiResult.queryType, aiResult.vegaSpec)}
          </div>

          {/* Follow-up suggestions */}
          {aiResult.followUps && aiResult.followUps.length > 0 && (
            <div className="px-6 py-3 border-t border-black/10">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-xs font-medium text-black/70">Follow up:</span>
                {aiResult.followUps.map((followUp, index) => (
                  <button
                    key={index}
                    onClick={() => {
                      setQuestion(followUp);
                      onSubmit(followUp, websiteId, model);
                    }}
                    disabled={isLoading}
                    className="text-xs px-3 py-1.5 border border-black/20 text-black hover:border-black hover:bg-black/5 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {followUp}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Action bar */}
          <div className="px-4 py-3 border-t border-black/10">
            <div className="flex items-center justify-between gap-3">
              {/* SQL toggle */}
              <details className="group flex-1">
                <summary className="cursor-pointer text-xs text-black/60 hover:text-black flex items-center">
                  <svg
                    className="w-3 h-3 mr-1.5 transition-transform group-open:rotate-90"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                  </svg>
                  <span>View SQL</span>
                </summary>
                <div className="mt-2">
                  <pre className="bg-black rounded p-3 overflow-x-auto font-mono text-xs leading-relaxed text-[#00D678] whitespace-pre-wrap">
                    {formatSQL(aiResult.query)}
                  </pre>
                </div>
              </details>

              {/* Save button */}
              <Button
                onClick={onSaveResult}
                disabled={isSavingResult}
                variant="outline"
                className="text-xs border-black text-black hover:bg-black hover:text-white"
              >
                {isSavingResult ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Example questions - Dashboard style (black/white) */}
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-black/70">Try:</span>
        {EXAMPLE_QUESTIONS.slice(0, 4).map((example, index) => (
          <button
            key={index}
            onClick={() => handleExampleClick(example)}
            disabled={isLoading}
            className="text-xs px-3 py-1.5 bg-black text-white hover:bg-black/80 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
            {example}
          </button>
        ))}
      </div>
    </div>
  );
};
