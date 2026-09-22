// Lens (Ask AI) is hidden while AI clients (MCP, skill) replace it. The pages
// still work. To bring the tabs back in one browser, run in the console:
//   localStorage.setItem("fusionaly:lens", "on")
export function isLensEnabled(): boolean {
	try {
		return window.localStorage.getItem("fusionaly:lens") === "on";
	} catch {
		return false;
	}
}
