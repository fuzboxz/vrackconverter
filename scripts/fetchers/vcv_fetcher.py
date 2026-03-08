"""VCV Library API fetcher for module registry.

VCV Library API: https://vcv.dk/api/plugins
Returns all plugins with their modules.
"""

import logging
from typing import List, Dict, Any
import requests

from .base import BaseFetcher

logger = logging.getLogger(__name__)


class VCVLibraryFetcher(BaseFetcher):
    """Fetch modules from VCV Library API."""

    VCV_API_URL = "https://vcv.dk/api/plugins"

    def __init__(self, timeout: int = 30):
        """Initialize fetcher.

        Args:
            timeout: Request timeout in seconds
        """
        self.timeout = timeout

    def get_platform_name(self) -> str:
        return "vcv"

    def fetch_modules(self) -> List[Dict[str, Any]]:
        """Fetch all modules from VCV Library."""
        modules = []
        try:
            response = requests.get(self.VCV_API_URL, timeout=self.timeout)
            response.raise_for_status()
            plugins = response.json()

            for plugin in plugins:
                plugin_name = plugin.get("name", "")
                plugin_slug = plugin.get("slug", "")

                for module in plugin.get("modules", []):
                    modules.append({
                        "name": module.get("name", ""),
                        "plugin": plugin_name,
                        "model": module.get("slug", ""),
                        "platform": self.get_platform_name(),
                        "tags": self._extract_tags(module),
                        "url": f"https://library.vcv.dk/plugins/{plugin_slug}",
                        "sources": [self.get_platform_name()],
                    })

            logger.info(f"Fetched {len(modules)} modules from VCV Library")

        except requests.RequestException as e:
            logger.error(f"Failed to fetch VCV Library modules: {e}")

        return modules

    def _extract_tags(self, module: Dict[str, Any]) -> List[str]:
        """Extract tags from VCV module data.

        VCV tags include categories like:
        - Oscillator, VCO, VCF, VCA, Envelope, LFO, etc.
        - Specific tags in module tagList
        """
        tags = []

        # Add from tagList
        for tag in module.get("tagList", []):
            tags.append(tag)

        return list(set(tags))
