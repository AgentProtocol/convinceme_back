#!/bin/bash

# =================================================================
# ConvinceMe Backend Deployment Script
# =================================================================
# This script deploys the latest backend changes to production
# while preserving the database and logs on the remote server.
# =================================================================

set -e  # Exit on any error

# Configuration
SERVER="root@46.101.198.5"
REMOTE_PATH="/root/convinceme_back"
SERVICE_NAME="convinceme_server"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Pre-deployment checks
echo "================================================================="
echo "🚀 ConvinceMe Backend Deployment Starting..."
echo "================================================================="

log_info "Checking prerequisites..."

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "internal" ]; then
    log_error "Please run this script from the convinceme_back directory"
    exit 1
fi

# Test SSH connection
log_info "Testing SSH connection to $SERVER..."
if ! ssh -o ConnectTimeout=10 -o BatchMode=yes $SERVER "echo 'SSH connection successful'" 2>/dev/null; then
    log_error "Cannot connect to $SERVER. Please check your SSH connection."
    exit 1
fi

log_success "SSH connection verified"

# Sync files to remote server
log_info "Syncing source code to production server..."
log_warning "Database and logs will be preserved on remote server"

rsync -avz --progress \
    --exclude='data/' \
    --exclude='logs/' \
    --exclude='.git/' \
    --exclude='.idea/' \
    --exclude='bin/' \
    --exclude='convinceme_server' \
    --exclude='convinceme' \
    --exclude='main' \
    --exclude='migrate' \
    --exclude='*.log' \
    --exclude='.env' \
    --exclude='*.db' \
    --exclude='*.sqlite' \
    --exclude='deploy.sh' \
    ./ $SERVER:$REMOTE_PATH/

log_success "Files synced successfully"

# Build and deploy on remote server
log_info "Building and deploying on remote server..."

ssh $SERVER << 'REMOTE_COMMANDS'
set -e

# Navigate to app directory
cd /root/convinceme_back

echo "🔨 Building application..."
# Clean old binaries
rm -f convinceme_server convinceme main migrate

# Build the main server
go build -o convinceme_server ./cmd/main.go
if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

# Make sure binary is executable
chmod +x convinceme_server

echo "🔄 Stopping existing service..."
# Stop the existing service gracefully
pkill convinceme_server 2>/dev/null || true

# Wait a bit for graceful shutdown
sleep 3

# Force kill if still running
pkill -9 convinceme_server 2>/dev/null || true

echo "📋 Checking system status..."
# Create logs directory if it doesn't exist
mkdir -p logs

# Check if database exists
if [ -d "data" ]; then
    echo "✅ Database directory preserved"
else
    echo "⚠️  No database directory found - you may need to run migrations"
fi

echo "🚀 Starting new service..."
# Start the new service in background
nohup ./convinceme_server > logs/app.log 2>&1 &
NEW_PID=$!

echo "⏱️  Waiting for service to start..."
sleep 5

# Check if process is still running
if kill -0 $NEW_PID 2>/dev/null; then
    echo "✅ Service started successfully (PID: $NEW_PID)"
    
    # Show recent logs to verify startup
    echo "📋 Recent startup logs:"
    tail -n 10 logs/app.log 2>/dev/null || echo "No logs available yet"
else
    echo "❌ Service failed to start!"
    echo "📋 Error logs:"
    tail -n 20 logs/app.log 2>/dev/null || echo "No logs available"
    exit 1
fi

REMOTE_COMMANDS

if [ $? -eq 0 ]; then
    log_success "Deployment completed successfully!"
    echo
    log_info "Service Information:"
    echo "  • Server: $SERVER"
    echo "  • Service: $SERVICE_NAME"
    echo "  • Logs: $REMOTE_PATH/logs/app.log"
    echo
    log_info "To check service status:"
    echo "  ssh $SERVER 'ps aux | grep convinceme_server'"
    echo
    log_info "To view logs:"
    echo "  ssh $SERVER 'tail -f $REMOTE_PATH/logs/app.log'"
    echo
    log_success "Deployment completed! 🎉"
else
    log_error "Deployment failed! Check the logs above for details."
    exit 1
fi

echo "================================================================="
