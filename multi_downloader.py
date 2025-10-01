#!/usr/bin/env python3
"""
Multi-Service Parallel File Downloader

A high-performance script for downloading files from multiple cloud services using parallel chunks.
Supports:
- Dropbox
- Google Drive  
- WeTransfer

Features:
- Async parallel chunk downloads using aiohttp
- Real-time progress tracking with tqdm
- Resume interrupted downloads
- Hash verification
- Robust error handling and retry logic
- Automatic service detection from URLs
- Manual service selection
"""

import asyncio
import argparse
import logging
import sys
import importlib.util
from pathlib import Path
from typing import Optional, Tuple

def load_service_module(service_name: str):
    """Dynamically load service module"""
    try:
        service_path = Path(__file__).parent / 'services' / f'{service_name}.py'
        spec = importlib.util.spec_from_file_location(f"services.{service_name}", service_path)
        if spec and spec.loader:
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            return module
        raise ImportError(f"Could not load service: {service_name}")
    except Exception as e:
        # Fallback to direct import
        if service_name == 'dropbox':
            from services.dropbox import DropboxService
            class Module:
                DropboxService = DropboxService
            return Module()
        elif service_name == 'gdrive':
            from services.gdrive import GoogleDriveService  
            class Module:
                GoogleDriveService = GoogleDriveService
            return Module()
        elif service_name == 'wetransfer':
            from services.wetransfer import WeTransferService
            class Module:
                WeTransferService = WeTransferService
            return Module()
        else:
            raise ImportError(f"Could not load service: {service_name}")

class MultiServiceDownloader:
    """Multi-service file downloader with automatic service detection"""
    
    def __init__(self):
        # Configure logging
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s - %(message)s',
            handlers=[
                logging.StreamHandler(),
                logging.FileHandler('multi_downloader.log')
            ]
        )
        self.logger = logging.getLogger(__name__)
    
    def detect_service(self, url: str) -> Optional[str]:
        """Detect which service should handle the URL"""
        url_lower = url.lower()
        
        if 'dropbox.com' in url_lower:
            return 'dropbox'
        elif 'drive.google.com' in url_lower or 'docs.google.com' in url_lower:
            return 'gdrive'
        elif 'wetransfer.com' in url_lower or 'we.tl' in url_lower:
            return 'wetransfer'
        
        return None
    
    def get_supported_services(self) -> list:
        """Get list of supported services"""
        return ['dropbox', 'gdrive', 'wetransfer']
    
    def get_user_service_choice(self, url: str) -> Optional[str]:
        """Interactive service selection"""
        # Try auto-detection first
        detected_service = self.detect_service(url)
        
        if detected_service:
            print(f"🔍 Auto-detected service: {detected_service}")
            
            choice = input("Use auto-detected service? (Y/n): ").strip().lower()
            if choice in ['', 'y', 'yes']:
                return detected_service
        
        # Manual selection
        print("\n📋 Available services:")
        services = self.get_supported_services()
        for i, service in enumerate(services, 1):
            print(f"  {i}. {service}")
        
        while True:
            try:
                choice = input("\nSelect service (number or name): ").strip()
                
                # Try parsing as number
                if choice.isdigit():
                    idx = int(choice) - 1
                    if 0 <= idx < len(services):
                        return services[idx]
                    else:
                        print("❌ Invalid number. Please try again.")
                        continue
                
                # Try parsing as service name
                if choice.lower() in [s.lower() for s in services]:
                    return choice.lower()
                
                print("❌ Invalid choice. Please try again.")
                
            except (ValueError, KeyboardInterrupt):
                print("\n❌ Operation cancelled.")
                return None
        
        return None
    
    def create_service(self, service_name: str, **kwargs):
        """Create service instance"""
        try:
            if service_name == 'dropbox':
                module = load_service_module('dropbox')
                return module.DropboxService(**kwargs)
            elif service_name == 'gdrive':
                module = load_service_module('gdrive')
                return module.GoogleDriveService(**kwargs)
            elif service_name == 'wetransfer':
                module = load_service_module('wetransfer')
                return module.WeTransferService(**kwargs)
            else:
                raise ValueError(f"Unknown service: {service_name}")
        except Exception as e:
            self.logger.error(f"Failed to create service {service_name}: {e}")
            raise
    
    async def download_file(self, url: str, service_name: str = None, **kwargs):
        """Download file using appropriate service"""
        
        # Determine service
        if service_name:
            if service_name not in self.get_supported_services():
                raise ValueError(f"Unknown service: {service_name}. Available: {self.get_supported_services()}")
            selected_service = service_name
        else:
            detected = self.detect_service(url)
            if not detected:
                raise ValueError(f"Could not detect service for URL. Supported services: {self.get_supported_services()}")
            selected_service = detected
        
        # Extract service-specific parameters
        service_kwargs = {
            'max_connections': kwargs.get('connections', 8),
            'chunk_size': kwargs.get('chunk_size', 2*1024*1024),
            'timeout': kwargs.get('timeout', 300)
        }
        
        # Create service instance
        service = self.create_service(selected_service, **service_kwargs)
        
        try:
            self.logger.info(f"Using {selected_service} service for download")
            
            return await service.download_file(
                url=url,
                output_dir=kwargs.get('output_dir', '.'),
                custom_filename=kwargs.get('filename'),
                custom_path=kwargs.get('custom_path'),
                resume=kwargs.get('resume', True),
                verify_hash=kwargs.get('verify_hash')
            )
        finally:
            # Clean up service resources
            if hasattr(service, '__aexit__'):
                await service.__aexit__(None, None, None)


def get_user_input() -> Tuple[str, int, str, Optional[str], Optional[str], Optional[str], bool, Optional[str]]:
    """Get user input for download parameters"""
    print("🔗 Multi-Service Parallel Downloader")
    print("=" * 50)
    
    # Get URL
    while True:
        url = input("Enter download URL: ").strip()
        if url:
            break
        print("❌ Please enter a valid URL")
    
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
    
    # Ask for service selection
    service_choice = input("Force specific service? (dropbox/gdrive/wetransfer, or leave blank for auto-detect): ").strip() or None
    
    return url, connections, output_dir, custom_filename, custom_path, verify_hash, resume, service_choice


async def main():
    """Main function"""
    parser = argparse.ArgumentParser(description="Multi-Service Parallel File Downloader")
    parser.add_argument("--url", help="URL to download")
    parser.add_argument("--service", choices=['dropbox', 'gdrive', 'wetransfer'], help="Force specific service")
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
    
    downloader = MultiServiceDownloader()
    
    try:
        if args.interactive or not args.url:
            # Interactive mode
            url, connections, output_dir, custom_filename, custom_path, verify_hash, resume, service_choice = get_user_input()
            
            # If no service specified, let user choose
            if not service_choice:
                service_choice = downloader.get_user_service_choice(url)
                if not service_choice:
                    print("❌ No service selected. Exiting.")
                    return
        else:
            # Command line mode
            url = args.url
            connections = args.connections
            output_dir = args.output_dir
            custom_filename = args.filename
            custom_path = args.custom_path
            verify_hash = args.verify_hash
            resume = not args.no_resume
            service_choice = args.service
        
        print(f"\n📥 Starting download...")
        print(f"URL: {url}")
        print(f"Service: {service_choice or 'auto-detect'}")
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
            service_name=service_choice,
            connections=connections,
            output_dir=output_dir,
            filename=custom_filename,
            custom_path=custom_path,
            chunk_size=args.chunk_size,
            resume=resume,
            verify_hash=verify_hash,
            timeout=args.timeout
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
        print("Install with: pip install aiohttp aiofiles tqdm tenacity requests")
        sys.exit(1)
    
    # Run the main function
    asyncio.run(main())