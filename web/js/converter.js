/**
 * converter.js - Port of Go converter.go + metamodule.go
 *
 * Main conversion pipeline: detect format -> normalize -> denormalize -> write.
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
'use strict';

// Format constants
NS.FORMAT_V2 = 'v2';
NS.FORMAT_V06 = 'v0.6';
NS.FORMAT_MIRACK = 'mirack';
NS.FORMAT_CARDINAL = 'cardinal';
NS.FORMAT_UNKNOWN = '';

/**
 * Detect format from filename and file data.
 * @param {string} filename - Input filename
 * @param {Uint8Array} data - Raw file data
 * @returns {string} Format constant
 */
NS.detectFormat = function(filename, data) {
  var lower = (filename || '').toLowerCase();

  // MiRack: .mrk directory or patch.vcv inside .mrk
  if (lower.endsWith('.mrk') || lower.indexOf('.mrk/') !== -1 || lower.indexOf('.mrk\\') !== -1) {
    return NS.FORMAT_MIRACK;
  }

  var dotIdx = lower.lastIndexOf('.');
  var ext = dotIdx >= 0 ? lower.substring(dotIdx) : '';

  if (ext === '.vcv') {
    var version = NS.extractVersion(data);
    if (version) {
      if (version.indexOf('2.') === 0) {
        // Could be V2 or Cardinal - check content
        var patch = null;
        try {
          // Try plain JSON first
          var decoder = new TextDecoder('utf-8');
          var str = decoder.decode(data);
          patch = JSON.parse(str);
        } catch(e) {
          // Not plain JSON, try zstd tar
          try {
            var json = NS.extractJSONFromV2(data);
            patch = NS.fromJSON(json);
          } catch(e2) {}
        }
        if (patch && NS.detectCardinalByContent && NS.detectCardinalByContent(patch)) {
          return NS.FORMAT_CARDINAL;
        }
        return NS.FORMAT_V2;
      }
      if (version.indexOf('0.') === 0) {
        return NS.FORMAT_V06;
      }
    }
    return NS.FORMAT_V06;
  }

  return NS.FORMAT_UNKNOWN;
};

/**
 * Read patch data and extract JSON based on format.
 * @param {Uint8Array} data - Raw file data
 * @param {string} format - Detected format
 * @returns {Uint8Array} JSON bytes
 */
NS.readPatchData = function(data, format) {
  // Always try plain JSON first (handles both MiRack/V06 and plain-JSON V2 output)
  try {
    var decoder = new TextDecoder('utf-8');
    var str = decoder.decode(data);
    var parsed = JSON.parse(str);
    if (parsed && typeof parsed === 'object') {
      return data;
    }
  } catch(e) {}
  // Fall back to zstd tar extraction
  return NS.extractJSONFromV2(data);
};

/**
 * Convert a patch from one format to another.
 * Main pipeline function.
 *
 * @param {Uint8Array} inputData - Raw input file data
 * @param {string} inputFilename - Input filename (for format detection)
 * @param {string} targetFormat - Target format constant
 * @param {Object} options - Conversion options
 * @param {boolean} [options.metamodule] - Add MetaModule module
 * @param {string} [options.sourceFormat] - Override source format detection
 * @returns {Object} Result
 */
NS.convertPatch = function(inputData, inputFilename, targetFormat, options) {
  options = options || {};
  var issues = [];
  var result = {
    success: false,
    skipped: false,
    error: null,
    issues: issues,
    outputData: null,
    outputFilename: null,
    sourceFormat: null,
    targetFormat: targetFormat
  };

  // Detect source format
  var sourceFormat = options.sourceFormat || NS.detectFormat(inputFilename, inputData);
  if (!sourceFormat || sourceFormat === NS.FORMAT_UNKNOWN) {
    result.error = 'Unable to detect input format for: ' + inputFilename;
    return result;
  }
  result.sourceFormat = sourceFormat;

  // Skip if same format
  if (sourceFormat === targetFormat) {
    result.skipped = true;
    return result;
  }

  // Read patch JSON
  var jsonBytes;
  try {
    jsonBytes = NS.readPatchData(inputData, sourceFormat);
  } catch(e) {
    result.error = 'Failed to read input file: ' + e.message;
    return result;
  }

  // Parse JSON
  var patch;
  try {
    patch = NS.fromJSON(jsonBytes);
  } catch(e) {
    result.error = 'Failed to parse JSON: ' + e.message;
    return result;
  }

  // Validate MiRack audio module constraints
  if ((sourceFormat === NS.FORMAT_MIRACK || targetFormat === NS.FORMAT_MIRACK) && patch.modules) {
    var validation = NS.validateMiRackAudioModuleCount(patch.modules);
    if (validation && !validation.valid) {
      result.skipped = true;
      result.issues = [validation.skipReason];
      return result;
    }
  }

  // Normalize source format
  try {
    switch (sourceFormat) {
      case NS.FORMAT_V2:       NS.normalizeV2(patch, issues); break;
      case NS.FORMAT_MIRACK:   NS.normalizeMiRack(patch, issues); break;
      case NS.FORMAT_V06:      NS.normalizeV06(patch, issues); break;
      case NS.FORMAT_CARDINAL: NS.normalizeCardinal(patch, issues); break;
    }
  } catch(e) {
    result.error = 'Normalization failed: ' + e.message;
    result.issues = issues;
    return result;
  }

  // Add MetaModule if requested
  if (options.metamodule) {
    var modules = patch.modules;
    if (Array.isArray(modules)) {
      modules.push(createHubMediumModule(modules, patch, inputFilename));
    }
  }

  // Denormalize to target format
  try {
    switch (targetFormat) {
      case NS.FORMAT_V2:       NS.denormalizeV2(patch, issues); break;
      case NS.FORMAT_MIRACK:   NS.denormalizeMiRack(patch, issues); break;
      case NS.FORMAT_V06:      NS.denormalizeV06(patch, issues); break;
      case NS.FORMAT_CARDINAL: NS.denormalizeCardinal(patch, issues); break;
      default:
        result.error = 'Unknown target format: ' + targetFormat;
        return result;
    }
  } catch(e) {
    result.error = 'Denormalization failed: ' + e.message;
    result.issues = issues;
    return result;
  }

  // Serialize JSON
  var patchJSON;
  try {
    patchJSON = NS.toJSON(patch);
  } catch(e) {
    result.error = 'Failed to serialize JSON: ' + e.message;
    return result;
  }

  // Create output
  var baseName = (inputFilename || 'patch').replace(/\.[^.\/]+$/, '');

  if (targetFormat === NS.FORMAT_MIRACK) {
    // MiRack: output as .mrk.zip containing patch.vcv
    var mrkDirName = baseName + '.mrk';
    try {
      result.outputData = NS.createMiRackZip(mrkDirName, patchJSON);
      result.outputFilename = mrkDirName + '.zip';
    } catch(e) {
      // Fallback: plain JSON
      result.outputData = patchJSON;
      result.outputFilename = baseName + '_mirack_patch.vcv';
      issues.push('Note: Output saved as plain JSON (zip unavailable). Place this file in an .mrk directory as patch.vcv for MiRack.');
    }
  } else if (targetFormat === NS.FORMAT_V06) {
    // V0.6: output plain JSON
    result.outputData = patchJSON;
    result.outputFilename = baseName + '_converted.vcv';
  } else {
    // V2/Cardinal: try zstd tar, fallback to plain JSON
    try {
      result.outputData = NS.createV2Patch(patchJSON);
    } catch(e) {
      result.outputData = patchJSON;
      result.outputFilename = baseName + '_converted.json';
      issues.push('Note: Output saved as plain JSON (zstd compression unavailable). VCV Rack can still read this file.');
    }
  }

  result.success = true;
  result.issues = issues;
  return result;
};

/**
 * Get human-readable format name.
 */
NS.getFormatDisplayName = function(format) {
  switch (format) {
    case NS.FORMAT_V2: return 'VCV Rack v2';
    case NS.FORMAT_V06: return 'VCV Rack v0.6';
    case NS.FORMAT_MIRACK: return 'MiRack';
    case NS.FORMAT_CARDINAL: return 'Cardinal';
    default: return 'Unknown';
  }
};

/**
 * Get all supported target formats for a given source format.
 */
NS.getSupportedTargets = function(sourceFormat) {
  var all = [NS.FORMAT_V2, NS.FORMAT_MIRACK, NS.FORMAT_V06, NS.FORMAT_CARDINAL];
  return all.filter(function(f) { return f !== sourceFormat; });
};

// ---- MetaModule support (port of metamodule.go) ----

function createHubMediumModule(existingModules, patch, inputFilename) {
  var maxX = -1;
  var topY = 0;
  for (var i = 0; i < existingModules.length; i++) {
    var mod = existingModules[i];
    if (!mod || typeof mod !== 'object') continue;
    if (Array.isArray(mod.pos) && mod.pos.length >= 2) {
      var x = typeof mod.pos[0] === 'number' ? mod.pos[0] : 0;
      var y = typeof mod.pos[1] === 'number' ? mod.pos[1] : 0;
      if (Math.floor(y) === topY && Math.floor(x) > maxX) {
        maxX = Math.floor(x);
      }
    }
  }

  var posX = maxX >= 0 ? maxX + 1 : 0;

  var patchName = 'Enter Patch Name';
  var patchDesc = 'Patch Description';

  if (patch.name && typeof patch.name === 'string' && patch.name !== '') {
    patchName = patch.name;
  } else if (inputFilename) {
    patchName = inputFilename.replace(/\.[^.\/]+$/, '');
  }
  if (patch.description && typeof patch.description === 'string' && patch.description !== '') {
    patchDesc = patch.description;
  }

  return {
    id: getNextModuleID(existingModules),
    plugin: '4msCompany',
    model: 'HubMedium',
    version: '2.1.4',
    params: getDefaultHubMediumParams(),
    data: getDefaultHubMediumData(patchName, patchDesc),
    pos: [posX, topY]
  };
}

function getDefaultHubMediumParams() {
  var params = new Array(14);
  for (var i = 0; i < 12; i++) {
    params[i] = { value: 0.5, id: i };
  }
  params[12] = { value: 0.0, id: 12 };
  params[13] = { value: 0.0, id: 13 };
  return params;
}

function getDefaultHubMediumData(patchName, patchDesc) {
  return {
    Mappings: new Array(8),
    KnobSetNames: new Array(8),
    Alias: { Input: new Array(8), Output: new Array(8) },
    PatchName: patchName,
    PatchDesc: patchDesc,
    MappingMode: 0,
    SuggestedSampleRate: 0,
    SuggestedBlockSize: 0,
    DefaultKnobSet: 0
  };
}

function getNextModuleID(modules) {
  var maxID = -1;
  for (var i = 0; i < modules.length; i++) {
    var mod = modules[i];
    if (mod && typeof mod === 'object') {
      var id = NS.getInt64(mod, 'id');
      if (id > maxID) maxID = id;
    }
  }
  return maxID + 1;
}

})(window.VRackConverter);
