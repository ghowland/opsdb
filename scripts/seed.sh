# scripts/seed.sh

bash
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

usage() {
    echo "usage: seed.sh --dos <dos-directory> --api <api-endpoint>"
    echo ""
    echo "Seeds an OpsDB instance with initial data from a DOS seed directory."
    echo "Requires the API to be running and the schema to be applied."
    echo ""
    echo "options:"
    echo "  --dos     path to DOS directory (e.g. dos/opsdb-ops-staging)"
    echo "  --api     OpsDB API endpoint (e.g. http://localhost:8080)"
    echo "  --token   auth token (or set OPSDB_AUTH_TOKEN env var)"
    echo "  --dry-run print what would be seeded without writing"
    exit 1
}

DOS_DIR=""
API_ENDPOINT=""
AUTH_TOKEN="${OPSDB_AUTH_TOKEN:-}"
DRY_RUN=false

while [ $# -gt 0 ]; do
    case "$1" in
        --dos)    DOS_DIR="$2"; shift 2 ;;
        --api)    API_ENDPOINT="$2"; shift 2 ;;
        --token)  AUTH_TOKEN="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        *)        usage ;;
    esac
done

if [ -z "$DOS_DIR" ] || [ -z "$API_ENDPOINT" ]; then
    usage
fi

SEED_DIR="${DOS_DIR}/seed"
if [ ! -d "$SEED_DIR" ]; then
    echo "error: seed directory not found at ${SEED_DIR}" >&2
    exit 1
fi

if [ -z "$AUTH_TOKEN" ]; then
    echo "error: auth token required (--token or OPSDB_AUTH_TOKEN)" >&2
    exit 1
fi

# check API is reachable
echo "checking API at ${API_ENDPOINT}..."
if ! curl -sf -o /dev/null "${API_ENDPOINT}/healthz" 2>/dev/null; then
    echo "error: API not reachable at ${API_ENDPOINT}" >&2
    exit 1
fi
echo "API is up."

# seed files are applied in lexicographic order
# convention: site.yaml first, then admin_user, then policies, then runner specs, then service accounts
SEED_FILES=($(find "$SEED_DIR" -name '*.yaml' -type f | sort))

if [ ${#SEED_FILES[@]} -eq 0 ]; then
    echo "no seed files found in ${SEED_DIR}"
    exit 0
fi

echo ""
echo "=== Seeding from ${SEED_DIR} ==="
echo "files to process: ${#SEED_FILES[@]}"
echo ""

SEEDED=0
FAILED=0

for seed_file in "${SEED_FILES[@]}"; do
    filename="$(basename "$seed_file")"
    echo -n "seeding ${filename}... "

    if [ "$DRY_RUN" = true ]; then
        echo "dry-run (skipped)"
        SEEDED=$((SEEDED + 1))
        continue
    fi

    # each seed file is a YAML document with:
    #   entity_type: <type>
    #   records:
    #     - field1: value1
    #       field2: value2
    # we convert to JSON and POST each record as a direct write (seed bypasses change management)

    entity_type="$(grep -m1 '^entity_type:' "$seed_file" | sed 's/^entity_type:[[:space:]]*//')"
    if [ -z "$entity_type" ]; then
        echo "SKIPPED (no entity_type found)"
        continue
    fi

    # extract records using a simple approach: find each "- " block under records:
    # for proper YAML parsing we would use yq, but keeping deps minimal
    # each record is POSTed as a JSON object to the seed endpoint
    record_count=0

    # use python3 if available for reliable YAML->JSON, fall back to curl with raw YAML
    if command -v python3 &>/dev/null; then
        python3 -c "
import yaml, json, sys
with open('${seed_file}') as f:
    doc = yaml.safe_load(f)
records = doc.get('records', [])
for r in records:
    print(json.dumps({'entity_type': '${entity_type}', 'record': r}))
" 2>/dev/null | while read -r json_line; do
            http_code=$(curl -sf -o /dev/null -w "%{http_code}" \
                -X POST \
                -H "Authorization: Bearer ${AUTH_TOKEN}" \
                -H "Content-Type: application/json" \
                -d "$json_line" \
                "${API_ENDPOINT}/api/v1/seed" 2>/dev/null || echo "000")
            if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
                record_count=$((record_count + 1))
            else
                echo "warning: HTTP ${http_code} seeding record to ${entity_type}" >&2
            fi
        done
        echo "ok (${entity_type})"
        SEEDED=$((SEEDED + 1))
    else
        # no python3: post the whole file as-is and let the API parse YAML
        http_code=$(curl -sf -o /dev/null -w "%{http_code}" \
            -X POST \
            -H "Authorization: Bearer ${AUTH_TOKEN}" \
            -H "Content-Type: application/x-yaml" \
            --data-binary "@${seed_file}" \
            "${API_ENDPOINT}/api/v1/seed" 2>/dev/null || echo "000")
        if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
            echo "ok (${entity_type})"
            SEEDED=$((SEEDED + 1))
        else
            echo "FAILED (HTTP ${http_code})"
            FAILED=$((FAILED + 1))
        fi
    fi
done

echo ""
echo "=== Seed Results ==="
echo "seeded: ${SEEDED}"
echo "failed: ${FAILED}"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi


