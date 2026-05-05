# scripts/generate-entity-catalog.sh

bash
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCHEMA_DIR="${REPO_ROOT}/schema"
DIRECTORY_YAML="${SCHEMA_DIR}/directory.yaml"
OUTPUT="${REPO_ROOT}/docs/reference/entity-catalog.md"

if [ ! -f "$DIRECTORY_YAML" ]; then
    echo "error: directory.yaml not found at ${DIRECTORY_YAML}" >&2
    exit 1
fi

echo "generating entity catalog from schema YAML files..."

cat > "$OUTPUT" <<'HEADER'
# Entity Catalog

Auto-generated from schema YAML files. Do not edit manually.
Re-generate with: `scripts/generate-entity-catalog.sh`

HEADER

ENTITY_COUNT=0
FIELD_COUNT=0
CURRENT_DOMAIN=""

# parse directory.yaml imports list and process each entity file
# uses grep+sed to avoid yq dependency
grep -E '^\s*-\s+' "$DIRECTORY_YAML" | sed 's/^[[:space:]]*-[[:space:]]*//' | while read -r rel_path; do
    filepath="${SCHEMA_DIR}/${rel_path}"
    if [ ! -f "$filepath" ]; then
        echo "warning: ${rel_path} listed in directory.yaml but file not found" >&2
        continue
    fi

    # extract domain from path: domains/01_identity/site.yaml -> 01_identity
    domain="$(echo "$rel_path" | cut -d'/' -f2)"
    if [ "$domain" != "$CURRENT_DOMAIN" ]; then
        CURRENT_DOMAIN="$domain"
        # strip numeric prefix for display: 01_identity -> Identity
        domain_display="$(echo "$domain" | sed 's/^[0-9]*_//' | sed 's/_/ /g' | sed 's/\b\(.\)/\u\1/g')"
        echo "" >> "$OUTPUT"
        echo "## ${domain_display}" >> "$OUTPUT"
        echo "" >> "$OUTPUT"
    fi

    # extract entity metadata from YAML using grep/sed (no yq dependency)
    entity_name="$(grep -m1 '^name:' "$filepath" | sed 's/^name:[[:space:]]*//')"
    description="$(grep -m1 '^description:' "$filepath" | sed 's/^description:[[:space:]]*//')"
    category="$(grep -m1 '^category:' "$filepath" | sed 's/^category:[[:space:]]*//')"
    versioned="$(grep -m1 '^versioned:' "$filepath" | sed 's/^versioned:[[:space:]]*//')"
    soft_delete="$(grep -m1 '^soft_delete:' "$filepath" | sed 's/^soft_delete:[[:space:]]*//')"
    hierarchical="$(grep -m1 '^hierarchical:' "$filepath" | sed 's/^hierarchical:[[:space:]]*//')"
    append_only="$(grep -m1 '^append_only:' "$filepath" | sed 's/^append_only:[[:space:]]*//')"

    if [ -z "$entity_name" ]; then
        echo "warning: no name found in ${rel_path}" >&2
        continue
    fi

    # count fields (lines that start with "  - name:" under fields section)
    num_fields="$(grep -c '^\s*- name:' "$filepath" 2>/dev/null || echo 0)"

    # build flags string
    flags=""
    [ "$versioned" = "true" ] && flags="${flags} versioned"
    [ "$soft_delete" = "true" ] && flags="${flags} soft-delete"
    [ "$hierarchical" = "true" ] && flags="${flags} hierarchical"
    [ "$append_only" = "true" ] && flags="${flags} append-only"
    flags="$(echo "$flags" | sed 's/^ //')"

    echo "### ${entity_name}" >> "$OUTPUT"
    echo "" >> "$OUTPUT"
    [ -n "$description" ] && echo "${description}" >> "$OUTPUT" && echo "" >> "$OUTPUT"
    echo "- **Category:** ${category}" >> "$OUTPUT"
    echo "- **Fields:** ${num_fields}" >> "$OUTPUT"
    [ -n "$flags" ] && echo "- **Flags:** ${flags}" >> "$OUTPUT"
    echo "- **Source:** \`${rel_path}\`" >> "$OUTPUT"
    echo "" >> "$OUTPUT"

    # list fields with type
    if [ "$num_fields" -gt 0 ]; then
        echo "| Field | Type |" >> "$OUTPUT"
        echo "|-------|------|" >> "$OUTPUT"
        # extract field name and type pairs
        # fields are YAML list items with name: and type: on consecutive-ish lines
        awk '
        /^fields:/ { in_fields=1; next }
        /^[a-z]/ && in_fields { exit }
        in_fields && /^[[:space:]]*- name:/ {
            gsub(/^[[:space:]]*- name:[[:space:]]*/, "")
            fname=$0
        }
        in_fields && /^[[:space:]]*type:/ {
            gsub(/^[[:space:]]*type:[[:space:]]*/, "")
            printf "| %s | %s |\n", fname, $0
        }
        ' "$filepath" >> "$OUTPUT"
        echo "" >> "$OUTPUT"
    fi

    ENTITY_COUNT=$((ENTITY_COUNT + 1))
    FIELD_COUNT=$((FIELD_COUNT + num_fields))
done

# append summary at end
echo "---" >> "$OUTPUT"
echo "" >> "$OUTPUT"
echo "*Total entities: count varies by schema version. Generated $(date -u +"%Y-%m-%d").*" >> "$OUTPUT"

echo "entity catalog written to ${OUTPUT}"


