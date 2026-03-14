# Feature: IP Range Blocking (CIDR)

## Goal
Let users block IP ranges (e.g., `85.23.45.0/24`) in addition to single IPs, so dynamic IPs from the same subnet are excluded without re-adding each one.

## Behavior
- User enters CIDR ranges or single IPs in the existing excluded_ips textarea
- Both `192.168.1.1` (single IP) and `85.23.45.0/24` (range) are valid entries
- Validation accepts valid IPs and valid CIDR notation, rejects garbage
- Events from IPs matching any range or single IP are silently dropped
- Placeholder and help text updated to show CIDR examples

## Acceptance
- [ ] `validateIPList()` accepts CIDR notation (e.g., `10.0.0.0/8`, `192.168.1.0/24`)
- [ ] `validateIPList()` rejects invalid CIDR (e.g., `10.0.0.0/33`, `foo/24`)
- [ ] `IsIPExcluded()` matches IPs against CIDR ranges
- [ ] Single IPs still work exactly as before
- [ ] Textarea placeholder shows CIDR example
- [ ] Help text explains range blocking
- [ ] Tests cover: single IP match, CIDR match, CIDR non-match, mixed list, invalid input

## Notes
- Go stdlib: `net.ParseCIDR()` for ranges, `net.ParseIP()` for singles
- `net.Contains()` checks if IP falls within CIDR range
- No new dependencies needed
