import { useEffect, useState } from "react";
import { Box, Cat, Check, Moon, Sun, Terminal } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { THEMES, Theme, getTheme, applyTheme } from "@/lib/theme";

const ICONS: Record<Theme, typeof Sun> = {
  light: Sun,
  dark: Moon,
  phosphor: Terminal,
  catppuccin: Cat,
  gruvbox: Box,
};

export function ThemeSwitcher() {
  const [theme, setTheme] = useState<Theme>("light");

  // Read the already-applied theme (set by the no-flash script) on mount.
  useEffect(() => setTheme(getTheme()), []);

  const choose = (next: Theme) => {
    applyTheme(next);
    setTheme(next);
  };

  const ActiveIcon = ICONS[theme];

  // modal={false}: a modal Radix overlay locks body scroll, which shifts
  // centered page content sideways. The non-modal menu avoids that drift.
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        className="flex items-center text-gray-500 hover:text-gray-900 transition-colors focus:outline-none"
        title="Theme"
        aria-label="Switch theme"
      >
        <ActiveIcon className="w-4 h-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-36">
        {THEMES.map(({ id, label }) => {
          const Icon = ICONS[id];
          return (
            <DropdownMenuItem
              key={id}
              onClick={() => choose(id)}
              className="flex items-center justify-between cursor-pointer"
            >
              <span className="flex items-center gap-2">
                <Icon className="w-4 h-4" />
                {label}
              </span>
              {id === theme && <Check className="w-4 h-4 text-green-600" />}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
