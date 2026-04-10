/**
 * archive.js - Port of Go archive.go
 *
 * Zstd tar archive handling for VCV Rack patch files.
 * Uses fzstd library (window.fzstd) for zstd decompression.
 * All functions attached to window.VRackConverter namespace.
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
  'use strict';

  var TAR_HEADER_SIZE = 512;
  var TAR_NAME_OFFSET = 0;
  var TAR_NAME_LENGTH = 100;
  var TAR_SIZE_OFFSET = 124;
  var TAR_SIZE_LENGTH = 12;
  var TAR_CHECKSUM_OFFSET = 148;
  var TAR_CHECKSUM_LENGTH = 8;

  /**
   * extractVersion attempts to extract the version field from patch data.
   * Handles both plain JSON and zstd-compressed tar archives.
   * Port of Go extractVersion().
   *
   * @param {Uint8Array} data - Raw file data
   * @returns {string|null} Version string, or null if not found
   */
  NS.extractVersion = function(data) {
    // First, try to parse as plain JSON
    try {
      var decoder = new TextDecoder('utf-8');
      var str = decoder.decode(data);
      var root = JSON.parse(str);
      if (root && typeof root.version === 'string') {
        return root.version;
      }
      return null;
    } catch (e) {
      // Not plain JSON, fall through to zstd tar
    }

    // Try as zstd-compressed tar archive
    if (!window.fzstd) {
      return null;
    }

    try {
      var decompressed = window.fzstd.decompress(data);
      var jsonBytes = extractTarEntry(decompressed, 'patch.json');
      if (!jsonBytes) {
        return null;
      }

      var dec = new TextDecoder('utf-8');
      var jsonStr = dec.decode(jsonBytes);
      var patchObj = JSON.parse(jsonStr);
      if (patchObj && typeof patchObj.version === 'string') {
        return patchObj.version;
      }
    } catch (e) {
      // Failed to decompress or parse
    }

    return null;
  };

  /**
   * extractJSONFromV2 extracts patch.json from a VCV Rack v2 format file.
   * Decompresses zstd, then extracts patch.json from the tar archive.
   * Port of Go ExtractJSONFromV2().
   *
   * @param {Uint8Array} data - Raw .vcv file data (zstd-compressed tar)
   * @returns {Uint8Array} Raw JSON bytes of patch.json
   * @throws {Error} If decompression fails or patch.json not found
   */
  NS.extractJSONFromV2 = function(data) {
    if (!window.fzstd) {
      throw new Error('fzstd library not loaded');
    }

    var decompressed = window.fzstd.decompress(data);
    var jsonBytes = extractTarEntry(decompressed, 'patch.json');
    if (!jsonBytes) {
      throw new Error('patch.json not found in archive');
    }
    return jsonBytes;
  };

  /**
   * createV2Patch creates a VCV Rack v2 format file from JSON bytes.
   * Creates a tar archive containing patch.json, then compresses with zstd.
   * Port of Go CreateV2Patch().
   *
   * @param {Uint8Array} jsonBytes - Serialized patch JSON
   * @returns {Uint8Array} Zstd-compressed tar archive
   * @throws {Error} If zstd compression is not available
   */
  NS.createV2Patch = function(jsonBytes) {
    // Build tar buffer containing patch.json
    var tarBuffer = createTarBuffer('patch.json', jsonBytes);

    // Compress with zstd
    if (window.fzstd && typeof window.fzstd.compress === 'function') {
      return window.fzstd.compress(tarBuffer);
    }

    throw new Error(
      'Zstd compression not available. The fzstd library loaded provides decompression only. ' +
      'To enable .vcv file creation, load a library that supports zstd compression (e.g., fflate + a zstd compress implementation).'
    );
  };

  /**
   * isV2Format checks if data represents a VCV Rack v2 format patch.
   * Detects by checking the version field: v2 has "2.x.x", v0.6 has "0.x.x".
   * Port of Go IsV2Format().
   *
   * @param {Uint8Array} data - Raw file data
   * @returns {boolean} True if the data is VCV Rack v2 format
   */
  NS.isV2Format = function(data) {
    var version = NS.extractVersion(data);
    if (!version) {
      return false;
    }
    return version.indexOf('2.') === 0;
  };

  // ---- Internal tar helpers ----

  /**
   * extractTarEntry extracts a named entry from tar data.
   * Handles both "patch.json" and "./patch.json" name matching.
   *
   * @param {Uint8Array} tarData - Decompressed tar data
   * @param {string} entryName - Name of the entry to extract
   * @returns {Uint8Array|null} Entry content bytes, or null if not found
   */
  function extractTarEntry(tarData, entryName) {
    var offset = 0;
    var prefixedName = './' + entryName;

    while (offset + TAR_HEADER_SIZE <= tarData.length) {
      // Read entry name (null-terminated string)
      var nameBytes = tarData.subarray(TAR_NAME_OFFSET + offset, TAR_NAME_OFFSET + offset + TAR_NAME_LENGTH);
      var name = readTarString(nameBytes);

      // Read entry size (octal string)
      var sizeBytes = tarData.subarray(TAR_SIZE_OFFSET + offset, TAR_SIZE_OFFSET + offset + TAR_SIZE_LENGTH);
      var size = readTarOctal(sizeBytes);

      // Check for end-of-archive marker (two zero blocks)
      if (isZeroBlock(tarData, offset)) {
        break;
      }

      // Data starts after the header
      var dataOffset = offset + TAR_HEADER_SIZE;

      if (name === entryName || name === prefixedName) {
        return tarData.slice(dataOffset, dataOffset + size);
      }

      // Advance to next entry: header + data rounded up to 512-byte boundary
      var dataBlocks = Math.ceil(size / TAR_HEADER_SIZE);
      offset = dataOffset + (dataBlocks * TAR_HEADER_SIZE);
    }

    return null;
  }

  /**
   * createTarBuffer builds a tar archive containing a single file.
   * Includes: file header + content + padding + two 512-byte zero end blocks.
   *
   * @param {string} filename - Entry filename
   * @param {Uint8Array} content - Entry content
   * @returns {Uint8Array} Complete tar archive
   */
  function createTarBuffer(filename, content) {
    var contentSize = content.length;
    var paddedContentSize = Math.ceil(contentSize / TAR_HEADER_SIZE) * TAR_HEADER_SIZE;
    // Header + padded content + two zero end blocks
    var totalSize = TAR_HEADER_SIZE + paddedContentSize + (2 * TAR_HEADER_SIZE);
    var buffer = new Uint8Array(totalSize);

    // Build header
    var header = new Uint8Array(TAR_HEADER_SIZE);

    // Name field (offset 0, 100 bytes)
    writeTarString(header, TAR_NAME_OFFSET, TAR_NAME_LENGTH, filename);

    // Mode field (offset 100, 8 bytes) - "0000644\0"
    writeTarString(header, 100, 8, '0000644\0');

    // UID field (offset 108, 8 bytes) - "0000000\0"
    writeTarString(header, 108, 8, '0000000\0');

    // GID field (offset 116, 8 bytes) - "0000000\0"
    writeTarString(header, 116, 8, '0000000\0');

    // Size field (offset 124, 12 bytes) - octal size string
    var sizeOctal = contentSize.toString(8);
    // Pad to 11 chars, then null-terminate
    while (sizeOctal.length < 11) {
      sizeOctal = '0' + sizeOctal;
    }
    sizeOctal = sizeOctal + '\0';
    writeTarString(header, TAR_SIZE_OFFSET, TAR_SIZE_LENGTH, sizeOctal);

    // Mtime field (offset 136, 12 bytes) - "00000000000\0"
    writeTarString(header, 136, 12, '00000000000\0');

    // Checksum field (offset 148, 8 bytes) - initially spaces for calculation
    writeTarString(header, TAR_CHECKSUM_OFFSET, TAR_CHECKSUM_LENGTH, '        ');

    // Type flag (offset 156, 1 byte) - '0' for regular file
    header[156] = 0x30; // '0'

    // USTAR magic (offset 257, 6 bytes) - "ustar\0"
    writeTarString(header, 257, 6, 'ustar\0');

    // USTAR version (offset 263, 2 bytes) - "00"
    writeTarString(header, 263, 2, '00');

    // Calculate checksum: sum of all unsigned bytes with checksum field as spaces (already set)
    var checksum = 0;
    for (var i = 0; i < TAR_HEADER_SIZE; i++) {
      checksum += header[i];
    }

    // Write checksum as 6-digit octal + null + space
    var checksumStr = checksum.toString(8);
    while (checksumStr.length < 6) {
      checksumStr = '0' + checksumStr;
    }
    checksumStr = checksumStr + '\0 ';
    writeTarString(header, TAR_CHECKSUM_OFFSET, TAR_CHECKSUM_LENGTH, checksumStr);

    // Copy header into buffer
    buffer.set(header, 0);

    // Copy content into buffer after header
    buffer.set(content, TAR_HEADER_SIZE);

    // Remaining bytes after content are already zero (zero-padded content + end blocks)
    return buffer;
  }

  /**
   * readTarString reads a null-terminated string from a Uint8Array subset.
   *
   * @param {Uint8Array} bytes - Bytes to read from
   * @returns {string} Decoded string (without null terminator)
   */
  function readTarString(bytes) {
    var end = bytes.length;
    for (var i = 0; i < bytes.length; i++) {
      if (bytes[i] === 0) {
        end = i;
        break;
      }
    }
    var decoder = new TextDecoder('utf-8');
    return decoder.decode(bytes.subarray(0, end));
  }

  /**
   * readTarOctal reads an octal string from tar header bytes.
   *
   * @param {Uint8Array} bytes - Bytes containing octal string
   * @returns {number} Parsed integer value
   */
  function readTarOctal(bytes) {
    var str = readTarString(bytes).trim();
    if (str.length === 0) {
      return 0;
    }
    return parseInt(str, 8) || 0;
  }

  /**
   * writeTarString writes an ASCII string into a Uint8Array at the given offset.
   *
   * @param {Uint8Array} buffer - Target buffer
   * @param {number} offset - Write offset
   * @param {number} maxLength - Maximum field length
   * @param {string} str - String to write
   */
  function writeTarString(buffer, offset, maxLength, str) {
    for (var i = 0; i < maxLength && i < str.length; i++) {
      buffer[offset + i] = str.charCodeAt(i);
    }
  }

  /**
   * isZeroBlock checks if a 512-byte block at the given offset is all zeros.
   * Used to detect end-of-archive markers.
   *
   * @param {Uint8Array} data - Tar data
   * @param {number} offset - Block offset
   * @returns {boolean} True if the block is all zeros
   */
  function isZeroBlock(data, offset) {
    for (var i = 0; i < TAR_HEADER_SIZE; i++) {
      if (data[offset + i] !== 0) {
        return false;
      }
    }
    return true;
  }

  // Expose internal tar helpers for testing
  NS._extractTarEntry = extractTarEntry;
  NS._createTarBuffer = createTarBuffer;

  /**
   * createMiRackZip creates a .zip file containing an .mrk directory
   * structure with patch.vcv inside.
   *
   * Structure:
   *   basename.mrk/
   *     patch.vcv  (the patch JSON)
   *
   * @param {string} dirName - The .mrk directory name (e.g. "patch.mrk")
   * @param {Uint8Array} patchJSON - The serialized patch JSON bytes
   * @returns {Uint8Array} ZIP file bytes
   */
  NS.createMiRackZip = function(dirName, patchJSON) {
    var fflate = window.fflate;
    if (!fflate) throw new Error('Zip library not available');

    var files = {};
    files[dirName + '/patch.vcv'] = patchJSON;
    return fflate.zipSync(files);
  };

})(window.VRackConverter);
