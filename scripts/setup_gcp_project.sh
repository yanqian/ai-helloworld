#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/setup_gcp_project.sh <project-id> <billing-account-id> [region] [artifact-repo] [service-name]

Required arguments:
  project-id            Globally unique new GCP project ID (e.g., ai-helloworld-1234)
  billing-account-id    Billing account to attach (see `gcloud beta billing accounts list`)

Optional positional arguments (or override via environment variables):
  region                Primary region for Artifact Registry & Cloud Run (default: ${REGION:-asia-southeast1})
  artifact-repo         Artifact Registry repo name for container images (default: ${ARTIFACT_REPO:-backend-repo})
  service-name          Cloud Run service name (default: ${SERVICE_NAME:-summarizer})

Configurable environment variables:
  PROJECT_NAME                Display name for the project (default: "AI Helloworld")
  FOLDER_ID                   Org/folder ID to place the project under.
  SA_NAME                     CI service account name (default: ci-deployer)
  SA_DISPLAY_NAME             Display label for the service account.
  SA_KEY_FILE                 Where to write the service account key (default: ./<project>-ci-deployer-key.json)
  SA_ROLES                    Space separated list of IAM roles to grant (override with caution).
  CLOUD_BUILD_BUCKET          Bucket URI for Cloud Build staging (default: gs://<project>_cloudbuild)
  SKIP_SA_KEY                 Set to "true" to avoid generating a service account key file.

Prerequisites:
  - You must be authenticated via `gcloud auth login` or an administrator service account.
  - The caller needs permissions to create projects, link billing, manage IAM, Artifact Registry, and Cloud Run.
EOF
}

log() {
  printf '\n[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

require_arg() {
  if [[ -z "${1:-}" ]]; then
    usage
    exit 1
  fi
}

ensure_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required on PATH."
    exit 1
  fi
}

ensure_command "gcloud"

PROJECT_ID=${1:-}
require_arg "$PROJECT_ID"

BILLING_ACCOUNT=${2:-}
require_arg "$BILLING_ACCOUNT"

REGION=${3:-${REGION:-asia-southeast1}}
ARTIFACT_REPO=${4:-${ARTIFACT_REPO:-backend-repo}}
SERVICE_NAME=${5:-${SERVICE_NAME:-summarizer}}
PROJECT_NAME=${PROJECT_NAME:-"AI Helloworld"}
FOLDER_ID=${FOLDER_ID:-}
SA_NAME=${SA_NAME:-ci-deployer}
SA_DISPLAY_NAME=${SA_DISPLAY_NAME:-"CI Deployer"}
DEFAULT_KEY_PATH="./${PROJECT_ID}-${SA_NAME}-key.json"
SA_KEY_FILE=${SA_KEY_FILE:-$DEFAULT_KEY_PATH}
CLOUD_BUILD_BUCKET=${CLOUD_BUILD_BUCKET:-"gs://${PROJECT_ID}_cloudbuild"}
SKIP_SA_KEY=${SKIP_SA_KEY:-false}

IFS=' ' read -r -a SA_ROLES <<<"${SA_ROLES:-roles/run.admin roles/iam.serviceAccountUser roles/artifactregistry.writer roles/cloudbuild.builds.editor roles/storage.admin roles/logging.viewer roles/viewer}"

SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

ensure_project() {
  if gcloud projects describe "$PROJECT_ID" >/dev/null 2>&1; then
    log "Project $PROJECT_ID already exists; skipping creation."
    return
  fi

  log "Creating project $PROJECT_ID ..."
  CREATE_ARGS=(--name="$PROJECT_NAME")
  if [[ -n "$FOLDER_ID" ]]; then
    CREATE_ARGS+=("--folder=$FOLDER_ID")
  fi
  gcloud projects create "$PROJECT_ID" "${CREATE_ARGS[@]}"
}

link_billing() {
  local enabled
  enabled=$(gcloud beta billing projects describe "$PROJECT_ID" --format="value(billingEnabled)" 2>/dev/null || echo "False")
  if [[ "$enabled" == "True" ]]; then
    log "Billing already linked."
    return
  fi

  log "Linking billing account $BILLING_ACCOUNT ..."
  gcloud beta billing projects link "$PROJECT_ID" --billing-account="$BILLING_ACCOUNT"
}

enable_apis() {
  local apis=(
    artifactregistry.googleapis.com
    cloudbuild.googleapis.com
    run.googleapis.com
    secretmanager.googleapis.com
    iam.googleapis.com
    cloudresourcemanager.googleapis.com
    serviceusage.googleapis.com
    compute.googleapis.com
    logging.googleapis.com
    storage.googleapis.com
  )
  log "Enabling required APIs..."
  gcloud services enable "${apis[@]}" --project="$PROJECT_ID"
}

ensure_artifact_repo() {
  if gcloud artifacts repositories describe "$ARTIFACT_REPO" --location="$REGION" --project="$PROJECT_ID" >/dev/null 2>&1; then
    log "Artifact Registry $ARTIFACT_REPO already exists."
    return
  fi

  log "Creating Artifact Registry repo $ARTIFACT_REPO ..."
  gcloud artifacts repositories create "$ARTIFACT_REPO" \
    --repository-format=docker \
    --location="$REGION" \
    --description="Containers for $SERVICE_NAME" \
    --project="$PROJECT_ID"
}

ensure_bucket() {
  if gcloud storage buckets describe "$CLOUD_BUILD_BUCKET" --project="$PROJECT_ID" >/dev/null 2>&1; then
    log "Cloud Build bucket $CLOUD_BUILD_BUCKET already exists."
    return
  fi

  log "Creating Cloud Build staging bucket $CLOUD_BUILD_BUCKET ..."
  gcloud storage buckets create "$CLOUD_BUILD_BUCKET" \
    --project="$PROJECT_ID" \
    --location="$REGION" \
    --uniform-bucket-level-access
}

ensure_service_account() {
  if gcloud iam service-accounts describe "$SA_EMAIL" --project="$PROJECT_ID" >/dev/null 2>&1; then
    log "Service account $SA_EMAIL already exists."
    return
  fi

  log "Creating service account $SA_EMAIL ..."
  gcloud iam service-accounts create "$SA_NAME" \
    --display-name="$SA_DISPLAY_NAME" \
    --project="$PROJECT_ID"
}

bind_sa_roles() {
  log "Granting IAM roles to $SA_EMAIL ..."
  for role in "${SA_ROLES[@]}"; do
    if gcloud projects get-iam-policy "$PROJECT_ID" \
      --flatten="bindings[].members" \
      --filter="bindings.members:serviceAccount:${SA_EMAIL} AND bindings.role:${role}" \
      --format="value(bindings.role)" | grep -q "$role"; then
      log "  Role $role already bound."
      continue
    fi
    gcloud projects add-iam-policy-binding "$PROJECT_ID" \
      --member="serviceAccount:${SA_EMAIL}" \
      --role="$role" \
      --quiet
  done
}

maybe_create_sa_key() {
  if [[ "$SKIP_SA_KEY" == "true" ]]; then
    log "SKIP_SA_KEY=true, not creating a service account key."
    return
  fi
  if [[ -f "$SA_KEY_FILE" ]]; then
    log "Service account key $SA_KEY_FILE already exists; remove it if you need a fresh key."
    return
  fi
  log "Generating service account key at $SA_KEY_FILE ..."
  gcloud iam service-accounts keys create "$SA_KEY_FILE" \
    --iam-account="$SA_EMAIL" \
    --project="$PROJECT_ID"
  log "Remember to store $SA_KEY_FILE securely (e.g., GitHub Actions secret) and then delete the local copy if not needed."
}

print_summary() {
  cat <<EOF

----------------------------------------------------------------------
Project bootstrap complete.

Project ID:        $PROJECT_ID
Billing account:   $BILLING_ACCOUNT
Region:            $REGION
Artifact repo:     $ARTIFACT_REPO
Cloud Run service: $SERVICE_NAME
Service account:   $SA_EMAIL
Key file:          $([[ "$SKIP_SA_KEY" == "true" ]] && echo "skipped" || echo "$SA_KEY_FILE")
Cloud Build bucket:$CLOUD_BUILD_BUCKET
----------------------------------------------------------------------

Next steps:
  1. Move/rename $SA_KEY_FILE -> key.json (matches repo expectation) and add it to your secret store (e.g., GitHub Actions secret GCP_SA_KEY).
  2. Update .github/workflows/ci.yml vars (project, region, repo) via GitHub → Settings → Variables.
  3. Update Makefile defaults if the project ID or region changed.
  4. Run: gcloud config set project $PROJECT_ID && make deploy (optional verification).
EOF
}

main() {
  ensure_project
  link_billing
  gcloud config set project "$PROJECT_ID" >/dev/null
  enable_apis
  ensure_artifact_repo
  ensure_bucket
  ensure_service_account
  bind_sa_roles
  maybe_create_sa_key
  print_summary
}

main "$@"
