/**
 * patch.js - Port of Go common.go
 *
 * Common utilities for VCV Rack patch conversion.
 * All functions attached to window.VRackConverter namespace.
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
  'use strict';

  // fundamentalModules contains modules from VCV Rack v0.6 Fundamental plugin.
  // This lookup is ONLY for v0.6 files - MiRack does NOT have "Fundamental" plugin.
  NS.fundamentalModules = {
    'VCO-1': true, 'VCO-2': true,
    'VCF': true,
    'VCA-1': true, 'VCA-2': true,
    'LFO': true, 'LFO-2': true,
    'ADSR': true, 'Decay': true,
    'VCMixer': true, 'Unity': true,
    '8vert': true,
    'Merge': true,
    'Split': true,
    'Sum': true,
    'Momentary': true, 'Button': true, 'Latch': true,
    'Gate': true,
    'Clock': true,
    'Noise': true,
    'SampleHold': true,
    'Scope': true,
    'Notes': true,
    'Text': true
  };

  /**
   * fromJSON parses JSON bytes (Uint8Array) into an object.
   * Port of Go FromJSON().
   *
   * @param {Uint8Array} data - JSON bytes
   * @returns {Object} Parsed object
   * @throws {Error} If JSON parsing fails
   */
  NS.fromJSON = function(data) {
    var decoder = new TextDecoder('utf-8');
    var str = decoder.decode(data);
    return JSON.parse(str);
  };

  /**
   * toJSON serializes an object to indented JSON bytes (Uint8Array).
   * Port of Go ToJSON().
   *
   * @param {Object} obj - Object to serialize
   * @returns {Uint8Array} JSON bytes with 2-space indentation
   */
  NS.toJSON = function(obj) {
    var str = JSON.stringify(obj, null, 2);
    var encoder = new TextEncoder();
    return encoder.encode(str);
  };

  /**
   * getInt64 extracts an integer value from a map using the given key.
   * Handles float64 (JSON number default) and integer types.
   * Port of Go getInt64FromMap().
   *
   * @param {Object} map - The map/object to read from
   * @param {string} key - The key to look up
   * @returns {number} Integer value, or 0 if not found/invalid
   */
  NS.getInt64 = function(map, key) {
    if (map && key in map) {
      var val = map[key];
      if (typeof val === 'number') {
        return Math.trunc(val);
      }
    }
    return 0;
  };

  /**
   * getModules safely extracts the modules array from a patch.
   * Port of Go getModules().
   *
   * @param {Object} patch - The patch object
   * @returns {Array|null} Modules array, or null if not present
   */
  NS.getModules = function(patch) {
    if (patch && Array.isArray(patch.modules)) {
      return patch.modules;
    }
    return null;
  };

  /**
   * findModuleByID searches for a module with the given ID in a modules array.
   * Port of Go findModuleByID().
   *
   * @param {Array} modules - Array of module objects
   * @param {number} id - Module ID to find
   * @returns {Object|null} Module object if found, null otherwise
   */
  NS.findModuleByID = function(modules, id) {
    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (mod && typeof mod === 'object' && NS.getInt64(mod, 'id') === id) {
        return mod;
      }
    }
    return null;
  };

  /**
   * convertColorToHex converts a color map with r,g,b,a float values (0-1 range)
   * to a hexadecimal string format "rrggbbaa".
   * Port of Go convertColorToHex().
   *
   * @param {Object} color - Color object with r, g, b, a properties (0-1 floats)
   * @returns {string} Hex color string "rrggbbaa", or empty string if invalid
   */
  NS.convertColorToHex = function(color) {
    if (!color || typeof color !== 'object') {
      return '';
    }

    var r = color.r;
    var g = color.g;
    var b = color.b;
    var a = color.a;

    if (typeof r !== 'number' || typeof g !== 'number' || typeof b !== 'number') {
      return '';
    }

    if (typeof a !== 'number') {
      a = 1.0;
    }

    var toHex = function(v) {
      var hex = Math.round(v * 255).toString(16);
      return hex.length < 2 ? '0' + hex : hex;
    };

    return toHex(r) + toHex(g) + toHex(b) + toHex(a);
  };

  /**
   * hexToRGB converts "#rrggbb" to {r, g, b} with values 0-255.
   * Accepts hex with or without "#" prefix.
   * Port of Go hexToRGB().
   *
   * @param {string} hexColor - Hex color string "#rrggbb" or "rrggbb"
   * @returns {Object|null} {r, g, b} with 0-255 values, or null if invalid
   */
  NS.hexToRGB = function(hexColor) {
    if (typeof hexColor !== 'string') {
      return null;
    }

    var s = hexColor;
    if (s.length > 0 && s.charAt(0) === '#') {
      s = s.substring(1);
    }

    if (s.length !== 6) {
      return null;
    }

    // Validate all chars are hex digits
    if (!/^[0-9a-fA-F]{6}$/.test(s)) {
      return null;
    }

    return {
      r: parseInt(s.substring(0, 2), 16),
      g: parseInt(s.substring(2, 4), 16),
      b: parseInt(s.substring(4, 6), 16)
    };
  };

  /**
   * rgbToHex converts RGB bytes (0-255) to "#rrggbb" hex string.
   * Port of Go rgbToHex().
   *
   * @param {number} r - Red component (0-255)
   * @param {number} g - Green component (0-255)
   * @param {number} b - Blue component (0-255)
   * @returns {string} Hex color string "#rrggbb"
   */
  NS.rgbToHex = function(r, g, b) {
    var toHex = function(v) {
      var hex = Math.round(v).toString(16);
      return hex.length < 2 ? '0' + hex : hex;
    };
    return '#' + toHex(r) + toHex(g) + toHex(b);
  };

  /**
   * convertParamIDToID converts paramId to id in parameters.
   * Used when normalizing v0.6/MiRack formats to v2.
   * Port of Go convertParamIDToID().
   *
   * @param {Object} mod - Module object
   * @param {number} moduleIndex - Module index (for issue reporting)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.convertParamIDToID = function(mod, moduleIndex, issues) {
    if (!mod || !Array.isArray(mod.params)) {
      return;
    }

    for (var i = 0; i < mod.params.length; i++) {
      var param = mod.params[i];
      if (!param || typeof param !== 'object') {
        if (issues) {
          issues.push('module[' + moduleIndex + ']: param[' + i + ']: invalid param object');
        }
        continue;
      }

      if ('paramId' in param) {
        param.id = param.paramId;
        delete param.paramId;
      } else if (!('id' in param)) {
        param.id = i;
      }
    }
  };

  /**
   * convertIDToParamID converts id to paramId in parameters.
   * Used when denormalizing v2 to v0.6/MiRack formats.
   * Port of Go convertIDToParamID().
   *
   * @param {Object} mod - Module object
   */
  NS.convertIDToParamID = function(mod) {
    if (!mod || !Array.isArray(mod.params)) {
      return;
    }

    for (var i = 0; i < mod.params.length; i++) {
      var param = mod.params[i];
      if (!param || typeof param !== 'object') {
        continue;
      }
      if ('id' in param) {
        param.paramId = param.id;
        delete param.id;
      }
    }
  };

  /**
   * convertDisabledToBypass converts disabled to bypass.
   * Used when normalizing v0.6/MiRack formats to v2.
   * Port of Go convertDisabledToBypass().
   *
   * @param {Object} mod - Module object
   * @param {Array} issues - Array to push warning messages to (unused but kept for API consistency)
   */
  NS.convertDisabledToBypass = function(mod, issues) {
    if (!mod || !('disabled' in mod)) {
      return;
    }

    if (typeof mod.disabled === 'boolean') {
      mod.bypass = mod.disabled;
    }
    delete mod.disabled;
  };

  /**
   * convertBypassToDisabled converts bypass to disabled.
   * Used when denormalizing v2 to v0.6/MiRack formats.
   * Port of Go convertBypassToDisabled().
   *
   * @param {Object} mod - Module object
   * @param {Array} issues - Array to push warning messages to (unused but kept for API consistency)
   */
  NS.convertBypassToDisabled = function(mod, issues) {
    if (!mod || !('bypass' in mod)) {
      return;
    }

    if (typeof mod.bypass === 'boolean') {
      mod.disabled = mod.bypass;
    }
    delete mod.bypass;
  };

  /**
   * parseStringKeyToInt parses a string key to integer.
   * Needed when handling JSON-deserialized maps with string keys.
   * Port of Go parseStringKeyToInt64().
   *
   * @param {string} s - String to parse
   * @returns {number} Parsed integer, or NaN if invalid
   */
  NS.parseStringKeyToInt = function(s) {
    return parseInt(s, 10);
  };

})(window.VRackConverter);
