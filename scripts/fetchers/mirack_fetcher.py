"""MiRack module fetcher.

MiRack modules are defined in the GitHub repository:
https://github.com/accursoft/mercury-modules
"""

import logging
from typing import List, Dict, Any
import requests

from .base import BaseFetcher

logger = logging.getLogger(__name__)


class MiRackFetcher(BaseFetcher):
    """Fetch modules from MiRack repository."""

    # MiRack modules JSON URL
    MIRACK_MODULES_URL = "https://raw.githubusercontent.com/accursoft/mercury-modules/master/modules.json"

    def __init__(self, timeout: int = 30):
        """Initialize fetcher.

        Args:
            timeout: Request timeout in seconds
        """
        self.timeout = timeout

    def get_platform_name(self) -> str:
        return "mirack"

    def fetch_modules(self) -> List[Dict[str, Any]]:
        """Fetch all modules from MiRack repository."""
        modules = []
        try:
            response = requests.get(self.MIRACK_MODULES_URL, timeout=self.timeout)
            response.raise_for_status()
            data = response.json()

            for plugin_name, plugin_data in data.items():
                for module in plugin_data.get("modules", []):
                    modules.append({
                        "name": module.get("name", ""),
                        "plugin": plugin_name,
                        "model": module.get("model", module.get("name", "")),
                        "platform": self.get_platform_name(),
                        "tags": module.get("tags", []),
                        "url": f"https://github.com/accursoft/mercury-modules",
                        "sources": [self.get_platform_name()],
                    })

            logger.info(f"Fetched {len(modules)} modules from MiRack")

        except requests.RequestException as e:
            logger.error(f"Failed to fetch MiRack modules: {e}")

        return modules
