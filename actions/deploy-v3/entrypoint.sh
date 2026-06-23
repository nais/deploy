#!/usr/bin/env bash
set -euo pipefail

# === Nais Deploy v3 ===
# Drop-in replacement for nais/deploy (v2) that uses:
# - deploy-cli for handlebars templating of resource files
# - nais CLI (nais alpha apply) for the actual deployment
#
# This allows existing workflows using handlebars templates ({{var}})
# to migrate seamlessly while using the new nais platform API.
#
# Key difference from v2: The Image resource (kind: Image) is NOT generated.
# Instead, workload-image is handled via `nais alpha apply --set spec.image=`.
# When workload-image is empty (e.g. what-changed only-inputs scenario),
# nais alpha apply will preserve the currently running image in the cluster.

# --- Configuration from action inputs ---
# Supports both action inputs (INPUT_*) and legacy env vars for backward compatibility
RESOURCE="${INPUT_RESOURCE:-${RESOURCE:-}}"
CLUSTER="${INPUT_CLUSTER:-${CLUSTER:-}}"
TEAM="${INPUT_TEAM:-${TEAM:-}}"
VARS="${INPUT_VARS:-${VARS:-}}"
VAR="${INPUT_VAR:-${VAR:-}}"
IMAGE="${INPUT_IMAGE:-${IMAGE:-}}"
WORKLOAD_IMAGE="${INPUT_WORKLOAD_IMAGE:-${WORKLOAD_IMAGE:-}}"
WAIT="${INPUT_WAIT:-${WAIT:-true}}"
TIMEOUT="${INPUT_TIMEOUT:-${TIMEOUT:-10m}}"
DRY_RUN="${INPUT_DRY_RUN:-${DRY_RUN:-false}}"

# --- Validation ---
if [ -z "$RESOURCE" ]; then
  echo "::error::Input 'resource' is required"
  exit 1
fi

if [ -z "$CLUSTER" ]; then
  echo "::error::Input 'cluster' is required"
  exit 1
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
# Returns 0 (true) if templating is needed, 1 (false) otherwise.
# Templating is needed when:
# - A vars file is specified (--vars)
# - Inline variables are specified (--var)
# - Any resource file contains {{...}} syntax
needs_templating() {
  [ -n "$VARS" ] && return 0
  [ -n "$VAR" ] && return 0

  # Check if any resource file contains handlebars syntax
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
# Creates a temporary vars file with all template variables merged.
# If IMAGE is set, it is injected as the "image" template variable (v2 compat).
prepare_vars_file() {
  local vars_file
  vars_file=$(mktemp /tmp/deploy-vars-XXXXXX.yaml)

  if [ -n "$VARS" ]; then
    cp "$VARS" "$vars_file"
  else
    echo "---" > "$vars_file"
  fi

  # Inject IMAGE as a template variable (same behavior as v2 entrypoint.sh)
  if [ -n "$IMAGE" ]; then
    if ! command -v yq &> /dev/null; then
      echo "::group::Install yq" >&2
      curl -sSL -o /tmp/yq "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64"
      chmod +x /tmp/yq
      export PATH="/tmp:$PATH"
      echo "::endgroup::" >&2
    fi
    yq eval ".image = \"${IMAGE}\"" -i "$vars_file"
  fi

  echo "$vars_file"
}

# --- Template a single resource file using deploy-cli ---
# deploy-cli with --dry-run --print-payload outputs a protojson DeploymentRequest.
# The kubernetes resources are at .kubernetes.resources[] as JSON objects
# (google.protobuf.Struct). We extract them and convert back to multi-document YAML.
template_resource() {
  local deploy_cli="$1"
  local resource_file="$2"
  local vars_file="$3"
  local output_file="$4"

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

  # Run deploy-cli: stdout = protojson payload, stderr = log messages
  local payload
  if ! payload=$("$deploy_cli" "${cli_args[@]}" 2>/dev/null); then
    # Retry showing stderr for diagnostics
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

  # Extract resources from protojson and convert to multi-document YAML.
  # Skip any Image resources (kind: Image, apiVersion: nais.io/v1) that deploy-cli
  # may have generated from WORKLOAD_IMAGE - we handle image via --set instead.
  local resource_count
  resource_count=$(echo "$payload" | jq '.kubernetes.resources | length')

  if [ "$resource_count" -eq 0 ]; then
    echo "::error::No resources found in deploy-cli output for: ${resource_file}"
    return 1
  fi

  : > "$output_file"
  for ((i=0; i<resource_count; i++)); do
    local kind
    kind=$(echo "$payload" | jq -r ".kubernetes.resources[$i].kind // empty")
    local api_version
    api_version=$(echo "$payload" | jq -r ".kubernetes.resources[$i].apiVersion // empty")

    # Skip Image resources generated by deploy-cli (we use --set spec.image instead)
    if [ "$kind" = "Image" ] && [ "$api_version" = "nais.io/v1" ]; then
      echo "  Skipping generated Image resource (handled via --set spec.image)"
      continue
    fi

    echo "---" >> "$output_file"
    echo "$payload" | jq ".kubernetes.resources[$i]" | yq eval -P - >> "$output_file"
  done

  # Ensure we wrote at least one resource
  if [ ! -s "$output_file" ]; then
    echo "::error::No applicable resources after filtering for: ${resource_file}"
    return 1
  fi
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
TEMPLATED=false

if needs_templating; then
  # --- Templating path: use deploy-cli for handlebars rendering ---
  TEMPLATED=true
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

    rendered_file="${RENDERED_DIR}/${file_index}-$(basename "$resource_file")"
    file_index=$((file_index + 1))
    echo "Rendering: ${resource_file}"

    template_resource "$DEPLOY_CLI" "$resource_file" "$VARS_FILE" "$rendered_file"

    RENDERED_FILES+=("$rendered_file")
    echo "  -> ${rendered_file}"
  done

  echo "::endgroup::"

  # Cleanup vars file
  rm -f "$VARS_FILE"
else
  # --- No templating needed: use resource files directly ---
  echo "No template variables detected; using resource files as-is"

  for resource_file in "${RESOURCE_FILES[@]}"; do
    resource_file=$(echo "$resource_file" | xargs)

    if [ ! -f "$resource_file" ]; then
      echo "::error::Resource file not found: ${resource_file}"
      exit 1
    fi

    RENDERED_FILES+=("$resource_file")
  done
fi

# --- Dry run: just print rendered resources ---
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

  # Set image via --set when we have an effective image to deploy.
  # - With templating: deploy-cli has substituted {{image}} in spec.image,
  #   but we still use --set to ensure consistency.
  # - Without templating: --set injects the image directly into spec.image.
  # - When EFFECTIVE_IMAGE is empty (e.g. what-changed only-inputs, no new build):
  #   nais alpha apply will preserve the image currently running in the cluster.
  if [ -n "$EFFECTIVE_IMAGE" ]; then
    APPLY_ARGS+=("--set" "spec.image=${EFFECTIVE_IMAGE}")
  fi

  echo "Running: nais ${APPLY_ARGS[*]}"
  nais "${APPLY_ARGS[@]}"
done

echo "::endgroup::"

# --- Cleanup ---
rm -rf "$RENDERED_DIR"
