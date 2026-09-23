// One pill style for every tab and toggle on the dashboard, as in the design:
// 28px high, 12px text, 8px corners.
export const tabClass = (active: boolean) =>
  `inline-flex items-center h-7 px-2.5 text-xs font-medium border rounded-lg whitespace-nowrap transition-colors ${
    active ? "bg-black text-white border-black" : "bg-white text-gray-600 hover:text-black"
  }`;
