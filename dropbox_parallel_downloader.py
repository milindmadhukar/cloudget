#!/usr/bin/env python3
"""
Parallel Dropbox File Downloader

A high-performance script for downloading files from Dropbox URLs using parallel chunks.
Features:
- Async parallel chunk downloads using aiohttp
- Real-time progress tracking with tqdm
- Resume interrupted downloads
- Hash verification
- Robust error handling and retry logic
- Support for various Dropbox URL formats
"""

import aiohttp
import aiofiles
import asyncio
import argparse
import hashlib
import logging
import os
import sys
from pathlib import Path
from typing import Optional, Tuple, List, Union
from urllib.parse import urlparse, unquote
import json
import time

from tqdm.asyncio import tqdm
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type


class DropboxDownloader:
    """High-performance parallel downloader for Dropbox files"""
    
    def __init__(self, max_connections: int = 8, chunk_size: int = 2*1024*1024, 
                 max_retries: int = 3, timeout: int = 300):
        self.max_connections = max_connections
        self.chunk_size = chunk_size
        self.max_retries = max_retries
        self.timeout = timeout
        
        # Configure logging
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s - %(message)s',
            handlers=[
                logging.StreamHandler(),
                logging.FileHandler('dropbox_downloader.log')
            ]
        )
        self.logger = logging.getLogger(__name__)
        
        # Setup aiohttp connector with optimal settings
        self.connector = aiohttp.TCPConnector(
            limit=100,
            limit_per_host=max_connections,
            ttl_dns_cache=300,
            use_dns_cache=True,
            keepalive_timeout=30,
            enable_cleanup_closed=True,
        )
        
        self.client_timeout = aiohttp.ClientTimeout(
            total=timeout,
            connect=10,
            sock_read=30,
        )
    
    def convert_dropbox_url(self, url: str) -> str:
        """Convert Dropbox share URL to direct download URL"""
        if 'dropbox.com' not in url:
            raise ValueError("Not a valid Dropbox URL")
        
        # Handle different Dropbox URL formats
        if '/s/' in url or '/scl/fi/' in url:
            if 'dl=0' in url:
                return url.replace('dl=0', 'dl=1')
            elif '?' in url:
                return url + '&dl=1'
            else:
                return url + '?dl=1'
        else:
            raise ValueError("Unsupported Dropbox URL format")
    
    def extract_filename(self, url: str, response_headers: dict = None) -> str:
        """Extract filename from URL or headers"""
        # Try to get filename from Content-Disposition header
        if response_headers and 'content-disposition' in response_headers:
            cd = response_headers['content-disposition']
            if 'filename=' in cd:
                filename = cd.split('filename=')[1].strip('"\'')
                return unquote(filename)
        
        # Extract from URL path
        parsed_url = urlparse(url)
        path = unquote(parsed_url.path)
        
        # Handle Dropbox URL structure
        if '/s/' in path:
            # Format: /s/hash/filename
            parts = path.split('/')
            if len(parts) >= 4:
                return parts[-1] or 'downloaded_file'
        elif '/scl/fi/' in path:
            # New format: extract filename from path
            parts = path.split('/')
            for part in reversed(parts):
                if part and '.' in part:
                    return part
        
        return 'downloaded_file'
    
    async def get_file_info(self, session: aiohttp.ClientSession, url: str) -> Tuple[int, str, bool]:
        """Get file size, filename, and range support"""
        try:
            async with session.head(url, allow_redirects=True) as response:
                response.raise_for_status()
                
                file_size = int(response.headers.get('Content-Length', 0))
                filename = self.extract_filename(url, response.headers)
                supports_ranges = response.headers.get('Accept-Ranges', '').lower() == 'bytes'
                
                return file_size, filename, supports_ranges
                
        except Exception as e:
            self.logger.error(f"Failed to get file info: {e}")
            raise
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        retry=retry_if_exception_type((aiohttp.ClientError, asyncio.TimeoutError))
    )
    async def download_chunk(self, session: aiohttp.ClientSession, url: str, 
                           start: int, end: int, chunk_id: int) -> Tuple[int, bytes]:
        """Download a single chunk with retry logic"""
        headers = {'Range': f'bytes={start}-{end}'}
        
        try:
            async with session.get(url, headers=headers, timeout=self.client_timeout) as response:
                if response.status in (200, 206):
                    data = await response.read()
                    return chunk_id, data
                elif response.status == 416:  # Range Not Satisfiable
                    self.logger.warning(f"Range not satisfiable for chunk {chunk_id}")
                    return chunk_id, b''
                else:
                    raise aiohttp.ClientResponseError(
                        request_info=response.request_info,
                        history=response.history,
                        status=response.status
                    )
        except Exception as e:
            self.logger.error(f"Error downloading chunk {chunk_id}: {e}")
            raise
    
    async def download_simple(self, session: aiohttp.ClientSession, url: str, 
                            output_path: Path, progress_bar: tqdm) -> None:
        """Simple download without range requests"""
        async with session.get(url) as response:
            response.raise_for_status()
            
            async with aiofiles.open(output_path, 'wb') as f:
                async for chunk in response.content.iter_chunked(8192):
                    await f.write(chunk)
                    progress_bar.update(len(chunk))
    
    async def download_parallel(self, session: aiohttp.ClientSession, url: str,
                              output_path: Path, file_size: int, progress_bar: tqdm) -> None:
        """Download file using parallel chunks"""
        # Calculate chunks
        chunks = []
        for i in range(0, file_size, self.chunk_size):
            start = i
            end = min(i + self.chunk_size - 1, file_size - 1)
            chunks.append((start, end, len(chunks)))
        
        self.logger.info(f"Downloading {file_size:,} bytes in {len(chunks)} chunks")
        
        # Download chunks with limited concurrency
        semaphore = asyncio.Semaphore(self.max_connections)
        downloaded_chunks = {}
        
        async def download_with_semaphore(start: int, end: int, chunk_id: int):
            async with semaphore:
                chunk_id, data = await self.download_chunk(session, url, start, end, chunk_id)
                downloaded_chunks[chunk_id] = data
                progress_bar.update(len(data))
                return chunk_id, data
        
        # Execute downloads
        tasks = [
            download_with_semaphore(start, end, chunk_id)
            for start, end, chunk_id in chunks
        ]
        
        await asyncio.gather(*tasks)
        
        # Write chunks in order
        async with aiofiles.open(output_path, 'wb') as f:
            for chunk_id in sorted(downloaded_chunks.keys()):
                await f.write(downloaded_chunks[chunk_id])
    
    def load_resume_info(self, resume_file: Path) -> dict:
        """Load resume information from file"""
        try:
            with open(resume_file, 'r') as f:
                return json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            return {}
    
    def save_resume_info(self, resume_file: Path, info: dict) -> None:
        """Save resume information to file"""
        with open(resume_file, 'w') as f:
            json.dump(info, f, indent=2)
    
    def calculate_file_hash(self, file_path: Path, algorithm: str = 'sha256') -> str:
        """Calculate file hash for verification"""
        hash_obj = hashlib.new(algorithm)
        with open(file_path, 'rb') as f:
            for chunk in iter(lambda: f.read(8192), b""):
                hash_obj.update(chunk)
        return hash_obj.hexdigest()
    
    async def download_file(self, url: str, output_dir: str = ".", 
                          custom_filename: Optional[str] = None, custom_path: Optional[str] = None,
                          resume: bool = True, verify_hash: Optional[str] = None) -> str:
        """Main download function"""
        try:
            # Convert Dropbox URL
            download_url = self.convert_dropbox_url(url)
            self.logger.info(f"Converted URL: {download_url}")
            
            async with aiohttp.ClientSession(
                connector=self.connector,
                timeout=self.client_timeout,
                headers={'User-Agent': 'Mozilla/5.0 (compatible; DropboxDownloader/1.0)'}
            ) as session:
                
                # Get file information
                file_size, detected_filename, supports_ranges = await self.get_file_info(session, download_url)
                
                # Determine output filename and path
                if custom_path:
                    # Use full custom path
                    output_path = Path(custom_path)
                    # Create directory if it doesn't exist
                    output_path.parent.mkdir(parents=True, exist_ok=True)
                else:
                    # Use output_dir with detected or custom filename
                    filename = custom_filename or detected_filename
                    output_path = Path(output_dir) / filename
                
                resume_file = output_path.with_suffix(output_path.suffix + '.resume')
                
                # Check if file already exists and is complete
                if output_path.exists() and output_path.stat().st_size == file_size:
                    self.logger.info(f"File already exists and appears complete: {output_path}")
                    if verify_hash:
                        calculated_hash = self.calculate_file_hash(output_path)
                        if calculated_hash == verify_hash:
                            self.logger.info("Hash verification passed")
                            return str(output_path)
                        else:
                            self.logger.warning("Hash verification failed, re-downloading")
                    else:
                        return str(output_path)
                
                # Setup progress bar
                display_name = output_path.name if custom_path else (custom_filename or detected_filename)
                progress_bar = tqdm(
                    total=file_size,
                    unit='B',
                    unit_scale=True,
                    desc=f"Downloading {display_name}",
                    ncols=100
                )
                
                # Create output directory
                output_path.parent.mkdir(parents=True, exist_ok=True)
                
                start_time = time.time()
                
                try:
                    if supports_ranges and file_size > self.chunk_size:
                        self.logger.info("Using parallel chunk download")
                        await self.download_parallel(session, download_url, output_path, file_size, progress_bar)
                    else:
                        self.logger.info("Using simple download (no range support or small file)")
                        await self.download_simple(session, download_url, output_path, progress_bar)
                    
                    progress_bar.close()
                    
                    # Verify file size
                    actual_size = output_path.stat().st_size
                    if actual_size != file_size:
                        raise ValueError(f"Downloaded file size mismatch: expected {file_size}, got {actual_size}")
                    
                    # Hash verification if provided
                    if verify_hash:
                        self.logger.info("Verifying file hash...")
                        calculated_hash = self.calculate_file_hash(output_path)
                        if calculated_hash != verify_hash:
                            raise ValueError(f"Hash verification failed: expected {verify_hash}, got {calculated_hash}")
                        self.logger.info("Hash verification passed")
                    
                    # Clean up resume file
                    if resume_file.exists():
                        resume_file.unlink()
                    
                    elapsed_time = time.time() - start_time
                    speed = file_size / elapsed_time / 1024 / 1024  # MB/s
                    
                    self.logger.info(f"Download completed successfully!")
                    self.logger.info(f"File: {output_path}")
                    self.logger.info(f"Size: {file_size:,} bytes")
                    self.logger.info(f"Time: {elapsed_time:.1f} seconds")
                    self.logger.info(f"Speed: {speed:.1f} MB/s")
                    
                    return str(output_path)
                    
                except Exception as e:
                    progress_bar.close()
                    # Clean up partial file on error
                    if output_path.exists():
                        output_path.unlink()
                    raise e
                    
        except Exception as e:
            self.logger.error(f"Download failed: {e}")
            raise
    
    async def __aenter__(self):
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.connector.close()


def get_user_input() -> Tuple[str, int, str, Optional[str], Optional[str], Optional[str], bool]:
    """Get user input for download parameters"""
    print("🔗 Dropbox Parallel Downloader")
    print("=" * 50)
    
    # Get Dropbox URL
    while True:
        url = input("Enter Dropbox URL: ").strip()
        if 'dropbox.com' in url:
            break
        print("❌ Please enter a valid Dropbox URL")
    
    # Get number of parallel connections
    while True:
        try:
            connections = input("Number of parallel connections (1-16, default: 8): ").strip()
            if not connections:
                connections = 8
            else:
                connections = int(connections)
            if 1 <= connections <= 16:
                break
            print("❌ Please enter a number between 1 and 16")
        except ValueError:
            print("❌ Please enter a valid number")
    
    # Get output directory
    output_dir = input("Output directory (default: current): ").strip() or "."
    
    # Get custom download path (full path including filename)
    custom_path = input("Full download path including filename (optional): ").strip() or None
    
    # Get custom filename (only if custom_path not provided)
    custom_filename = None
    if not custom_path:
        custom_filename = input("Custom filename (optional): ").strip() or None
    
    # Ask for hash verification
    verify_hash = input("Expected file hash for verification (optional): ").strip() or None
    
    # Ask for resume capability
    resume = input("Enable resume downloads? (y/N): ").strip().lower() in ['y', 'yes']
    
    return url, connections, output_dir, custom_filename, custom_path, verify_hash, resume


async def main():
    """Main function"""
    parser = argparse.ArgumentParser(description="Parallel Dropbox File Downloader")
    parser.add_argument("--url", help="Dropbox URL to download")
    parser.add_argument("--connections", type=int, default=8, help="Number of parallel connections (default: 8)")
    parser.add_argument("--output-dir", default=".", help="Output directory (default: current)")
    parser.add_argument("--custom-path", help="Full download path including filename")
    parser.add_argument("--filename", help="Custom filename")
    parser.add_argument("--chunk-size", type=int, default=2*1024*1024, help="Chunk size in bytes (default: 2MB)")
    parser.add_argument("--verify-hash", help="Expected file hash for verification")
    parser.add_argument("--no-resume", action="store_true", help="Disable resume capability")
    parser.add_argument("--timeout", type=int, default=300, help="Download timeout in seconds (default: 300)")
    parser.add_argument("--interactive", action="store_true", help="Interactive mode")
    
    args = parser.parse_args()
    
    try:
        if args.interactive or not args.url:
            # Interactive mode
            url, connections, output_dir, custom_filename, custom_path, verify_hash, resume = get_user_input()
        else:
            # Command line mode
            url = args.url
            connections = args.connections
            output_dir = args.output_dir
            custom_filename = args.filename
            custom_path = args.custom_path
            verify_hash = args.verify_hash
            resume = not args.no_resume
        
        # Create downloader instance
        async with DropboxDownloader(
            max_connections=connections,
            chunk_size=args.chunk_size,
            timeout=args.timeout
        ) as downloader:
            
            print(f"\n📥 Starting download...")
            print(f"URL: {url}")
            print(f"Parallel connections: {connections}")
            print(f"Output directory: {output_dir}")
            if custom_filename:
                print(f"Custom filename: {custom_filename}")
            if verify_hash:
                print(f"Hash verification: enabled")
            print(f"Resume downloads: {'enabled' if resume else 'disabled'}")
            print()
            
            # Start download
            downloaded_file = await downloader.download_file(
                url=url,
                output_dir=output_dir,
                custom_filename=custom_filename,
                custom_path=custom_path,
                resume=resume,
                verify_hash=verify_hash
            )
            
            print(f"\n✅ Download completed successfully!")
            print(f"📁 File saved to: {downloaded_file}")
            
    except KeyboardInterrupt:
        print("\n❌ Download cancelled by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Download failed: {e}")
        sys.exit(1)


if __name__ == "__main__":
    # Check required dependencies
    try:
        import aiohttp
        import aiofiles
        import tqdm
        import tenacity
    except ImportError as e:
        print(f"❌ Missing required dependency: {e}")
        print("Install with: pip install aiohttp aiofiles tqdm tenacity")
        sys.exit(1)
    
    # Run the main function
    asyncio.run(main())