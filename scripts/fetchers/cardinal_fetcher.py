"""Cardinal module fetcher.

Cardinal bundles many open-source VCV plugins. Module info is from
the Cardinal repository manifest.
"""

import logging
from typing import List, Dict, Any
import requests

from .base import BaseFetcher

logger = logging.getLogger(__name__)


class CardinalFetcher(BaseFetcher):
    """Fetch modules from Cardinal bundled plugins."""

    # Cardinal plugin manifest
    CARDINAL_PLUGINS_URL = "https://raw.githubusercontent.com/DISTRHO/Cardinal/master/manifest.json"

    def __init__(self, timeout: int = 30):
        """Initialize fetcher.

        Args:
            timeout: Request timeout in seconds
        """
        self.timeout = timeout

    def get_platform_name(self) -> str:
        return "cardinal"

    def fetch_modules(self) -> List[Dict[str, Any]]:
        """Fetch all bundled Cardinal modules."""
        modules = []
        try:
            response = requests.get(self.CARDINAL_PLUGINS_URL, timeout=self.timeout)
            response.raise_for_status()
            data = response.json()

            for plugin in data.get("plugins", []):
                plugin_name = plugin.get("name", "")

                for module in plugin.get("modules", []):
                    modules.append({
                        "name": module.get("name", ""),
                        "plugin": plugin_name,
                        "model": module.get("model", module.get("name", "")),
                        "platform": self.get_platform_name(),
                        "tags": module.get("tags", []),
                        "url": "https://github.com/DISTRHO/Cardinal",
                        "sources": [self.get_platform_name()],
                    })

            logger.info(f"Fetched {len(modules)} modules from Cardinal")

        except requests.RequestException as e:
            logger.error(f"Failed to fetch Cardinal modules: {e}")

        return modules
