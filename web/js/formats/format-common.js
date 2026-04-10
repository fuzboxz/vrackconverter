/**
 * format-common.js - Port of Go legacy.go
 *
 * Shared V06-style normalization/denormalization for V0.6 and MiRack formats.
 * All functions attached to window.VRackConverter namespace.
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
  'use strict';

  /**
   * Normalize a V06-style format (v0.6 or MiRack) to v2 internal format.
   * Port of Go NormalizeV06Style().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Object} config - Format-specific configuration
   * @param {string} config.formatName - Format name for logging
   * @param {boolean} config.hasFundamental - Whether format has Fundamental plugin
   * @param {Function} [config.convertColor] - Optional color conversion callback
   * @param {Function} [config.normalizePlugin] - Optional plugin name conversion callback
   * @param {Function} [config.denormalizePlugin] - Optional plugin name conversion callback (unused in normalize)
   * @param {Array} issues - Array to push warning messages to
   * @returns {undefined}
   */
  NS.normalizeV06Style = function(patch, config, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      issues.push(config.formatName + ' normalization: no modules array found');
      return;
    }

    // Build index-to-ID mapping for cable reference conversion
    var indexToID = {};
    var nextID = 0;

    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') {
        issues.push(config.formatName + ' normalization: module[' + i + ']: invalid module object');
        continue;
      }

      // Apply plugin mapping using the format-specific callback
      var plugin = mod.plugin;
      if (typeof plugin === 'string' && typeof mod.model === 'string') {
        if (config.normalizePlugin) {
          var result = config.normalizePlugin(plugin, mod.model);
          if (result.modified) {
            var oldPlugin = plugin;
            mod.plugin = result.plugin;
            issues.push(config.formatName + ' normalization: module[' + i + ']: ' +
              oldPlugin + '/' + mod.model + ' -> ' + result.plugin + '/' + mod.model);
          }
        }
      }

      // Assign valid module ID for VCV Rack 2
      var moduleID;
      if ('id' in mod) {
        moduleID = NS.getInt64(mod, 'id');
        if (moduleID < 0) {
          var oldID = moduleID;
          moduleID = nextID;
          nextID++;
          mod.id = moduleID;
          issues.push(config.formatName + ' normalization: module[' + i + ']: reassigned negative ID ' + oldID + ' to ' + moduleID);
        } else {
          if (moduleID >= nextID) {
            nextID = moduleID + 1;
          }
        }
      } else {
        moduleID = nextID;
        nextID++;
        mod.id = moduleID;
      }
      indexToID[i] = moduleID;

      // Convert paramId to id in parameters
      NS.convertParamIDToID(mod, i, issues);

      // Convert disabled to bypass (v2 format)
      NS.convertDisabledToBypass(mod, issues);

      // Remove format-specific fields not used in v2
      delete mod.sumPolyInputs;
    }

    // Store the index-to-ID mapping for later use during denormalization
    patch._originalIndexToID = indexToID;

    // Store expander links for V2 roundtrip
    var expanderLinks = {};
    var hasExpanderLinks = false;
    for (var j = 0; j < modules.length; j++) {
      var m = modules[j];
      if (!m || typeof m !== 'object') continue;
      var id = NS.getInt64(m, 'id');
      if (id >= 0) {
        var links = {};
        if ('leftModuleId' in m && m.leftModuleId != null) {
          links.leftModuleId = NS.getInt64(m, 'leftModuleId');
        }
        if ('rightModuleId' in m && m.rightModuleId != null) {
          links.rightModuleId = NS.getInt64(m, 'rightModuleId');
        }
        var linkKeys = Object.keys(links);
        if (linkKeys.length > 0) {
          expanderLinks[id] = links;
          hasExpanderLinks = true;
        }
      }
    }
    if (hasExpanderLinks) {
      patch._expanderLinks = expanderLinks;
    }

    // Convert wires to cables
    if ('wires' in patch) {
      patch.cables = patch.wires;
      delete patch.wires;

      var cables = patch.cables;
      if (Array.isArray(cables)) {
        var validCables = [];
        for (var ci = 0; ci < cables.length; ci++) {
          var cable = cables[ci];
          if (!cable || typeof cable !== 'object') {
            issues.push(config.formatName + ' normalization: cable[' + ci + ']: invalid cable object');
            continue;
          }

          // Get cable references (these are array indices in v0.6/MiRack)
          var outputModuleIdx = NS.getInt64(cable, 'outputModuleId');
          var inputModuleIdx = NS.getInt64(cable, 'inputModuleId');

          // Convert array indices to module IDs
          var outputModuleID = indexToID[outputModuleIdx];
          var inputModuleID = indexToID[inputModuleIdx];

          if (outputModuleID === undefined) {
            issues.push(config.formatName + ' normalization: cable[' + ci + ']: outputModuleId index ' + outputModuleIdx + ' out of range');
            continue;
          }
          if (inputModuleID === undefined) {
            issues.push(config.formatName + ' normalization: cable[' + ci + ']: inputModuleId index ' + inputModuleIdx + ' out of range');
            continue;
          }

          // Update cable with resolved module IDs
          cable.outputModuleId = outputModuleID;
          cable.inputModuleId = inputModuleID;

          // Apply format-specific color conversion if provided
          if (config.convertColor) {
            config.convertColor(cable, issues);
          }

          validCables.push(cable);
        }
        patch.cables = validCables;
      }
    }

    // Ensure version is set
    patch.version = '2.6.6';
  };

  /**
   * Get the ID-to-index mapping from a normalized v2 patch.
   * Port of Go GetIDToIndexMapping().
   */
  function getIDToIndexMapping(patch) {
    if (patch._idToIndex) {
      return patch._idToIndex;
    }
    return null;
  }

  /**
   * Denormalize from v2 internal format to a V06-style format.
   * Port of Go DenormalizeV06Style().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Object} config - Format-specific configuration (same as normalize)
   * @param {Array} issues - Array to push warning messages to
   * @returns {undefined}
   */
  NS.denormalizeV06Style = function(patch, config, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      issues.push(config.formatName + ' denormalization: no modules array found');
      return;
    }

    // Build ID-to-index mapping
    var idToIndex = null;

    // First try to get the original index-to-ID mapping
    if (patch._originalIndexToID) {
      idToIndex = {};
      var indexToID = patch._originalIndexToID;
      var keys = Object.keys(indexToID);
      for (var ki = 0; ki < keys.length; ki++) {
        var idx = parseInt(keys[ki], 10);
        var mid = indexToID[keys[ki]];
        idToIndex[mid] = idx;
      }
    }

    // If no original mapping found, try the v2 normalization mapping
    if (!idToIndex) {
      idToIndex = getIDToIndexMapping(patch);
    }

    // Fall back to building mapping on-the-fly
    if (!idToIndex) {
      idToIndex = {};
      for (var fi = 0; fi < modules.length; fi++) {
        var fm = modules[fi];
        if (fm && typeof fm === 'object') {
          var fid = NS.getInt64(fm, 'id');
          if (fid >= 0) {
            idToIndex[fid] = fi;
          }
        }
      }
    }

    // Convert modules
    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') continue;

      // Apply plugin mapping
      var plugin = mod.plugin;
      if (typeof plugin === 'string' && typeof mod.model === 'string') {
        if (config.denormalizePlugin) {
          var newPlugin = plugin;
          var modified = false;

          // Special handling for MiRack: Fundamental -> Core
          if (!config.hasFundamental && plugin === 'Fundamental') {
            newPlugin = 'Core';
            modified = true;
          } else {
            var res = config.denormalizePlugin(plugin, mod.model);
            newPlugin = res.plugin;
            modified = res.modified;
          }

          if (modified) {
            var oldPlugin = plugin;
            mod.plugin = newPlugin;
            // Only log if not the automatic MiRack Fundamental -> Core conversion
            if (!(!config.hasFundamental && oldPlugin === 'Fundamental')) {
              issues.push(config.formatName + ' denormalization: module[' + i + ']: ' +
                oldPlugin + '/' + mod.model + ' -> ' + newPlugin + '/' + mod.model);
            }
          }
        }
      }

      // Convert bypass to disabled
      NS.convertBypassToDisabled(mod, issues);

      // Convert id to paramId in parameters
      NS.convertIDToParamID(mod);

      // Remove v2-specific fields
      delete mod.version;
      delete mod.leftModuleId;
      delete mod.rightModuleId;
    }

    // Convert cables to wires
    if ('cables' in patch) {
      patch.wires = patch.cables;
      delete patch.cables;

      var wires = patch.wires;
      if (Array.isArray(wires)) {
        var validWires = [];
        for (var wi = 0; wi < wires.length; wi++) {
          var wire = wires[wi];
          if (!wire || typeof wire !== 'object') continue;

          // Get module IDs
          var outputModuleID = NS.getInt64(wire, 'outputModuleId');
          var inputModuleID = NS.getInt64(wire, 'inputModuleId');

          // Convert module IDs to array indices
          var outputIndex = idToIndex[outputModuleID];
          var inputIndex = idToIndex[inputModuleID];

          if (outputIndex === undefined) {
            issues.push(config.formatName + ' denormalization: wire[' + wi + ']: outputModuleId ' + outputModuleID + ' not found in mapping');
          } else {
            wire.outputModuleId = outputIndex;
          }

          if (inputIndex === undefined) {
            issues.push(config.formatName + ' denormalization: wire[' + wi + ']: inputModuleId ' + inputModuleID + ' not found in mapping');
          } else {
            wire.inputModuleId = inputIndex;
          }

          // Remove cable ID
          delete wire.id;

          // Apply format-specific color conversion
          if (config.convertColor) {
            config.convertColor(wire, issues);
          }

          validWires.push(wire);
        }
        patch.wires = validWires;
      }
    }

    // Remove v2-specific fields
    delete patch.masterModuleId;
    delete patch._idToIndex;
    delete patch._originalIndexToID;

    // Set version to v0.6 format
    patch.version = '0.6.13';
  };

})(window.VRackConverter);
