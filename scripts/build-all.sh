# scripts/build-all.sh

bash
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="${REPO_ROOT}/bin"
VERSION="${VERSION:-$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

mkdir -p "$BUILD_DIR"

TARGETS=(
    "tools/opsdb-schema/cmd"
    "tools/opsdb-api/cmd"
    "tools/importers/opsdb-import-aws/cmd"
    "tools/importers/opsdb-import-gcp/cmd"
    "tools/importers/opsdb-import-k8s/cmd"
    "tools/importers/opsdb-import-identity/cmd"
    "tools/importers/opsdb-import-monitoring/cmd"
    "tools/importers/opsdb-import-oncall/cmd"
    "tools/importers/opsdb-import-secrets/cmd"
    "tools/runners/change-set-executor/cmd"
    "tools/runners/emergency-review-monitor/cmd"
    "tools/runners/notification-runner/cmd"
    "tools/runners/reaper/cmd"
    "tools/runners/schema-executor/cmd"
)

FAILED=0
BUILT=0

echo "=== OpsDB Build ==="
echo "version: ${VERSION}"
echo "build time: ${BUILD_TIME}"
echo "output: ${BUILD_DIR}"
echo ""

for target in "${TARGETS[@]}"; do
    # binary name from parent directory of cmd/
    bin_name="$(basename "$(dirname "$target")")"
    # special case: cmd directly under a tool means the tool IS the binary
    parent="$(dirname "$target")"
    if [ "$(basename "$parent")" = "cmd" ]; then
        bin_name="$(basename "$(dirname "$parent")")"
    fi

    echo -n "building ${bin_name}... "
    if go build -ldflags "$LDFLAGS" -o "${BUILD_DIR}/${bin_name}" "./${target}"; then
        echo "ok"
        BUILT=$((BUILT + 1))
    else
        echo "FAILED"
        FAILED=$((FAILED + 1))
    fi
done

echo ""
echo "=== Results ==="
echo "built: ${BUILT}"
echo "failed: ${FAILED}"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi

echo ""
echo "all binaries in ${BUILD_DIR}:"
ls -lh "$BUILD_DIR"


