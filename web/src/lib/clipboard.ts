// copyText copies text to the clipboard. navigator.clipboard exists only on
// HTTPS and localhost, so on plain HTTP (a LAN address) it falls back to the
// older execCommand("copy").
export async function copyText(text: string): Promise<void> {
	if (navigator.clipboard?.writeText) {
		await navigator.clipboard.writeText(text);
		return;
	}
	const area = document.createElement("textarea");
	area.value = text;
	area.setAttribute("readonly", "");
	area.style.position = "fixed";
	area.style.opacity = "0";
	document.body.appendChild(area);
	area.select();
	const copied = document.execCommand("copy");
	area.remove();
	if (!copied) throw new Error("copy failed");
}
