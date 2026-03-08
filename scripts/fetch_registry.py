"""Main script to fetch module data from all sources.

This script:
1. Fetches modules from VCV Library, MiRack, Cardinal, MetaModule
2. Saves raw modules to data/modules_raw.json
3. Deduplicates and saves to data/modules.json
"""

import json
import logging
import os
from typing import List, Dict, Any

from fetchers import (
    VCVLibraryFetcher,
    MiRackFetcher,
    CardinalFetcher,
    MetaModuleFetcher,
)
from deduplicate import deduplicate, load_alias_mappings

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


def save_json(data: Dict[str, Any], path: str) -> None:
    """Save data to JSON file.

    Args:
        data: Dictionary to save
        path: Output file path
    """
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(data, f, indent=2)


def main():
    """Fetch and process module data from all sources."""
    # Initialize fetchers
    fetchers = [
        VCVLibraryFetcher(),
        MiRackFetcher(),
        CardinalFetcher(),
        MetaModuleFetcher(),
    ]

    # Fetch all modules
    all_modules: List[Dict[str, Any]] = []
    for fetcher in fetchers:
        logger.info(f"Fetching from {fetcher.get_platform_name()}...")
        try:
            modules = fetcher.fetch_modules()
            all_modules.extend(modules)
        except Exception as e:
            logger.error(f"Error fetching from {fetcher.get_platform_name()}: {e}")

    logger.info(f"Total modules fetched: {len(all_modules)}")

    # Save raw output
    raw_path = "data/modules_raw.json"
    save_json({"modules": all_modules}, raw_path)
    logger.info(f"Saved raw modules to {raw_path}")

    # Load aliases and deduplicate
    alias_map = load_alias_mappings()
    deduped = deduplicate(all_modules, alias_map)

    # Save deduplicated output
    deduped_path = "data/modules.json"
    save_json({"modules": deduped}, deduped_path)
    logger.info(f"Saved {len(deduped)} deduplicated modules to {deduped_path}")

    # Print summary
    print(f"\nSummary:")
    print(f"  Total raw modules: {len(all_modules)}")
    print(f"  Deduplicated modules: {len(deduped)}")
    print(f"  Reduction: {len(all_modules) - len(deduped)} ({100 * (len(all_modules) - len(deduped)) / len(all_modules):.1f}%)")


if __name__ == "__main__":
    main()
