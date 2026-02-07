#!/usr/bin/env sh
set -eu

log() {
  level="$1"; shift
  msg="$1"; shift
  if [ $# -gt 0 ]; then
    printf '%s\t%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$level" "$msg" "$1"
  else
    printf '%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$level" "$msg"
  fi
}

flags_dir="${1:?Usage: generate-acl-data.sh <flags-dir> <output-path>}"
output_path="${2:?Usage: generate-acl-data.sh <flags-dir> <output-path>}"

if [ ! -d "$flags_dir" ]; then
  log ERROR "flags directory not found" "{\"path\": \"$flags_dir\"}" >&2
  exit 1
fi

result='{}'
seen=""

for access_file in $(find "$flags_dir" -mindepth 3 -maxdepth 3 -name "access.yml" -type f | sort); do
  namespace=$(basename "$(dirname "$access_file")")

  # Skip already-processed namespaces (same namespace appears across environments)
  case " $seen " in
    *" $namespace "*) continue ;;
  esac
  seen="$seen $namespace"

  # Extract writers from YAML and build a JSON array
  writers=$(grep '^ *- ' "$access_file" | sed 's/^ *- *//' | jq -R '.' | jq -s '.')

  result=$(echo "$result" | jq --arg ns "$namespace" --argjson w "$writers" '. + {($ns): $w}')
done

# Write final structure (atomic write via temp file)
tmp_output="${output_path}.tmp"
jq -n --argjson nta "$result" '{"namespace_team_access": $nta}' > "$tmp_output"
mv "$tmp_output" "$output_path"

log INFO "generated ACL data" "{\"path\": \"$output_path\"}"
