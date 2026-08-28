#!/usr/bin/env node
// The command `npx cs-lint` runs. It finds the binary for this machine and
// becomes it: same arguments, same streams, same exit status.
//
// Nothing here interprets the run. cs-lint separates three exits — 0 nothing
// found, 1 something found, 2 could not run at all — and a gate reads all
// three, so a launcher that collapsed them would report a broken run as a
// passing one. The status is passed through untouched, and the launcher's own
// failures exit 2, which is the code that already means the linter did not get
// to run.

import { spawnSync } from "node:child_process";
import { constants } from "node:os";
import { binaryPath } from "../index.mjs";

// The exit code cs-lint uses for a run that could not happen. A launcher that
// cannot find its binary is one of those.
const COULD_NOT_RUN = 2;

let bin;
try {
  bin = binaryPath();
} catch (err) {
  console.error(`cs-lint: ${err.message}`);
  process.exit(COULD_NOT_RUN);
}

const result = spawnSync(bin, process.argv.slice(2), {
  // The linter writes a report for a person and reads nothing, so the streams
  // are the parent's. Piping them would buffer the report until the run ended
  // and drop the colours a terminal would have got.
  stdio: "inherit",
  // Large reports on stdout are the normal case for a first run against a
  // repository that has never been linted.
  maxBuffer: Infinity,
});

if (result.error) {
  const { code } = result.error;
  const hint =
    code === "ENOENT"
      ? "the file is missing; reinstall with `rm -rf node_modules && npm install`"
      : code === "EACCES"
        ? "the file is not executable; some archive tools drop the permission bit"
        : result.error.message;
  console.error(`cs-lint: cannot run ${bin}: ${hint}`);
  process.exit(COULD_NOT_RUN);
}

// A binary killed by a signal has no exit code. Reporting the shell's
// 128 + signal keeps a Ctrl-C from reading as a clean run, which is what a
// bare `process.exit(result.status)` would produce, since status is null here.
if (result.signal) {
  console.error(`cs-lint: killed by ${result.signal}`);
  process.exit(128 + (constants.signals[result.signal] ?? 0));
}

process.exit(result.status ?? COULD_NOT_RUN);
