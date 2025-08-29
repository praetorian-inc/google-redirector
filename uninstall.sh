#!/bin/bash

set -e

# Check for redirector name argument
if [ -z "$1" ]; then
    echo "❌ Error: Redirector name is required"
    echo "Usage: ./uninstall.sh <redirector-name>"
    echo "Example: ./uninstall.sh panw"
    exit 1
fi

REDIRECTOR_NAME="$1"
echo "🗑️  Uninstalling Google Redirector '$REDIRECTOR_NAME'"

PROJECT_ID=$(gcloud config get-value project)
if [ -z "$PROJECT_ID" ]; then
    echo "❌ Error: No GCP project configured. Run 'gcloud config set project YOUR-PROJECT-ID' first"
    exit 1
fi

echo "📋 Using GCP project: $PROJECT_ID"
SERVICE_NAME="redirector-$REDIRECTOR_NAME"
REGION=${GOOGLE_CLOUD_REGION:-"us-central1"}
REPO_NAME="google-redirector"
IMAGE_NAME="$REGION-docker.pkg.dev/$PROJECT_ID/$REPO_NAME/$SERVICE_NAME"

echo "☁️  Deleting Cloud Run service '$SERVICE_NAME'..."
gcloud run services delete $SERVICE_NAME \
    --region $REGION \
    --quiet || echo "⚠️  Service '$SERVICE_NAME' not found or already deleted"

echo "🗑️  Deleting container image '$IMAGE_NAME'..."
gcloud artifacts docker images delete $IMAGE_NAME \
    --quiet || echo "⚠️  Image '$IMAGE_NAME' not found or already deleted"

echo "✅ Uninstall complete for redirector '$REDIRECTOR_NAME'"
echo "ℹ️  Note: Artifact Registry repository '$REPO_NAME' is shared and not deleted"
echo "🌐 To delete the repository (affects ALL redirectors):"
echo "   gcloud artifacts repositories delete $REPO_NAME --location=$REGION"