"""Module deduplication with hybrid merge + source tracking.

This module implements a single deduplication strategy that:
1. Merges duplicate entries (same plugin/model)
2. Tracks all sources/platforms for each module
3. Applies alias mappings from alias-mappings.yaml
"""

import logging
from typing import List, Dict, Any, Optional
import yaml

logger = logging.getLogger(__name__)


def load_alias_mappings(path: str = "scripts/alias-mappings.yaml") -> Dict[str, Dict[str, str]]:
    """Load alias mappings from YAML file.

    Args:
        path: Path to alias-mappings.yaml file

    Returns:
        Dict mapping (plugin, model) to aliases
    """
    try:
        with open(path, "r") as f:
            data = yaml.safe_load(f) or {}
            return data.get("aliases", {})
    except FileNotFoundError:
        logger.warning(f"Alias mappings file not found: {path}")
        return {}
    except yaml.YAMLError as e:
        logger.error(f"Error parsing alias mappings: {e}")
        return {}


def _get_module_key(module: Dict[str, Any]) -> tuple:
    """Get unique key for module deduplication.

    Uses (plugin, model) tuple as the unique identifier.
    """
    return (module.get("plugin", "").lower(), module.get("model", "").lower())


def _merge_modules(existing: Dict[str, Any], new: Dict[str, Any]) -> Dict[str, Any]:
    """Merge two module entries, combining data.

    Merge strategy:
    - Keep the name from the first (most commonly VCV)
    - Merge sources (deduplicated)
    - Merge tags (deduplicated)
    - Keep non-empty URL from either
    - Keep all platforms in sources list
    """
    merged = existing.copy()

    # Merge sources
    existing_sources = set(existing.get("sources", []))
    new_sources = set(new.get("sources", []))
    merged["sources"] = list(existing_sources | new_sources)

    # Merge tags
    existing_tags = set(existing.get("tags", []))
    new_tags = set(new.get("tags", []))
    merged["tags"] = list(existing_tags | new_tags)

    # Keep URL if we don't have one
    if not merged.get("url") and new.get("url"):
        merged["url"] = new["url"]

    return merged


def _apply_aliases(modules: List[Dict[str, Any]], alias_map: Dict[str, Dict[str, str]]) -> List[Dict[str, Any]]:
    """Apply alias mappings to modules.

    For each module, check if there's an alias entry and:
    1. Add the alias as an alternative name
    2. Merge data from aliased modules if they exist in the list
    """
    if not alias_map:
        return modules

    # Create a lookup by (plugin, model)
    module_lookup = {_get_module_key(m): m for m in modules}
    result = []

    for module in modules:
        key = _get_module_key(module)
        plugin, model = key

        # Check if this module has aliases defined
        alias_key = f"{plugin}/{model}"
        if alias_key in alias_map:
            aliases = alias_map[alias_key]
            if isinstance(aliases, str):
                aliases = {"name": aliases}
            elif not isinstance(aliases, dict):
                aliases = {}

            if aliases:
                module = module.copy()
                module["aliases"] = aliases

        result.append(module)

    return result


def deduplicate(modules: List[Dict[str, Any]], alias_map: Optional[Dict[str, Dict[str, str]]] = None) -> List[Dict[str, Any]]:
    """Deduplicate modules using hybrid merge + track strategy.

    This combines the best of "merge" and "track" strategies:
    - Merges data from duplicate entries
    - Tracks all sources and platforms
    - Applies alias mappings

    Args:
        modules: List of module dictionaries from all fetchers
        alias_map: Optional alias mappings dictionary

    Returns:
        Deduplicated list of modules
    """
    if alias_map is None:
        alias_map = {}

    # Group by (plugin, model) key
    groups: Dict[tuple, List[Dict[str, Any]]] = {}

    for module in modules:
        key = _get_module_key(module)
        if key not in groups:
            groups[key] = []
        groups[key].append(module)

    logger.info(f"Deduplicating {len(modules)} modules into {len(groups)} unique entries")

    # Merge each group
    result = []
    for key, group in groups.items():
        if len(group) == 1:
            merged = group[0]
        else:
            # Start with the first module and merge others
            merged = group[0]
            for module in group[1:]:
                merged = _merge_modules(merged, module)

            # Log multi-source modules
            sources = merged.get("sources", [])
            if len(sources) > 1:
                logger.debug(f"Merged {key} from sources: {sources}")

        result.append(merged)

    # Apply aliases
    result = _apply_aliases(result, alias_map)

    return result


def main():
    """CLI entry point for deduplication."""
    import json
    import sys

    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")

    if len(sys.argv) < 2:
        print("Usage: python deduplicate.py <input.json> [output.json]")
        print("  input.json  - Input file with modules array")
        print("  output.json - Optional output file (defaults to stdout)")
        sys.exit(1)

    input_file = sys.argv[1]
    output_file = sys.argv[2] if len(sys.argv) > 2 else None

    # Load input
    with open(input_file, "r") as f:
        data = json.load(f)
        modules = data.get("modules", [])

    # Load aliases
    alias_map = load_alias_mappings()

    # Deduplicate
    deduped = deduplicate(modules, alias_map)

    output = {"modules": deduped}

    if output_file:
        with open(output_file, "w") as f:
            json.dump(output, f, indent=2)
        print(f"Wrote {len(deduped)} deduplicated modules to {output_file}")
    else:
        print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
