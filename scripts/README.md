# Module Registry

Scripts for fetching and maintaining a cross-platform module registry for RackConverter.

## Overview

The module registry collects data from multiple VCV Rack-compatible platforms:

- **VCV Library** - Official VCV Rack plugin repository
- **MiRack** - iOS VCV Rack implementation
- **Cardinal** - Open-source DAW plugin version of VCV Rack
- **MetaModule** - Hardware VCV Rack-compatible device by 4ms

## Files

```
scripts/
├── fetchers/
│   ├── __init__.py
│   ├── base.py              # Base fetcher class
│   ├── vcv_fetcher.py       # VCV Library API
│   ├── mirack_fetcher.py    # MiRack modules
│   ├── cardinal_fetcher.py  # Cardinal bundled plugins
│   └── metamodule_fetcher.py # MetaModule marketplace
├── alias-mappings.yaml      # Cross-platform module aliases
├── deduplicate.py           # Deduplication logic
├── fetch_registry.py        # Main entry point
└── README.md                # This file
```

## Usage

### Fetch and update registry

```bash
cd scripts
python fetch_registry.py
```

This will:
1. Fetch modules from all sources
2. Save raw data to `data/modules_raw.json`
3. Deduplicate and save to `data/modules.json`

### Run deduplication only

```bash
cd scripts
python deduplicate.py data/modules_raw.json data/modules.json
```

## GitHub Actions

The registry can be updated via GitHub Actions manually:

1. Go to **Actions** → **Update Module Registry**
2. Click **Run workflow**
3. Click **Run workflow** to confirm

The workflow will:
- Fetch all module data
- Deduplicate with aliases
- Commit changes to `data/modules.json` and `data/modules_raw.json`

## Module Data Format

Each module entry has:

```json
{
  "name": "VCO-1",
  "plugin": "Fundamental",
  "model": "VCO",
  "platform": "vcv",
  "tags": ["Oscillator", "VCO"],
  "url": "https://library.vcv.dk/plugins/fundamental",
  "sources": ["vcv", "cardinal"]
}
```

### Fields

| Field | Description |
|-------|-------------|
| `name` | Display name of the module |
| `plugin` | Plugin name |
| `model` | Model identifier (slug) |
| `platform` | Primary platform (vcv, mirack, cardinal, metamodule) |
| `tags` | Category tags |
| `url` | Source URL |
| `sources` | List of platforms where this module exists |
| `aliases` | Optional: Alternative names/mappings |

## Adding a New Fetcher

1. Create a new fetcher class in `scripts/fetchers/`:

```python
from .base import BaseFetcher
from typing import List, Dict, Any

class NewPlatformFetcher(BaseFetcher):
    def get_platform_name(self) -> str:
        return "newplatform"

    def fetch_modules(self) -> List[Dict[str, Any]]:
        # Fetch and return modules
        return [{"name": "...", "plugin": "...", ...}]
```

2. Add to `scripts/fetchers/__init__.py`:

```python
from .newplatform_fetcher import NewPlatformFetcher

__all__ = [..., "NewPlatformFetcher"]
```

3. Import and use in `scripts/fetch_registry.py`:

```python
from fetchers import NewPlatformFetcher

fetchers = [
    ...,
    NewPlatformFetcher(),
]
```

## Alias Mappings

Edit `scripts/alias-mappings.yaml` to add cross-platform aliases:

```yaml
aliases:
  # Simple name alias
  "Plugin/Model": "Alternative Name"

  # Full mapping
  "Plugin/Model":
    name: "Display Name"
    plugin: "OtherPlugin"
    model: "OtherModel"
```

## Development

### Install dependencies

```bash
pip install requests pyyaml
```

### Run with debug logging

```bash
python -c "import logging; logging.basicConfig(level=logging.DEBUG)"
python fetch_registry.py
```
