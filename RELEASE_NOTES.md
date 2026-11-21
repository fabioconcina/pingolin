## Fixes

- **The web service no longer reports the connection as down after a
  reboot.** When systemd started both services together, the web service
  checked for the daemon before it had written its PID file, started its
  own prober without permission to send ICMP, and every ping failed. It now
  waits a few seconds for the daemon to come up. The web unit is also
  granted `CAP_NET_RAW`, so its fallback prober can ping if the daemon
  really isn't running. Existing installs pick up the unit change by
  re-running `sudo pingolin service install` and restarting
  `pingolin-web.service`.

- **`--ping-interval`, `--dns-interval`, `--http-interval` and `--retention`
  now take effect.** The flags were accepted but ignored, so the config file
  values (or the defaults) always won.

- **`pingolin history` prints its header rule again.** The line under the
  title came out as unprintable bytes instead of a row of box-drawing
  characters.

## Simpler dashboard

- **Sparklines always show the last hour.** The 1h/6h/24h/7d/30d range
  selector is gone. Longer ranges mostly surfaced old jitter spikes you
  couldn't act on, so the charts now stick to the last hour with -1h and
  now labels on the axis.

## Verifiable releases

- **Binaries are built in CI, with SBOMs and provenance attestations.**
  Each release still ships `pingolin-{os}-{arch}` for Linux and macOS on
  amd64 and arm64 plus `checksums.txt`, so existing download links keep
  working. It now also includes an SPDX SBOM per binary, and the checksums
  carry a GitHub build attestation you can check with
  `gh attestation verify pingolin-linux-amd64 --repo fabioconcina/pingolin`.
