# Dropbox Parallel Downloader

A high-performance Python script for downloading files from Dropbox URLs using parallel chunk downloads with async/await.

## Features

- 🚀 **Parallel Downloads**: Downloads files in parallel chunks for maximum speed
- 📊 **Real-time Progress**: Beautiful progress bars with download speed and ETA
- 🔄 **Resume Support**: Resume interrupted downloads automatically
- 🛡️ **Error Handling**: Robust retry logic with exponential backoff
- ✅ **Hash Verification**: Optional file integrity verification
- 🎯 **Smart Detection**: Automatically detects filename and file size
- 🌐 **URL Support**: Works with various Dropbox share URL formats
- 📱 **Interactive Mode**: User-friendly interactive prompts
- 🖥️ **CLI Mode**: Full command-line interface for automation
- 🐳 **Docker Support**: Pre-built Docker images for easy deployment

## Installation

### Option 1: Docker (Recommended)

Pull the pre-built Docker image:

```bash
docker pull ghcr.io/milindmadhukar/dropbox-downloader:latest
```

### Option 2: Local Installation

1. Clone or download the script
2. Install dependencies:
```bash
pip install -r requirements.txt
```

## Usage

### Docker Usage

#### Quick Start with Interactive Mode

```bash
docker run -it --rm \
  -v "$(pwd)/downloads:/downloads" \
  ghcr.io/milindmadhukar/dropbox-downloader:latest \
  --interactive
```

#### Command Line Mode

```bash
docker run --rm \
  -v "$(pwd)/downloads:/downloads" \
  ghcr.io/milindmadhukar/dropbox-downloader:latest \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --connections 8 \
  --output-dir /downloads
```

#### Download to Custom Path

```bash
docker run --rm \
  -v "$(pwd)/my-downloads:/app/downloads" \
  ghcr.io/milindmadhukar/dropbox-downloader:latest \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --custom-path "/app/downloads/my-file.zip"
```

#### High-Performance Download

```bash
docker run --rm \
  -v "$(pwd)/downloads:/downloads" \
  ghcr.io/milindmadhukar/dropbox-downloader:latest \
  --url "https://www.dropbox.com/s/abc123/largefile.zip?dl=0" \
  --connections 16 \
  --chunk-size 5242880 \
  --output-dir /downloads
```

#### With Hash Verification

```bash
docker run --rm \
  -v "$(pwd)/downloads:/downloads" \
  ghcr.io/milindmadhukar/dropbox-downloader:latest \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --verify-hash "sha256_hash_here" \
  --output-dir /downloads
```

### Local Usage

### Interactive Mode

Simply run the script and follow the prompts:

```bash
python dropbox_parallel_downloader.py --interactive
```

### Command Line Mode

Download with custom parameters:

```bash
python dropbox_parallel_downloader.py \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --connections 8 \
  --output-dir "./downloads" \
  --filename "my_file.zip"
```

### Command Line Options

- `--url`: Dropbox URL to download (required)
- `--connections`: Number of parallel connections (1-16, default: 8)
- `--output-dir`: Output directory (default: current directory)
- `--custom-path`: Full download path including filename
- `--filename`: Custom filename (optional)
- `--chunk-size`: Chunk size in bytes (default: 2MB)
- `--verify-hash`: Expected file hash for verification (optional)
- `--no-resume`: Disable resume capability
- `--timeout`: Download timeout in seconds (default: 300)
- `--interactive`: Enable interactive mode

## Examples

### Basic Download
```bash
python dropbox_parallel_downloader.py --url "https://www.dropbox.com/s/abc123/document.pdf?dl=0"
```

### High-Performance Download
```bash
python dropbox_parallel_downloader.py \
  --url "https://www.dropbox.com/s/abc123/largefile.zip?dl=0" \
  --connections 16 \
  --chunk-size 5242880 \
  --output-dir "./downloads"
```

### With Custom Download Path
```bash
python dropbox_parallel_downloader.py \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --custom-path "/path/to/my-custom-file.zip"
```

### With Hash Verification
```bash
python dropbox_parallel_downloader.py \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --verify-hash "sha256_hash_here" \
  --filename "verified_file.zip"
```

### Automated Download (No Resume)
```bash
python dropbox_parallel_downloader.py \
  --url "https://www.dropbox.com/s/abc123/file.zip?dl=0" \
  --no-resume \
  --timeout 600
```

## Supported URL Formats

The script automatically handles various Dropbox URL formats:

- Standard share URLs: `https://www.dropbox.com/s/abc123/file.zip?dl=0`
- New share URLs: `https://www.dropbox.com/scl/fi/abc123/file.zip?rlkey=def456&dl=0`
- Raw URLs: `https://dl.dropboxusercontent.com/s/abc123/file.zip`

## Performance Tuning

### Optimal Settings for Different Scenarios

**Small Files (< 10MB):**
```bash
--connections 4 --chunk-size 1048576  # 1MB chunks
```

**Medium Files (10MB - 1GB):**
```bash
--connections 8 --chunk-size 2097152  # 2MB chunks (default)
```

**Large Files (> 1GB):**
```bash
--connections 12 --chunk-size 5242880  # 5MB chunks
```

**Slow Network:**
```bash
--connections 4 --chunk-size 524288 --timeout 600  # 512KB chunks
```

## Technical Details

### How It Works

1. **URL Conversion**: Converts Dropbox share URLs to direct download URLs
2. **File Analysis**: Uses HEAD requests to determine file size and range support
3. **Chunk Calculation**: Divides file into optimal chunks based on size and settings
4. **Parallel Download**: Downloads chunks concurrently using aiohttp
5. **Progress Tracking**: Real-time progress updates with tqdm
6. **File Assembly**: Combines chunks in correct order
7. **Verification**: Optional hash verification for file integrity

### Error Handling

- **Automatic Retry**: Failed chunks are retried with exponential backoff
- **Timeout Handling**: Configurable timeouts for different scenarios
- **Network Errors**: Robust handling of connection issues
- **Resume Support**: Incomplete downloads can be resumed
- **Cleanup**: Automatic cleanup of temporary files on errors

### Logging

The script creates detailed logs in `dropbox_downloader.log` including:
- Download progress and statistics
- Error messages and retry attempts
- Performance metrics
- Hash verification results

## Troubleshooting

### Common Issues

**"Range requests not supported"**
- The server doesn't support parallel downloads
- Script will automatically fall back to single-threaded download

**"Permission denied" or "403 Forbidden"**
- The Dropbox link may be private or expired
- Try accessing the URL in a browser first

**"Hash verification failed"**
- The downloaded file is corrupted
- Try downloading again with resume enabled

**Slow download speeds**
- Reduce the number of connections (`--connections 4`)
- Increase chunk size (`--chunk-size 5242880`)
- Check your network connection

### Performance Tips

1. **Optimal Connections**: Start with 8 connections, adjust based on performance
2. **Chunk Size**: Larger chunks (2-5MB) are usually more efficient
3. **Network Stability**: Use fewer connections on unstable networks
4. **Resume Downloads**: Always enable resume for large files
5. **Hash Verification**: Only use when file integrity is critical

## Building Docker Image Locally

If you want to build the Docker image yourself:

```bash
# Clone the repository
git clone <your-repo-url>
cd dropbox-parallel-downloader

# Build the image
docker build -t dropbox-downloader .

# Run the locally built image
docker run -it --rm \
  -v "$(pwd)/downloads:/downloads" \
  dropbox-downloader:latest \
  --interactive
```

## Docker Image Details

- **Base Image**: `python:3.11-slim` (optimized for size)
- **Supported Architectures**: `linux/amd64`, `linux/arm64`
- **Image Size**: ~150MB (compressed)
- **Security**: Runs as non-root user
- **Auto-built**: Images are automatically built and pushed via GitHub Actions

### Docker Environment Variables

- `DOWNLOAD_DIR`: Default download directory (default: `/downloads`)

### Volume Mounts

Always mount a volume to persist downloaded files:

```bash
# Mount current directory's downloads folder
-v "$(pwd)/downloads:/downloads"

# Mount a specific path
-v "/path/to/downloads:/downloads"

# Mount with custom internal path
-v "/local/path:/app/files"
```

## Dependencies

- `aiohttp>=3.8.0`: Async HTTP client
- `aiofiles>=22.0.0`: Async file operations
- `tqdm>=4.64.0`: Progress bars
- `tenacity>=8.2.0`: Retry logic

## License

This script is provided as-is for educational and personal use.
