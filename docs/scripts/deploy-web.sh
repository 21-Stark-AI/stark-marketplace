#!/usr/bin/env bash
set -euo pipefail

# deploy-web.sh — manual, local deploy of the web registry origin.
#
# The `.github/workflows/web-deploy.yml` workflow AUTO-deploys on every push to
# main that touches web/server/index.json/bundles. Use this script for a
# MANUAL/local deploy (local gcloud ADC + docker, not WIF) — an out-of-band
# publish, or when you can't wait for CI. The native CC marketplace reads the
# public repo directly and does NOT need this site.
#
# Auth: uses your LOCAL gcloud ADC + docker (not WIF). You must be
# authenticated to the ev-infra-group project with rights to push to GAR and
# `gcloud run deploy` the stark-marketplace service (run.developer). The
# allUsers run.invoker binding + LB/DNS stay Terraform-owned in ev-infra-group.
#
# Usage:
#   docs/scripts/deploy-web.sh            # build + push + deploy
#   DRY_RUN=1 docs/scripts/deploy-web.sh  # print what would run, do nothing

GCP_PROJECT_ID="ev-infra-group"
GCP_REGION="us-central1"
GAR_REPO="stark-marketplace"
CR_SERVICE="stark-marketplace"
RUNTIME_SA="stark-marketplace-run@ev-infra-group.iam.gserviceaccount.com"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

SHA="$(git rev-parse HEAD)"
IMAGE="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/${GAR_REPO}/${CR_SERVICE}"

run() {
  echo "+ $*"
  if [ "${DRY_RUN:-0}" != "1" ]; then "$@"; fi
}

echo "==> Building SPA (web/)"
run bash -c 'cd web && npm ci && npm run lint && npm run typecheck && npm test && npm run build'

echo "==> Staging index.json + bundles/ into web/dist (atomic content-hashed unit)"
run bash -c 'mkdir -p web/dist/bundles && cp index.json web/dist/index.json && (cp bundles/*.json web/dist/bundles/ 2>/dev/null || true)'

echo "==> Building + pushing image ${IMAGE}:${SHA} (linux/amd64 for Cloud Run)"
run gcloud auth configure-docker "${GCP_REGION}-docker.pkg.dev" --quiet
# Cloud Run runs linux/amd64. On an arm64 Mac a plain `docker build` produces an
# arm64 image that fails to start with "exec format error", so cross-build the
# amd64 image with buildx (build + push in one step). Requires the buildx plugin
# (`brew install docker-buildx` + symlink into ~/.docker/cli-plugins).
run docker buildx build --platform linux/amd64 \
  -t "${IMAGE}:${SHA}" -t "${IMAGE}:latest" --push .

echo "==> Deploying Cloud Run ${CR_SERVICE} (behind platform LB)"
run gcloud run deploy "${CR_SERVICE}" \
  --project "${GCP_PROJECT_ID}" \
  --region "${GCP_REGION}" \
  --image "${IMAGE}:${SHA}" \
  --service-account "${RUNTIME_SA}" \
  --ingress internal-and-cloud-load-balancing \
  --port 8080 \
  --cpu 1 --memory 256Mi \
  --min-instances 0 --max-instances 4 --concurrency 80 \
  --quiet

echo "==> Done. Verify: curl -sI https://marketplace.21stark.com/ | head -1"
echo "    Prune old GAR images afterwards to keep storage near-zero:"
echo "    gcloud artifacts docker images list ${IMAGE} --include-tags"
