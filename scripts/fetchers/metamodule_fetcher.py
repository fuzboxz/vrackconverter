"""MetaModule module fetcher.

MetaModule is a hardware VCV Rack-compatible device with its own
module marketplace.
"""

import logging
from typing import List, Dict, Any
import requests
import re

from .base import BaseFetcher

logger = logging.getLogger(__name__)


class MetaModuleFetcher(BaseFetcher):
    """Fetch modules from MetaModule repository."""

    # MetaModule module list
    METAMODULE_API_URL = "https://4ms.company/metamodule/api/modules"

    def __init__(self, timeout: int = 30):
        """Initialize fetcher.

        Args:
            timeout: Request timeout in seconds
        """
        self.timeout = timeout

    def get_platform_name(self) -> str:
        return "metamodule"

    def fetch_modules(self) -> List[Dict[str, Any]]:
        """Fetch all MetaModule modules."""
        modules = []
        try:
            response = requests.get(self.METAMODULE_API_URL, timeout=self.timeout)
            response.raise_for_status()
            data = response.json()

            for plugin in data.get("plugins", []):
                plugin_name = plugin.get("plugin", "")

                for module in plugin.get("modules", []):
                    modules.append({
                        "name": module.get("name", ""),
                        "plugin": plugin_name,
                        "model": module.get("model", module.get("name", "")),
                        "platform": self.get_platform_name(),
                        "tags": self._extract_tags(module),
                        "url": "https://4ms.company/metamodule/",
                        "sources": [self.get_platform_name()],
                    })

            logger.info(f"Fetched {len(modules)} modules from MetaModule")

        except requests.RequestException as e:
            logger.error(f"Failed to fetch MetaModule modules: {e}")

        return modules

    def _extract_tags(self, module: Dict[str, Any]) -> List[str]:
        """Extract tags from MetaModule module data."""
        tags = []

        # Add category as tag
        if "category" in module:
            tags.append(module["category"])

        return list(set(tags))
