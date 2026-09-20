"use strict";

// Postinstall: fetch and cryptographically verify the hivex binary.
//
// Security model: the download is verified against the SHA256 listed in the
// release's checksums.txt. If the archive is tampered with, or the hash file
// is unreachable, the install MUST fail — silently continuing would allow a
// compromised release token to plant a backdoored binary on every machine
// that runs `npm install hivex`.
//
// Escape hatches (opt-in only):
//   HIVEX_SKIP_POSTINSTALL=1
//     Skip the download entirely. The bin/hivex.js shim will attempt an
//     (also-verified) download on first invocation. Use this for packaging
//     builds, offline mirrors, or CI images that restore a prebuilt bin/.
//
//   HIVEX_POSTINSTALL_SOFT_FAIL=1
//     Downgrade a *network* failure (e.g., GitHub unreachable behind a
//     corporate proxy) from fatal to a warning. SHA256 mismatches are ALWAYS
//     fatal and cannot be soft-failed — that path exists to catch tampering.

const { downloadBinary } = require("./download-binary");
const { downloadCloudflared } = require("./download-cloudflared");

if (process.env.HIVEX_SKIP_POSTINSTALL === "1") {
  process.stderr.write(
    "hivex: postinstall skipped via HIVEX_SKIP_POSTINSTALL=1\n",
  );
  process.exit(0);
}

// Cloudflared is BEST-EFFORT: a failure here must not block the hivex
// install, because tunnels are an optional feature and a corp proxy that
// blocks github.com release assets shouldn't make `npm install hivex` fail
// outright. The runtime path in cmd/hivex/tunnel.go already returns a clear
// "cloudflared is not installed" message when the user clicks Start tunnel,
// so a soft failure here just defers the install hint to that moment.
//
// Skip via HIVEX_SKIP_CLOUDFLARED=1 for offline builds and air-gapped CI
// images that prefer to ship without the bundled tunnel binary.
async function tryDownloadCloudflared() {
  if (process.env.HIVEX_SKIP_CLOUDFLARED === "1") {
    process.stderr.write(
      "hivex: cloudflared bundling skipped via HIVEX_SKIP_CLOUDFLARED=1\n",
    );
    return;
  }
  try {
    await downloadCloudflared();
  } catch (err) {
    const message = err && err.message ? err.message : String(err);
    process.stderr.write(
      `hivex: cloudflared bundle failed (${message}).\n` +
        `hivex: continuing — the Public Tunnel feature will surface a missing-binary error at runtime.\n`,
    );
  }
}

downloadBinary().then(tryDownloadCloudflared).catch((err) => {
  const message = err && err.message ? err.message : String(err);
  const isIntegrityFailure =
    message.includes("SHA256 mismatch") ||
    message.includes("Cannot verify download integrity");

  // Integrity failures are ALWAYS fatal. No soft-fail, no retry-on-first-run.
  if (isIntegrityFailure) {
    process.stderr.write(
      `\nhivex: SECURITY: ${message}\n` +
        `hivex: aborting install. No binary has been placed in bin/.\n\n`,
    );
    process.exit(1);
  }

  // Non-integrity failures (network, DNS, disk, unsupported platform).
  if (process.env.HIVEX_POSTINSTALL_SOFT_FAIL === "1") {
    process.stderr.write(
      `hivex: postinstall download failed (${message}).\n` +
        `hivex: continuing because HIVEX_POSTINSTALL_SOFT_FAIL=1 is set. ` +
        `The binary will be fetched (and verified) on first run.\n`,
    );
    process.exit(0);
  }

  process.stderr.write(
    `\nhivex: postinstall download failed: ${message}\n` +
      `hivex: set HIVEX_POSTINSTALL_SOFT_FAIL=1 to downgrade this to a ` +
      `warning, or HIVEX_SKIP_POSTINSTALL=1 to skip the download entirely.\n\n`,
  );
  process.exit(1);
});
