#!/usr/bin/env python3
"""
Service factory and URL detection logic for multi-service downloader
"""

import logging
from typing import Optional, List, Type, Any
from urllib.parse import urlparse


class ServiceDetector:
    """Detects which service should handle a given URL"""
    
    def __init__(self):
        # We'll import services locally to avoid import issues
        self.logger = logging.getLogger(__name__)
    
    def detect_service_name(self, url: str) -> Optional[str]:
        """Detect which service name can handle the given URL"""
        url_lower = url.lower()
        
        if 'dropbox.com' in url_lower:
            return 'dropbox'
        elif 'drive.google.com' in url_lower or 'docs.google.com' in url_lower:
            return 'gdrive'
        elif 'wetransfer.com' in url_lower or 'we.tl' in url_lower:
            return 'wetransfer'
        
        self.logger.warning(f"No service found for URL: {url}")
        return None
    
    def get_supported_services(self) -> List[str]:
        """Get list of supported service names"""
        return ['dropbox', 'gdrive', 'wetransfer']


class ServiceFactory:
    """Factory for creating download service instances"""
    
    def __init__(self):
        self.detector = ServiceDetector()
        self.logger = logging.getLogger(__name__)
    
    def create_service(self, url: str = None, service_name: str = None, **kwargs):
        """Create appropriate service instance"""
        # Import services here to avoid circular imports
        import sys
        from pathlib import Path
        sys.path.append(str(Path(__file__).parent / 'services'))
        
        if service_name:
            # User specified service
            detected_service_name = service_name.lower()
        elif url:
            # Auto-detect from URL
            detected_service_name = self.detector.detect_service_name(url)
            if not detected_service_name:
                raise ValueError(f"Unsupported URL. Supported services: {self.detector.get_supported_services()}")
        else:
            raise ValueError("Either url or service_name must be provided")
        
        # Create service instance based on detected service
        if detected_service_name == 'dropbox':
            from dropbox import DropboxService
            return DropboxService(**kwargs)
        elif detected_service_name in ['gdrive', 'googledrive']:
            from gdrive import GoogleDriveService
            return GoogleDriveService(**kwargs)
        elif detected_service_name == 'wetransfer':
            from wetransfer import WeTransferService
            return WeTransferService(**kwargs)
        else:
            raise ValueError(f"Unknown service: {detected_service_name}. Supported services: {self.detector.get_supported_services()}")
    
    def get_supported_services(self) -> List[str]:
        """Get list of supported service names"""
        return self.detector.get_supported_services()


def get_user_service_choice(url: str) -> Optional[str]:
    """Interactive service selection"""
    detector = ServiceDetector()
    
    # Try auto-detection first
    detected_service = detector.detect_service_name(url)
    
    if detected_service:
        print(f"🔍 Auto-detected service: {detected_service}")
        
        choice = input("Use auto-detected service? (Y/n): ").strip().lower()
        if choice in ['', 'y', 'yes']:
            return detected_service
    
    # Manual selection
    print("\n📋 Available services:")
    services = detector.get_supported_services()
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