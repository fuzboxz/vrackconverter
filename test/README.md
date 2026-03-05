# End-to-End Tests

This directory contains regression tests using real-world patch files to ensure the converter works correctly.

## Test Files

- `legacy-patch.vcv` - Simple v0.6 patch for basic testing
- `realistic-v06-patch.vcv` - More complex v0.6 patch with multiple modules
- `mirackoutput.mrk` - MiRack bundle (directory with patch.vcv inside)
- `morningstarling.vcv` - Complex v0.6 patch with 58 modules for regression testing
  - Author: agnetha (https://patchstorage.com/author/agnetha/)
  - Source: https://patchstorage.com/starling/
  - License: MIT

## Running Tests

```bash
# Run only e2e tests
go test -v ./test/...

# Run all tests including e2e
go test -v ./...
```

## Test Categories

1. **TestE2E_RealWorldPatches** - Converts each test patch and verifies:
   - Input is valid v0.6 format
   - Output is valid v2.6 format
   - "wires" key is converted to "cables"
   - Output is valid zstd-compressed tar archive

2. **TestE2E_ConversionIdempotency** - Ensures converting the same input twice produces identical JSON output

3. **TestE2E_MrkBundleStructure** - Verifies .mrk bundle has required files and valid content

4. **TestE2E_AllTestFilesAreValid** - Validates all test files in this directory can be parsed

5. **TestE2E_MorningstarlingRegression** - Tests conversion of the morningstarling patch
6. **TestE2E_SkipV2Format** - Verifies v2 format files are detected and skipped

## Adding New Test Cases

To add a regression test for a new patch:

1. Add your test file (`.vcv` or `.mrk`) to this directory
2. Add an entry to the `tests` slice in `TestE2E_RealWorldPatches`
3. Run the tests to verify the patch converts correctly
