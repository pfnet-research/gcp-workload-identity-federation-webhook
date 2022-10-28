#!/usr/bin/env bash

# If the machine is an Apple Silicon Mac (darwin, arm64), this helper outputs amd64
# as envtest for (darwin, arm64) is not officially released at the time of writing.
# This helper intends developers to use envtest for (darwin, arm64) via Rosetta.

set -euo pipefail

ORIGINAL_OS=$(GOOS="" go env GOOS)
ORIGINAL_ARCH=$(GOARCH="" go env GOARCH)

if [[ ${ORIGINAL_OS} == "darwin" && ${ORIGINAL_ARCH} == "arm64" ]]; then
  echo amd64
else
  echo ${ORIGINAL_ARCH}
fi
