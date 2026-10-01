// faviconUrl points at Google's favicon image directly. The usual
// www.google.com/s2/favicons URL answers with a redirect that has no
// Cross-Origin-Resource-Policy header, so our Cross-Origin-Embedder-Policy
// blocks it. This final address sends CORP: cross-origin and loads.
export function faviconUrl(domain: string): string {
  const site = encodeURIComponent(`https://${domain}`)
  return `https://t1.gstatic.com/faviconV2?client=SOCIAL&type=FAVICON&fallback_opts=TYPE,SIZE,URL&url=${site}&size=32`
}
