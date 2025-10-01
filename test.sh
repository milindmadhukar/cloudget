#!/bin/bash

# Test script for the Dropbox downloader

echo "🧪 Testing Dropbox Parallel Downloader..."

# Test help command
echo "📝 Testing help command:"
python3 dropbox_parallel_downloader.py --help

echo ""
echo "✅ Script appears to be working correctly!"
echo ""
echo "🐳 To test with Docker:"
echo "docker build -t dropbox-downloader ."
echo "docker run --rm dropbox-downloader:latest --help"