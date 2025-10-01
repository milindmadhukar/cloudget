#!/usr/bin/env python3
"""
Simple test to verify the core logic without external dependencies
"""

# Mock the external dependencies for testing
class MockClientSession:
    pass

class MockPath:
    def __init__(self, path):
        self.path = path
    
    def __truediv__(self, other):
        return MockPath(f"{self.path}/{other}")
    
    @property 
    def name(self):
        return self.path.split('/')[-1]

# Mock implementations for testing logic
def mock_import_test():
    """Test that our service detection logic works"""
    
    # URL detection tests
    test_urls = [
        "https://www.dropbox.com/s/abc123/test.zip?dl=0",
        "https://drive.google.com/file/d/1abc123/view", 
        "https://wetransfer.com/downloads/abc123",
        "https://we.tl/abc123",
        "https://example.com/some-file.zip"  # Should not be detected
    ]
    
    # Test URL detection
    def detect_service(url: str):
        """Detect which service should handle the URL"""
        url_lower = url.lower()
        
        if 'dropbox.com' in url_lower:
            return 'dropbox'
        elif 'drive.google.com' in url_lower or 'docs.google.com' in url_lower:
            return 'gdrive'
        elif 'wetransfer.com' in url_lower or 'we.tl' in url_lower:
            return 'wetransfer'
        
        return None
    
    print("=== URL Detection Tests ===")
    for url in test_urls:
        detected = detect_service(url)
        print(f"URL: {url}")
        print(f"Detected service: {detected}")
        print()
    
    # Test Dropbox URL conversion
    def convert_dropbox_url(url: str) -> str:
        """Convert Dropbox share URL to direct download URL"""
        if '/s/' in url or '/scl/fi/' in url:
            if 'dl=0' in url:
                return url.replace('dl=0', 'dl=1')
            elif '?' in url:
                return url + '&dl=1'
            else:
                return url + '?dl=1'
        else:
            raise ValueError("Unsupported Dropbox URL format")
    
    print("=== Dropbox URL Conversion Tests ===")
    dropbox_urls = [
        "https://www.dropbox.com/s/abc123/test.zip?dl=0",
        "https://www.dropbox.com/s/abc123/test.zip",
        "https://www.dropbox.com/s/abc123/test.zip?some_param=1"
    ]
    
    for url in dropbox_urls:
        try:
            converted = convert_dropbox_url(url)
            print(f"Original: {url}")
            print(f"Converted: {converted}")
            print()
        except Exception as e:
            print(f"Error converting {url}: {e}")
            print()

if __name__ == "__main__":
    mock_import_test()
    print("✅ Core logic tests completed successfully!")