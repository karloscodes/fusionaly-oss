# Security: Sec-Fetch-Site Protection

## Overview

Fusionaly implements **strict Sec-Fetch-Site header validation** to prevent server-to-server spoofing attacks on event ingestion endpoints (`/x/api/v1/events` and `/x/api/v1/events/beacon`).

## What is Sec-Fetch-Site?

`Sec-Fetch-Site` is a browser-set HTTP header that indicates the relationship between the request initiator's origin and the target origin. **Crucially, this header:**

- ✅ Is **automatically set by modern browsers**
- ✅ **Cannot be spoofed** by JavaScript (it's a "forbidden header")
- ✅ **Cannot be set** by server-to-server tools (curl, Postman, Python requests, etc.)
- ✅ Provides strong protection against CSRF and spoofing attacks

### Header Values

| Value | Meaning | Analytics Use Case |
|-------|---------|-------------------|
| `cross-site` | Request from different site | ✅ **Allowed** - Tracking script loaded from your website |
| `same-site` | Request from same site | ✅ **Allowed** - Same site, different subdomain |
| `same-origin` | Request from same origin | ✅ **Allowed** - Same origin request |
| `none` | Direct navigation | ✅ **Allowed** - User typed URL, bookmark |
| *missing* | No header present | ❌ **BLOCKED** - Server-to-server request |

## Protection Mechanism

### Two-Layer Defense

1. **Strict Presence Check**: Rejects POST requests without `Sec-Fetch-Site` header
2. **Value Validation**: Ensures header value is legitimate (using cartridge middleware)

```go
// Strict validation: reject if Sec-Fetch-Site is missing
strictSecFetchCheck := func(c *fiber.Ctx) error {
    if c.Method() == "POST" && c.Get("Sec-Fetch-Site") == "" {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "browser requests only",
        })
    }
    return c.Next()
}

// Value validation using cartridge middleware
secFetchForEvents := cartridgemiddleware.SecFetchSiteMiddleware(
    cartridgemiddleware.SecFetchSiteConfig{
        AllowedValues: []string{"cross-site", "same-site", "same-origin", "none"},
        Methods:       []string{"POST"},
    },
)
```

### What Gets Blocked

All server-to-server requests are automatically blocked:

- ❌ `curl` commands
- ❌ Postman/Insomnia API clients
- ❌ Python requests library
- ❌ Node.js fetch/axios
- ❌ wget/httpie
- ❌ Custom scripts trying to inject fake analytics
- ❌ Bot traffic attempting to manipulate metrics

### What Gets Allowed

Only legitimate browser requests:

- ✅ Your tracking script (loaded via `<script>` tag)
- ✅ `navigator.sendBeacon()` calls
- ✅ `fetch()` / `XMLHttpRequest` from browser
- ✅ All modern browsers (Chrome, Firefox, Safari, Edge)

## Browser Support

| Browser | Sec-Fetch-Site Support | Status |
|---------|----------------------|--------|
| Chrome 76+ | ✅ Yes | Supported |
| Edge 79+ | ✅ Yes | Supported |
| Firefox 90+ | ✅ Yes | Supported |
| Safari 15.5+ | ✅ Yes | Supported |
| Opera 63+ | ✅ Yes | Supported |
| Older browsers | ❌ No | **Blocked** |

**Note**: This is intentional. For self-hosted analytics where security is critical, we prioritize preventing spoofing over supporting ancient browsers.

## Testing

Run the security tests:

```bash
go test ./api/v1 -run "TestSecFetchSiteProtection|TestServerToServerBlocking" -v
```

### Example Test Output

```
✅ Successfully allowed: Legitimate browser request from tracked website
✅ Successfully allowed: Browser request from same site (subdomain)
✅ Successfully allowed: Browser request from same origin
✅ Successfully blocked: curl command with spoofed Origin header
✅ Successfully blocked: Postman API client
✅ Successfully blocked: Python script using requests library
✅ Successfully blocked: Node.js server-side fetch
✅ Successfully blocked: wget command
```

## Why This Matters for Self-Hosting

For self-hosted analytics where you control the deployment:

1. **Prevents Metric Manipulation**: Competitors/malicious actors can't inflate your numbers
2. **Protects Data Integrity**: Only real browser traffic is tracked
3. **GDPR/Privacy Compliant**: No need to trust external validation services
4. **Zero Additional Configuration**: Works out of the box
5. **Performance**: No extra database lookups or API calls

## Comparison with Other Approaches

| Approach | Spoofing Protection | Browser Compatibility | Complexity |
|----------|-------------------|---------------------|------------|
| **Sec-Fetch-Site (ours)** | ✅ Excellent | Modern browsers only | Low |
| API Keys | ✅ Excellent | ✅ All browsers | Medium (key management) |
| Origin Header Only | ⚠️ Weak (easily spoofed) | ✅ All browsers | Low |
| CORS Only | ❌ None (client-side) | ✅ All browsers | Low |
| IP Whitelist | ⚠️ Medium (VPNs, proxies) | ✅ All browsers | High (maintenance) |

## Trade-offs

### Pros ✅
- **Maximum security** against server-to-server spoofing
- **No key management** overhead
- **No database lookups** (header validation only)
- **Works perfectly** with modern browsers

### Cons ⚠️
- **Blocks old browsers** (pre-2020)
- **Not suitable** for server-side tracking (intentional)
- **No fallback** for non-browser clients

For self-hosted environments where you control the deployment and prioritize data integrity, this is the **optimal solution**.

## Related Protections

This works in combination with:
1. **Origin validation** (checks against registered domains)
2. **Rate limiting** (100 requests/minute per IP)
3. **CORS** (prevents credential theft)
4. **Website registration** (domain must exist in database)

## References

- [MDN: Sec-Fetch-Site](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Sec-Fetch-Site)
- [W3C Fetch Metadata Specification](https://www.w3.org/TR/fetch-metadata/)
- [Google: Protect your resources from web attacks](https://web.dev/fetch-metadata/)
