#!/bin/bash

set -e

# Check for redirector name argument
if [ -z "$1" ]; then
    echo "❌ Error: Redirector name is required"
    echo "Usage: ./deploy.sh <redirector-name>"
    echo "Example: ./deploy.sh acmecorp"
    exit 1
fi

REDIRECTOR_NAME="$1"
echo "🚀 Deploying Google Redirector '$REDIRECTOR_NAME' to Google Cloud Run"

if [ -z "$BACKEND_URL" ]; then
    echo "❌ Error: BACKEND_URL environment variable is not set"
    echo "Example: export BACKEND_URL=https://your-backend.com"
    exit 1
fi

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

echo "🏗️  Setting up Artifact Registry repository..."
gcloud artifacts repositories create $REPO_NAME \
    --repository-format=docker \
    --location=$REGION \
    --description="Google Cloud Redirector images" \
    --quiet 2>/dev/null || echo "Repository already exists, continuing..."

echo "📦 Building Docker image..."
docker build -t $IMAGE_NAME .

echo "🔐 Configuring Docker for Artifact Registry..."
gcloud auth configure-docker $REGION-docker.pkg.dev

echo "📤 Pushing image to Artifact Registry..."
docker push $IMAGE_NAME

echo "☁️  Deploying to Cloud Run..."
gcloud run deploy $SERVICE_NAME \
    --image $IMAGE_NAME \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --set-env-vars "BACKEND_URL=$BACKEND_URL" \
    --memory 512Mi \
    --cpu 1 \
    --concurrency 100 \
    --timeout 300 \
    --max-instances 10 \
    --min-instances 1

echo "✅ Deployment complete!"
echo "🌐 Service URL:"
gcloud run services describe $SERVICE_NAME --platform managed --region $REGION --format 'value(status.url)'
