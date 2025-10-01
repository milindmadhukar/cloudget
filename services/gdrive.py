#!/usr/bin/env python3
"""
Google Drive download service implementation
"""

import re
import json
import aiohttp
import aiofiles
import asyncio
import hashlib
import logging
import time
from abc import ABC, abstractmethod
from pathlib import Path
from typing import Optional, Tuple, Any
from urllib.parse import urlparse, parse_qs, unquote

from tqdm import tqdm as TqdmClass
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type


class BaseDownloadService(ABC):
    """Abstract base class for download services"""
    
    def __init__(self, max_connections: int = 8, chunk_size: int = 2*1024*1024, 
                 max_retries: int = 3, timeout: int = 300):
        self.max_connections = max_connections
        self.chunk_size = chunk_size
        self.max_retries = max_retries
        self.timeout = timeout
        
        # Configure logging
        self.logger = logging.getLogger(self.__class__.__name__)
        
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
    
    @abstractmethod
    def is_supported_url(self, url: str) -> bool:
        """Check if the service supports this URL"""
        pass
    
    @abstractmethod
    def convert_to_download_url(self, url: str) -> str:
        """Convert share URL to direct download URL"""
        pass
    
    @abstractmethod
    def extract_filename(self, url: str, response_headers=None) -> str:
        """Extract filename from URL or headers"""
        pass
    
    async def get_file_info(self, session: aiohttp.ClientSession, url: str) -> Tuple[int, str, bool]:
        """Get file size, filename, and range support"""
        try:
            headers = self.get_default_headers()
            async with session.head(url, allow_redirects=True, headers=headers) as response:
                response.raise_for_status()
                
                file_size = int(response.headers.get('Content-Length', 0))
                filename = self.extract_filename(url, response.headers)
                supports_ranges = response.headers.get('Accept-Ranges', '').lower() == 'bytes'
                
                return file_size, filename, supports_ranges
                
        except Exception as e:
            self.logger.error(f"Failed to get file info: {e}")
            raise
    
    def get_default_headers(self) -> dict:
        """Get default headers for requests"""
        return {
            'Accept-Encoding': 'identity',
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
        }
    
    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=10),
        retry=retry_if_exception_type((aiohttp.ClientError, asyncio.TimeoutError))
    )
    async def download_chunk(self, session: aiohttp.ClientSession, url: str, 
                           start: int, end: int, chunk_id: int) -> Tuple[int, bytes]:
        """Download a single chunk with retry logic"""
        headers = self.get_default_headers()
        headers['Range'] = f'bytes={start}-{end}'
        
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
                            output_path: Path, progress_bar: Any) -> None:
        """Simple download without range requests"""
        headers = self.get_default_headers()
        async with session.get(url, headers=headers) as response:
            response.raise_for_status()
            
            async with aiofiles.open(output_path, 'wb') as f:
                async for chunk in response.content.iter_chunked(8192):
                    await f.write(chunk)
                    progress_bar.update(len(chunk))
    
    async def download_parallel(self, session: aiohttp.ClientSession, url: str,
                              output_path: Path, file_size: int, progress_bar: Any) -> None:
        """Download file using parallel chunks"""
        # Calculate chunks
        chunks = []
        for i in range(0, file_size, self.chunk_size):
            start = i
            end = min(i + self.chunk_size - 1, file_size - 1)
            chunks.append((start, end, len(chunks)))
        
        self.logger.info(f"Downloading {file_size:,} bytes in {len(chunks)} chunks")
        
        # Create file with correct size using truncate (more efficient)
        async with aiofiles.open(output_path, 'wb') as f:
            await f.truncate(file_size)
        
        # Download chunks with limited concurrency and write directly to file
        semaphore = asyncio.Semaphore(self.max_connections)
        write_lock = asyncio.Lock()
        
        async def download_and_write_chunk(start: int, end: int, chunk_id: int):
            async with semaphore:
                try:
                    chunk_id, data = await self.download_chunk(session, url, start, end, chunk_id)
                    if data:
                        # Write chunk directly to file at correct position
                        async with write_lock:
                            async with aiofiles.open(output_path, 'r+b') as f:
                                await f.seek(start)
                                await f.write(data)
                        progress_bar.update(len(data))
                    return chunk_id, len(data) if data else 0
                except Exception as e:
                    self.logger.error(f"Failed to download chunk {chunk_id}: {e}")
                    raise
        
        # Execute downloads
        tasks = [
            download_and_write_chunk(start, end, chunk_id)
            for start, end, chunk_id in chunks
        ]
        
        await asyncio.gather(*tasks)
    
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
            # Convert to download URL
            download_url = self.convert_to_download_url(url)
            self.logger.info(f"Converted URL: {download_url}")
            
            async with aiohttp.ClientSession(
                connector=self.connector,
                timeout=self.client_timeout,
                headers=self.get_default_headers()
            ) as session:
                
                # Get file information
                file_size, detected_filename, supports_ranges = await self.get_file_info(session, download_url)
                
                # Determine output filename and path
                if custom_path:
                    output_path = Path(custom_path)
                    output_path.parent.mkdir(parents=True, exist_ok=True)
                else:
                    filename = custom_filename or detected_filename
                    output_path = Path(output_dir) / filename
                
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
                progress_bar = TqdmClass(
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


class GoogleDriveService(BaseDownloadService):
    """Google Drive download service"""
    
    def is_supported_url(self, url: str) -> bool:
        """Check if the service supports this URL"""
        return 'drive.google.com' in url or 'docs.google.com' in url
    
    def convert_to_download_url(self, url: str) -> str:
        """Convert Google Drive share URL to direct download URL"""
        if not self.is_supported_url(url):
            raise ValueError("Not a valid Google Drive URL")
        
        # Extract file ID from various Google Drive URL formats
        file_id = self._extract_file_id(url)
        if not file_id:
            raise ValueError("Could not extract file ID from Google Drive URL")
        
        # For large files, Google Drive requires additional parameters
        return f"https://drive.google.com/uc?export=download&id={file_id}&confirm=t"
    
    def _extract_file_id(self, url: str) -> Optional[str]:
        """Extract file ID from Google Drive URL"""
        # Pattern 1: /file/d/{file_id}/
        match = re.search(r'/file/d/([a-zA-Z0-9_-]+)', url)
        if match:
            return match.group(1)
        
        # Pattern 2: id={file_id}
        match = re.search(r'[?&]id=([a-zA-Z0-9_-]+)', url)
        if match:
            return match.group(1)
        
        # Pattern 3: /open?id={file_id}
        match = re.search(r'/open\?id=([a-zA-Z0-9_-]+)', url)
        if match:
            return match.group(1)
        
        # Pattern 4: /d/{file_id}
        match = re.search(r'/d/([a-zA-Z0-9_-]+)', url)
        if match:
            return match.group(1)
        
        return None
    
    def extract_filename(self, url: str, response_headers=None) -> str:
        """Extract filename from URL or headers"""
        # Try to get filename from Content-Disposition header
        if response_headers and 'content-disposition' in response_headers:
            cd = response_headers['content-disposition']
            if 'filename=' in cd:
                filename = cd.split('filename=')[1].strip('"\'')
                return unquote(filename)
        
        # For Google Drive, we often can't determine filename from URL alone
        # The actual filename will come from the response headers
        return 'google_drive_file'
    
    async def get_file_info(self, session, url: str):
        """Get file info with special handling for Google Drive virus scan page"""
        try:
            headers = self.get_default_headers()
            async with session.head(url, allow_redirects=True, headers=headers) as response:
                # Google Drive might redirect to virus scan warning for large files
                if 'accounts.google.com' in str(response.url) or 'drive.google.com/uc' in str(response.url):
                    # Try to get the actual download link
                    download_url = await self._handle_virus_scan_redirect(session, url)
                    if download_url:
                        return await super().get_file_info(session, download_url)
                
                response.raise_for_status()
                
                file_size = int(response.headers.get('Content-Length', 0))
                filename = self.extract_filename(url, response.headers)
                supports_ranges = response.headers.get('Accept-Ranges', '').lower() == 'bytes'
                
                return file_size, filename, supports_ranges
                
        except Exception as e:
            self.logger.error(f"Failed to get file info: {e}")
            raise
    
    async def _handle_virus_scan_redirect(self, session, url: str) -> Optional[str]:
        """Handle Google Drive virus scan redirect for large files"""
        try:
            headers = self.get_default_headers()
            async with session.get(url, headers=headers) as response:
                content = await response.text()
                
                # Look for download link in the virus scan page
                # Google Drive virus scan page contains a form with the actual download URL
                download_match = re.search(r'action="([^"]*)"[^>]*>.*?name="id"', content, re.DOTALL)
                if download_match:
                    download_url = download_match.group(1).replace('&amp;', '&')
                    return download_url
                
                # Alternative: look for direct download link
                confirm_match = re.search(r'&amp;confirm=([^&"]*)', content)
                if confirm_match:
                    file_id = self._extract_file_id(url)
                    confirm_code = confirm_match.group(1)
                    return f"https://drive.google.com/uc?export=download&confirm={confirm_code}&id={file_id}"
                
        except Exception as e:
            self.logger.warning(f"Could not handle virus scan redirect: {e}")
        
        return None