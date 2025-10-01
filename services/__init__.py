"""Multi-service downloader services package"""

from .base import BaseDownloadService
from .dropbox import DropboxService
from .gdrive import GoogleDriveService  
from .wetransfer import WeTransferService

__all__ = ['BaseDownloadService', 'DropboxService', 'GoogleDriveService', 'WeTransferService']