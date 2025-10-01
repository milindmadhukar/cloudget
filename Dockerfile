# Use Python 3.11 slim image for smaller size
FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Create downloads directory
RUN mkdir -p /downloads

# Install system dependencies for faster builds
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better Docker layer caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy the application
COPY dropbox_parallel_downloader.py .

# Create non-root user for security
RUN useradd --create-home --shell /bin/bash app \
    && chown -R app:app /app /downloads
USER app

# Set default download directory
ENV DOWNLOAD_DIR=/downloads

# Make script executable
RUN chmod +x dropbox_parallel_downloader.py

# Default command
ENTRYPOINT ["python", "dropbox_parallel_downloader.py"]
CMD ["--help"]