## Fixes

- **Outages now get the right cause.** pingolin used to decide the cause
  the moment an outage opened, from the last DNS and HTTP results it had.
  Those came from before the outage, so they were almost always successes,
  and nearly every outage was labelled `icmp_blocked`, even a full loss of
  the internet connection. The cause is now based only on the DNS and HTTP
  probes that ran during the outage. It starts as `connection_down`, is
  rechecked every minute while the outage lasts, and is settled when it
  ends. Lookups through the `system` resolver no longer count, because a
  local cache such as Pi-hole keeps answering with stale records while the
  uplink is down. Outages recorded before this release keep their old
  label.
