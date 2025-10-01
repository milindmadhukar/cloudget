# CloudGet

A high-performance Go CLI tool for downloading files from Dropbox, Google Drive, and WeTransfer with parallel chunk downloads and resume support.

## Features

- 🚀 **Multi-Service Support**: Download from Dropbox, Google Drive, and WeTransfer
- ⚡ **Parallel Downloads**: High-speed downloads using configurable chunks and connections
- 🔄 **Resume Support**: Automatically resume interrupted downloads
- 🛡️ **Robust Error Handling**: Built-in retry logic and timeout handling
- ✅ **Hash Verification**: Optional file integrity verification (MD5, SHA1, SHA256, SHA512)
- 🎯 **Smart URL Detection**: Automatically detects and converts share URLs to direct downloads
- 📱 **Rich CLI**: Comprehensive command-line interface with progress indicators
- 🐳 **Docker Support**: Lightweight Docker images for easy deployment
- 🌐 **Cross-Platform**: Native binaries for Windows, Linux, macOS, and FreeBSD

## Installation

### Option 1: Download Pre-built Binaries

Download the appropriate binary for your platform from the [releases page](https://github.com/milindmadhukar/cloudget/releases):

| Platform | Architecture | Download |
|----------|--------------|----------|
| Windows | x86_64 | `cloudget-windows-amd64.zip` |
| Windows | ARM64 | `cloudget-windows-arm64.zip` |
| Linux | x86_64 | `cloudget-linux-amd64.tar.gz` |
| Linux | ARM64 | `cloudget-linux-arm64.tar.gz` |
| Linux | ARM | `cloudget-linux-arm.tar.gz` |
| Linux | i386 | `cloudget-linux-386.tar.gz` |
| macOS | Intel | `cloudget-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `cloudget-darwin-arm64.tar.gz` |
| FreeBSD | x86_64 | `cloudget-freebsd-amd64.tar.gz` |
| FreeBSD | ARM64 | `cloudget-freebsd-arm64.tar.gz` |

```bash
# Extract and install (Linux/macOS)
tar -xzf cloudget-linux-amd64.tar.gz
chmod +x cloudget-linux-amd64
sudo mv cloudget-linux-amd64 /usr/local/bin/cloudget

# Windows
# Extract the .zip file and add to PATH
```

### Option 2: Docker

```bash
docker pull ghcr.io/milindmadhukar/cloudget:latest
```

### Option 3: Build from Source

```bash
git clone https://github.com/milindmadhukar/cloudget.git
cd cloudget
go build -o cloudget ./cmd/downloader
```

## Usage

### Basic Examples

```bash
# Download a single file
cloudget -url "https://dropbox.com/s/abc123/file.zip"

# Download multiple files
cloudget -urls "https://dropbox.com/s/abc/file1.zip,https://drive.google.com/file/d/xyz/view"

# Download from a file containing URLs
cloudget -url-file urls.txt -output-dir ./downloads

# Download with custom settings
cloudget -url "https://we.tl/t-abc123" -chunk-size 5MB -max-connections 16
```

### Command Line Options

```
-url string                URL to download
-urls string               Comma-separated list of URLs to download  
-url-file string           File containing URLs to download (one per line)
-output-dir string         Output directory for downloads (default ".")
-output string             Specific output file path (for single URL)
-filename string           Custom filename (for single URL)
-chunk-size string         Chunk size for downloads (e.g., 1MB, 512KB) (default "2MB")
-max-connections int       Maximum concurrent connections per download (default 8)
-timeout duration          Download timeout (default 5m0s)
-resume                    Enable download resume (default true)
-progress                  Show download progress (default true)
-hash-algorithm string     Hash algorithm (md5, sha1, sha256, sha512) (default "sha256")
-verify-hash string        Expected hash for verification
-verbose                   Enable verbose logging
-quiet                     Suppress all output except errors
-help                      Show help message
```

### Docker Usage

```bash
# Basic download
docker run --rm -v "$(pwd):/downloads" ghcr.io/milindmadhukar/cloudget:latest \
  -url "https://dropbox.com/s/abc123/file.zip"

# Download to specific directory
docker run --rm -v "/path/to/downloads:/downloads" ghcr.io/milindmadhukar/cloudget:latest \
  -url "https://drive.google.com/file/d/xyz/view" -output-dir /downloads

# High-performance download
docker run --rm -v "$(pwd):/downloads" ghcr.io/milindmadhukar/cloudget:latest \
  -url "https://we.tl/t-abc123" -chunk-size 5MB -max-connections 16
```

## Supported Services

### Dropbox
- Standard share URLs: `https://dropbox.com/s/abc123/file.zip`
- New share URLs: `https://dropbox.com/scl/fi/abc123/file.zip`
- Direct URLs: `https://dl.dropboxusercontent.com/s/abc123/file.zip`

### Google Drive
- File view URLs: `https://drive.google.com/file/d/FILE_ID/view`
- Direct URLs: `https://drive.google.com/uc?id=FILE_ID`
- Docs URLs: `https://docs.google.com/document/d/FILE_ID`

### WeTransfer
- Transfer URLs: `https://we.tl/t-TRANSFER_ID`
- Wetransfer.com URLs: `https://wetransfer.com/downloads/TRANSFER_ID`

## Performance Tuning

### Optimal Settings by File Size

**Small Files (< 10MB):**
```bash
cloudget -url "URL" -chunk-size 1MB -max-connections 4
```

**Medium Files (10MB - 1GB):**
```bash
cloudget -url "URL" -chunk-size 2MB -max-connections 8  # Default
```

**Large Files (> 1GB):**
```bash
cloudget -url "URL" -chunk-size 5MB -max-connections 16
```

**Slow/Unstable Network:**
```bash
cloudget -url "URL" -chunk-size 512KB -max-connections 4 -timeout 10m
```

## Advanced Usage

### Hash Verification

```bash
# Verify file integrity
cloudget -url "URL" -verify-hash "expected_sha256_hash" -hash-algorithm sha256
```

### Batch Downloads

```bash
# Create a file with URLs (one per line)
echo "https://dropbox.com/s/abc/file1.zip" > urls.txt
echo "https://drive.google.com/file/d/xyz/view" >> urls.txt
echo "https://we.tl/t-def456" >> urls.txt

# Download all files
cloudget -url-file urls.txt -output-dir ./downloads
```

### Resume Downloads

```bash
# Downloads automatically resume by default
# To disable resume:
cloudget -url "URL" -resume=false
```

## Building

### Local Build

```bash
# Build for current platform
go build -o cloudget ./cmd/downloader

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o cloudget-linux-amd64 ./cmd/downloader
GOOS=windows GOARCH=amd64 go build -o cloudget-windows-amd64.exe ./cmd/downloader
GOOS=darwin GOARCH=arm64 go build -o cloudget-darwin-arm64 ./cmd/downloader
```

### Docker Build

```bash
# Build Docker image
docker build -t cloudget .

# Multi-platform build
docker buildx build --platform linux/amd64,linux/arm64 -t cloudget .
```

## Development

### Running Tests

```bash
go test ./...
```

### Code Structure

```
cmd/downloader/          # CLI application
pkg/
├── interfaces/          # Interface definitions  
├── downloader/          # Core download manager
├── services/            # Service implementations
│   ├── dropbox/         # Dropbox service
│   ├── gdrive/          # Google Drive service
│   └── wetransfer/      # WeTransfer service
└── utils/               # Utilities (hash, HTTP, resume)
```

## Troubleshooting

### Common Issues

**"Service not found for URL"**
- The URL format may not be supported
- Check that the URL is a valid share link

**"Failed to convert URL"** 
- The share URL may be private or expired
- Try accessing the URL in a browser first

**"Download failed"**
- Check your internet connection
- Try reducing max-connections or chunk-size
- Verify the file is still available

**Slow Downloads**
- Reduce max-connections to 4-6
- Increase chunk-size to 5MB
- Check available bandwidth

### Performance Tips

1. **Connection Count**: Start with 8, adjust based on performance
2. **Chunk Size**: Larger chunks (2-5MB) are usually more efficient  
3. **Network**: Use fewer connections on unstable networks
4. **Resume**: Always enabled by default for reliability
5. **Verification**: Only use hash verification when integrity is critical

## License

MIT License - see LICENSE file for details.