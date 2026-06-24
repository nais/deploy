#!/usr/bin/env bash
set -euo pipefail

# === Nais Deploy v3 ===
# Drop-in replacement for nais/deploy/actions/deploy@v2.
# Uses the same environment variables as v2:
#   CLUSTER, RESOURCE, IMAGE, WORKLOAD_IMAGE, VARS, VAR, TEAM, WAIT, TIMEOUT, DRY_RUN
#
# Internally:
# - deploy-cli handles handlebars templating of resource files
# - nais CLI (nais alpha apply) handles the actual deployment
#
# Key difference from v2: No Image resource (kind: Image) is generated.
# Instead, workload-image is handled via `nais alpha apply --set spec.image=`.
# When WORKLOAD_IMAGE is empty (e.g. what-changed only-inputs scenario),
# nais alpha apply preserves the currently running image in the cluster.

# --- Configuration from environment variables (same as v2) ---
RESOURCE="${RESOURCE:-}"
CLUSTER="${CLUSTER:-}"
TEAM="${TEAM:-}"
VARS="${VARS:-}"
VAR="${VAR:-}"
IMAGE="${IMAGE:-}"
WORKLOAD_IMAGE="${WORKLOAD_IMAGE:-}"
WAIT="${WAIT:-true}"
TIMEOUT="${TIMEOUT:-10m}"
DRY_RUN="${DRY_RUN:-false}"

# --- Ensure yq is available ---
ensure_yq() {
  if command -v yq &> /dev/null; then
    return
  fi
  echo "::group::Install yq" >&2
  curl -sSL -o /tmp/yq "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64"
  chmod +x /tmp/yq
  export PATH="/tmp:$PATH"
  echo "::endgroup::" >&2
}

# --- Validation ---
if [ -z "$RESOURCE" ]; then
  echo "::error::RESOURCE is required"
  exit 1
fi

if [ -z "$CLUSTER" ]; then
  echo "::error::CLUSTER is required"
  exit 1
fi

# --- Detect TEAM from resource files if not set via env ---
if [ -z "$TEAM" ]; then
  ensure_yq
  IFS=',' read -ra _files <<< "$RESOURCE"
  for _f in "${_files[@]}"; do
    _f=$(echo "$_f" | xargs)
    if [ -f "$_f" ]; then
      _detected=$(yq eval-all '[.metadata.labels.team // ""] | map(select(. != "")) | .[0] // ""' "$_f" 2>/dev/null || true)
      if [ -n "$_detected" ] && [ "$_detected" != "null" ]; then
        TEAM="$_detected"
        echo "Detected team '${TEAM}' from ${_f}"
        break
      fi
      _detected=$(yq eval-all '[.metadata.namespace // ""] | map(select(. != "")) | .[0] // ""' "$_f" 2>/dev/null || true)
      if [ -n "$_detected" ] && [ "$_detected" != "null" ]; then
        TEAM="$_detected"
        echo "Detected team '${TEAM}' from namespace in ${_f}"
        break
      fi
    fi
  done

  if [ -z "$TEAM" ]; then
    echo "::error::TEAM is required. Set it via env var or metadata.labels.team in your resource file."
    exit 1
  fi
fi

# --- Resolve the effective image ---
# Priority: IMAGE (template variable) > WORKLOAD_IMAGE (--set spec.image)
# If neither is set, nais alpha apply will use the image currently running in the cluster.
EFFECTIVE_IMAGE=""
if [ -n "$IMAGE" ]; then
  EFFECTIVE_IMAGE="$IMAGE"
elif [ -n "$WORKLOAD_IMAGE" ]; then
  EFFECTIVE_IMAGE="$WORKLOAD_IMAGE"
fi

# --- Helper: check if resource files use handlebars templates ---
needs_templating() {
  [ -n "$VARS" ] && return 0
  [ -n "$VAR" ] && return 0

  IFS=',' read -ra files <<< "$RESOURCE"
  for f in "${files[@]}"; do
    f=$(echo "$f" | xargs)
    if grep -qE '\{\{.*\}\}' "$f" 2>/dev/null; then
      return 0
    fi
  done

  return 1
}

# --- Download deploy-cli for templating ---
download_deploy_cli() {
  local deploy_cli="/tmp/deploy-cli"

  if [ -f "$deploy_cli" ]; then
    echo "$deploy_cli"
    return
  fi

  echo "::group::Download deploy-cli for templating" >&2

  local url="https://github.com/nais/deploy/releases/download/v2/deploy-linux"
  echo "Downloading deploy-cli from ${url}..." >&2
  curl -sSL -o "$deploy_cli" "$url"
  chmod +x "$deploy_cli"

  echo "deploy-cli downloaded successfully" >&2
  echo "::endgroup::" >&2

  echo "$deploy_cli"
}

# --- Prepare template variables file ---
prepare_vars_file() {
  local vars_file
  vars_file=$(mktemp /tmp/deploy-vars-XXXXXX.yaml)

  if [ -n "$VARS" ]; then
    cp "$VARS" "$vars_file"
  else
    echo "---" > "$vars_file"
  fi

  if [ -n "$IMAGE" ]; then
    ensure_yq
    yq eval ".image = \"${IMAGE}\"" -i "$vars_file"
  fi

  echo "$vars_file"
}

# --- Template a resource file using deploy-cli, output one file per resource ---
# Appends file paths to the RENDERED_FILES array.
template_resources() {
  local deploy_cli="$1"
  local resource_file="$2"
  local vars_file="$3"
  local output_dir="$4"
  local file_prefix="$5"

  local cli_args=(
    --resource "$resource_file"
    --cluster "$CLUSTER"
    --team "${TEAM:-templating}"
    --dry-run
    --print-payload
  )

  if [ -n "$vars_file" ]; then
    cli_args+=(--vars "$vars_file")
  fi

  if [ -n "$VAR" ]; then
    cli_args+=(--var "$VAR")
  fi

  local payload
  if ! payload=$("$deploy_cli" "${cli_args[@]}" 2>/dev/null); then
    echo "::warning::deploy-cli failed silently, retrying with verbose output..."
    if ! payload=$("$deploy_cli" "${cli_args[@]}"); then
      echo "::error::Failed to template resource: ${resource_file}"
      return 1
    fi
  fi

  if [ -z "$payload" ]; then
    echo "::error::deploy-cli produced empty output for: ${resource_file}"
    return 1
  fi

  local resource_count
  resource_count=$(echo "$payload" | jq '.kubernetes.resources | length')

  if [ "$resource_count" -eq 0 ]; then
    echo "::error::No resources found in deploy-cli output for: ${resource_file}"
    return 1
  fi

  local wrote=0
  for ((i=0; i<resource_count; i++)); do
    local kind api_version
    kind=$(echo "$payload" | jq -r ".kubernetes.resources[$i].kind // empty")
    api_version=$(echo "$payload" | jq -r ".kubernetes.resources[$i].apiVersion // empty")

    # Skip Image resources generated by deploy-cli (we use --set spec.image instead)
    if [ "$kind" = "Image" ] && [ "$api_version" = "nais.io/v1" ]; then
      echo "  Skipping generated Image resource (handled via --set spec.image)"
      continue
    fi

    local out_file="${output_dir}/${file_prefix}-${i}.yaml"
    echo "$payload" | jq ".kubernetes.resources[$i]" | yq eval -P - > "$out_file"
    RENDERED_FILES+=("$out_file")
    echo "  -> ${out_file} (${kind})"
    wrote=$((wrote + 1))
  done

  if [ "$wrote" -eq 0 ]; then
    echo "::error::No applicable resources after filtering for: ${resource_file}"
    return 1
  fi
}

# --- Split a multi-document YAML file into one file per document ---
# Appends file paths to the RENDERED_FILES array.
split_yaml() {
  local input_file="$1"
  local output_dir="$2"
  local file_prefix="$3"

  ensure_yq
  local doc_count
  doc_count=$(yq eval-all '[.] | length' "$input_file")

  if [ "$doc_count" -le 1 ]; then
    RENDERED_FILES+=("$input_file")
    return
  fi

  for ((i=0; i<doc_count; i++)); do
    local out_file="${output_dir}/${file_prefix}-${i}.yaml"
    yq eval "select(documentIndex == $i)" "$input_file" > "$out_file"
    RENDERED_FILES+=("$out_file")
    local kind
    kind=$(yq eval '.kind // "unknown"' "$out_file" 2>/dev/null || echo "unknown")
    echo "  -> ${out_file} (${kind})"
  done
}

# --- Main ---
echo "::group::Nais Deploy v3"
echo "Cluster: ${CLUSTER}"
echo "Resources: ${RESOURCE}"
[ -n "$TEAM" ] && echo "Team: ${TEAM}"
[ -n "$EFFECTIVE_IMAGE" ] && echo "Image: ${EFFECTIVE_IMAGE}"
[ -z "$EFFECTIVE_IMAGE" ] && echo "Image: (will use currently running image)"
echo "Wait: ${WAIT}"
echo "Timeout: ${TIMEOUT}"
echo "::endgroup::"

# Parse resource file list
IFS=',' read -ra RESOURCE_FILES <<< "$RESOURCE"
RENDERED_DIR=$(mktemp -d /tmp/deploy-rendered-XXXXXX)
RENDERED_FILES=()

if needs_templating; then
  DEPLOY_CLI=$(download_deploy_cli)
  VARS_FILE=$(prepare_vars_file)

  echo "::group::Template resources"

  file_index=0
  for resource_file in "${RESOURCE_FILES[@]}"; do
    resource_file=$(echo "$resource_file" | xargs)

    if [ ! -f "$resource_file" ]; then
      echo "::error::Resource file not found: ${resource_file}"
      exit 1
    fi

    echo "Rendering: ${resource_file}"
    template_resources "$DEPLOY_CLI" "$resource_file" "$VARS_FILE" "$RENDERED_DIR" "$file_index"
    file_index=$((file_index + 1))
  done

  echo "::endgroup::"

  rm -f "$VARS_FILE"
else
  echo "No template variables detected; using resource files as-is"

  ensure_yq
  file_index=0
  for resource_file in "${RESOURCE_FILES[@]}"; do
    resource_file=$(echo "$resource_file" | xargs)

    if [ ! -f "$resource_file" ]; then
      echo "::error::Resource file not found: ${resource_file}"
      exit 1
    fi

    split_yaml "$resource_file" "$RENDERED_DIR" "$file_index"
    file_index=$((file_index + 1))
  done
fi

# --- Dry run ---
if [ "$DRY_RUN" = "true" ]; then
  echo "::group::Dry run - rendered resources"
  for f in "${RENDERED_FILES[@]}"; do
    echo "=== $(basename "$f") ==="
    cat "$f"
    echo ""
  done
  echo "::endgroup::"
  echo "Dry run complete. No deployment performed."
  exit 0
fi

# --- Deploy using nais CLI ---
echo "::group::Deploy with nais CLI"

for rendered_file in "${RENDERED_FILES[@]}"; do
  echo "Deploying: $(basename "$rendered_file")"

  APPLY_ARGS=(
    "alpha" "apply" "$rendered_file"
    "--environment" "$CLUSTER"
    "--allow-ignored-fields"
  )

  if [ -n "$TEAM" ]; then
    APPLY_ARGS+=("--team" "$TEAM")
  fi

  if [ "$WAIT" = "true" ]; then
    APPLY_ARGS+=("--wait" "--timeout" "$TIMEOUT")
  fi

  # Only set spec.image on workload resources (Application, Naisjob), not on ConfigMaps etc.
  if [ -n "$EFFECTIVE_IMAGE" ]; then
    local_kind=$(yq eval '.kind // ""' "$rendered_file" 2>/dev/null || true)
    if [ "$local_kind" = "Application" ] || [ "$local_kind" = "Naisjob" ]; then
      APPLY_ARGS+=("--set" "spec.image=${EFFECTIVE_IMAGE}")
    fi
  fi

  echo "Running: nais ${APPLY_ARGS[*]}"
  nais "${APPLY_ARGS[@]}"
done

echo "::endgroup::"

# --- Cleanup ---
rm -rf "$RENDERED_DIR"
