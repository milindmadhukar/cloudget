#!/usr/bin/env python3
"""
Integration tests for multi-service downloader
"""

import asyncio
import tempfile
import shutil
import unittest
from pathlib import Path
import sys
import os

# Add parent directory to path to import modules
sys.path.insert(0, str(Path(__file__).parent))

class TestServiceDetection(unittest.TestCase):
    """Test service detection functionality"""
    
    def setUp(self):
        """Set up test environment"""
        import importlib.util
        
        # Load the MultiServiceDownloader class
        main_path = Path(__file__).parent / 'multi_downloader.py'
        spec = importlib.util.spec_from_file_location("multi_downloader", main_path)
        self.multi_module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.multi_module)
        
        self.downloader = self.multi_module.MultiServiceDownloader()
    
    def test_dropbox_detection(self):
        """Test Dropbox URL detection"""
        test_urls = [
            "https://www.dropbox.com/s/abc123/file.zip?dl=0",
            "https://dropbox.com/scl/fi/def456/file.pdf",
            "https://www.dropbox.com/s/xyz789/document.docx"
        ]
        
        for url in test_urls:
            with self.subTest(url=url):
                service = self.downloader.detect_service(url)
                self.assertEqual(service, 'dropbox', f"Failed to detect Dropbox for: {url}")
    
    def test_gdrive_detection(self):
        """Test Google Drive URL detection"""
        test_urls = [
            "https://drive.google.com/file/d/abc123/view",
            "https://drive.google.com/open?id=def456",
            "https://docs.google.com/document/d/xyz789/edit"
        ]
        
        for url in test_urls:
            with self.subTest(url=url):
                service = self.downloader.detect_service(url)
                self.assertEqual(service, 'gdrive', f"Failed to detect Google Drive for: {url}")
    
    def test_wetransfer_detection(self):
        """Test WeTransfer URL detection"""
        test_urls = [
            "https://wetransfer.com/downloads/abc123",
            "https://we.tl/def456",
            "https://www.wetransfer.com/downloads/xyz789"
        ]
        
        for url in test_urls:
            with self.subTest(url=url):
                service = self.downloader.detect_service(url)
                self.assertEqual(service, 'wetransfer', f"Failed to detect WeTransfer for: {url}")
    
    def test_unsupported_url(self):
        """Test unsupported URL detection"""
        test_urls = [
            "https://example.com/file.zip",
            "https://github.com/user/repo/releases/download/v1.0/file.tar.gz",
            "ftp://files.example.com/file.bin"
        ]
        
        for url in test_urls:
            with self.subTest(url=url):
                service = self.downloader.detect_service(url)
                self.assertIsNone(service, f"Should not detect service for: {url}")


class TestServiceCreation(unittest.TestCase):
    """Test service instance creation"""
    
    def setUp(self):
        """Set up test environment"""
        import importlib.util
        
        # Load the MultiServiceDownloader class
        main_path = Path(__file__).parent / 'multi_downloader.py'
        spec = importlib.util.spec_from_file_location("multi_downloader", main_path)
        self.multi_module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.multi_module)
        
        self.downloader = self.multi_module.MultiServiceDownloader()
    
    def test_dropbox_service_creation(self):
        """Test Dropbox service creation"""
        service = self.downloader.create_service('dropbox')
        self.assertIsNotNone(service)
        self.assertTrue(hasattr(service, 'download_file'))
        self.assertTrue(hasattr(service, 'is_supported_url'))
    
    def test_gdrive_service_creation(self):
        """Test Google Drive service creation"""
        service = self.downloader.create_service('gdrive')
        self.assertIsNotNone(service)
        self.assertTrue(hasattr(service, 'download_file'))
        self.assertTrue(hasattr(service, 'is_supported_url'))
    
    def test_wetransfer_service_creation(self):
        """Test WeTransfer service creation"""
        service = self.downloader.create_service('wetransfer')
        self.assertIsNotNone(service)
        self.assertTrue(hasattr(service, 'download_file'))
        self.assertTrue(hasattr(service, 'is_supported_url'))
    
    def test_invalid_service_creation(self):
        """Test invalid service creation raises error"""
        with self.assertRaises(ValueError):
            self.downloader.create_service('invalid_service')


class TestServiceFunctionality(unittest.TestCase):
    """Test individual service functionality"""
    
    def setUp(self):
        """Set up test environment"""
        import importlib.util
        
        # Load service modules
        self.services_path = Path(__file__).parent / 'services'
        
        # Load Dropbox service
        dropbox_spec = importlib.util.spec_from_file_location(
            "dropbox", self.services_path / 'dropbox.py'
        )
        self.dropbox_module = importlib.util.module_from_spec(dropbox_spec)
        dropbox_spec.loader.exec_module(self.dropbox_module)
        
        # Load Google Drive service
        gdrive_spec = importlib.util.spec_from_file_location(
            "gdrive", self.services_path / 'gdrive.py'
        )
        self.gdrive_module = importlib.util.module_from_spec(gdrive_spec)
        gdrive_spec.loader.exec_module(self.gdrive_module)
        
        # Load WeTransfer service
        wetransfer_spec = importlib.util.spec_from_file_location(
            "wetransfer", self.services_path / 'wetransfer.py'
        )
        self.wetransfer_module = importlib.util.module_from_spec(wetransfer_spec)
        wetransfer_spec.loader.exec_module(self.wetransfer_module)
    
    def test_dropbox_url_support(self):
        """Test Dropbox URL support detection"""
        service = self.dropbox_module.DropboxService()
        
        # Test supported URLs
        supported_urls = [
            "https://www.dropbox.com/s/abc123/file.zip?dl=0",
            "https://dropbox.com/scl/fi/def456/file.pdf"
        ]
        
        for url in supported_urls:
            with self.subTest(url=url):
                self.assertTrue(service.is_supported_url(url))
        
        # Test unsupported URLs
        unsupported_urls = [
            "https://drive.google.com/file/d/abc123/view",
            "https://wetransfer.com/downloads/abc123"
        ]
        
        for url in unsupported_urls:
            with self.subTest(url=url):
                self.assertFalse(service.is_supported_url(url))
    
    def test_gdrive_url_support(self):
        """Test Google Drive URL support detection"""
        service = self.gdrive_module.GoogleDriveService()
        
        # Test supported URLs
        supported_urls = [
            "https://drive.google.com/file/d/abc123/view",
            "https://docs.google.com/document/d/xyz789/edit"
        ]
        
        for url in supported_urls:
            with self.subTest(url=url):
                self.assertTrue(service.is_supported_url(url))
        
        # Test unsupported URLs
        unsupported_urls = [
            "https://www.dropbox.com/s/abc123/file.zip",
            "https://wetransfer.com/downloads/abc123"
        ]
        
        for url in unsupported_urls:
            with self.subTest(url=url):
                self.assertFalse(service.is_supported_url(url))
    
    def test_wetransfer_url_support(self):
        """Test WeTransfer URL support detection"""
        service = self.wetransfer_module.WeTransferService()
        
        # Test supported URLs
        supported_urls = [
            "https://wetransfer.com/downloads/abc123",
            "https://we.tl/def456"
        ]
        
        for url in supported_urls:
            with self.subTest(url=url):
                self.assertTrue(service.is_supported_url(url))
        
        # Test unsupported URLs
        unsupported_urls = [
            "https://www.dropbox.com/s/abc123/file.zip",
            "https://drive.google.com/file/d/abc123/view"
        ]
        
        for url in unsupported_urls:
            with self.subTest(url=url):
                self.assertFalse(service.is_supported_url(url))


class TestFilenameExtraction(unittest.TestCase):
    """Test filename extraction from URLs and headers"""
    
    def setUp(self):
        """Set up test environment"""
        import importlib.util
        
        # Load service modules
        self.services_path = Path(__file__).parent / 'services'
        
        # Load Dropbox service
        dropbox_spec = importlib.util.spec_from_file_location(
            "dropbox", self.services_path / 'dropbox.py'
        )
        self.dropbox_module = importlib.util.module_from_spec(dropbox_spec)
        dropbox_spec.loader.exec_module(self.dropbox_module)
    
    def test_dropbox_filename_extraction(self):
        """Test Dropbox filename extraction"""
        service = self.dropbox_module.DropboxService()
        
        # Test URL-based extraction
        test_cases = [
            ("https://www.dropbox.com/s/abc123/document.pdf", "document.pdf"),
            ("https://dropbox.com/scl/fi/def456/image.jpg", "image.jpg")
        ]
        
        for url, expected_filename in test_cases:
            with self.subTest(url=url):
                filename = service.extract_filename(url)
                self.assertEqual(filename, expected_filename)
        
        # Test header-based extraction
        headers = {
            'content-disposition': 'attachment; filename="test-file.zip"'
        }
        filename = service.extract_filename("https://example.com", headers)
        self.assertEqual(filename, "test-file.zip")


def run_tests():
    """Run all tests"""
    # Create test suite
    test_suite = unittest.TestSuite()
    
    # Add test classes
    test_classes = [
        TestServiceDetection,
        TestServiceCreation, 
        TestServiceFunctionality,
        TestFilenameExtraction
    ]
    
    for test_class in test_classes:
        tests = unittest.TestLoader().loadTestsFromTestCase(test_class)
        test_suite.addTests(tests)
    
    # Run tests
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(test_suite)
    
    return result.wasSuccessful()


if __name__ == "__main__":
    print("🧪 Running Multi-Service Downloader Integration Tests")
    print("=" * 60)
    
    success = run_tests()
    
    if success:
        print("\n✅ All tests passed!")
        sys.exit(0)
    else:
        print("\n❌ Some tests failed!")
        sys.exit(1)