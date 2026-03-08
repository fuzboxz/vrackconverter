"""Base fetcher class for module registry."""

from abc import ABC, abstractmethod
from typing import List, Dict, Any


class BaseFetcher(ABC):
    """Abstract base class for module fetchers."""

    @abstractmethod
    def fetch_modules(self) -> List[Dict[str, Any]]:
        """
        Fetch modules from the source.

        Returns:
            List of module dictionaries with common fields:
                - name: Module/display name
                - plugin: Plugin name
                - model: Model identifier
                - platform: Source platform (vcv, mirack, cardinal, metamodule)
                - tags: List of category tags
                - url: Source URL (optional)
        """
        pass

    @abstractmethod
    def get_platform_name(self) -> str:
        """Return the platform identifier."""
        pass

    def _normalize_module(self, module: Dict[str, Any]) -> Dict[str, Any]:
        """Normalize module data to common schema."""
        return {
            "name": module.get("name", ""),
            "plugin": module.get("plugin", ""),
            "model": module.get("model", ""),
            "platform": module.get("platform", self.get_platform_name()),
            "tags": module.get("tags", []),
            "url": module.get("url", ""),
            "sources": module.get("sources", [self.get_platform_name()]),
        }
