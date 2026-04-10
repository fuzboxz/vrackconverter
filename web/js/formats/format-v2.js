/**
 * format-v2.js - Port of Go v2.go
 *
 * VCV Rack v2 format handler. V2 is the "superset" format -
 * normalization is mostly validation and building ID-to-index mapping.
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
  'use strict';

  /**
   * Normalize a VCV Rack v2 patch to internal format.
   * V2 is the canonical format, so this builds mappings and validates structure.
   * Port of Go NormalizeV2().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.normalizeV2 = function(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      issues.push('v2 normalization: no modules array found');
      return;
    }

    // Build ID-to-index mapping
    var idToIndex = {};
    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') {
        issues.push('v2 normalization: module[' + i + ']: invalid module object');
        continue;
      }
      var id = NS.getInt64(mod, 'id');
      if (id >= 0) {
        if (idToIndex[id] !== undefined) {
          issues.push('v2 normalization: duplicate module ID ' + id + ' at indices ' + idToIndex[id] + ' and ' + i);
        }
        idToIndex[id] = i;
      }
    }
    patch._idToIndex = idToIndex;

    // Validate cables reference valid module IDs
    if (Array.isArray(patch.cables)) {
      for (var ci = 0; ci < patch.cables.length; ci++) {
        var cable = patch.cables[ci];
        if (!cable || typeof cable !== 'object') {
          issues.push('v2 normalization: cable[' + ci + ']: invalid cable object');
          continue;
        }
        var outputModID = NS.getInt64(cable, 'outputModuleId');
        var inputModID = NS.getInt64(cable, 'inputModuleId');
        if (idToIndex[outputModID] === undefined) {
          issues.push('v2 normalization: cable[' + ci + ']: outputModuleId ' + outputModID + ' not found');
        }
        if (idToIndex[inputModID] === undefined) {
          issues.push('v2 normalization: cable[' + ci + ']: inputModuleId ' + inputModID + ' not found');
        }
      }
    }

    // Ensure cables array exists
    if (!patch.cables) {
      if (patch.wires) {
        patch.cables = patch.wires;
        delete patch.wires;
        issues.push('v2 normalization: converted wires to cables');
      } else {
        patch.cables = [];
      }
    }

    // Store expander links
    var expanderLinks = {};
    var hasLinks = false;
    for (var ei = 0; ei < modules.length; ei++) {
      var em = modules[ei];
      if (!em || typeof em !== 'object') continue;
      var eid = NS.getInt64(em, 'id');
      if (eid >= 0) {
        var links = {};
        if ('leftModuleId' in em && em.leftModuleId != null) {
          links.leftModuleId = NS.getInt64(em, 'leftModuleId');
        }
        if ('rightModuleId' in em && em.rightModuleId != null) {
          links.rightModuleId = NS.getInt64(em, 'rightModuleId');
        }
        if (Object.keys(links).length > 0) {
          expanderLinks[eid] = links;
          hasLinks = true;
        }
      }
    }
    if (hasLinks) {
      patch._expanderLinks = expanderLinks;
    }

    // Ensure version
    if (!patch.version || typeof patch.version !== 'string' || patch.version === '') {
      patch.version = '2.6.6';
      issues.push('v2 normalization: set default version to 2.6.6');
    }
  };

  /**
   * Denormalize from internal format to VCV Rack v2 format.
   * Adds v2-specific fields and ensures patch validity.
   * Port of Go DenormalizeV2().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.denormalizeV2 = function(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      issues.push('v2 denormalization: no modules array found');
      return;
    }

    // Ensure all modules have required v2 fields
    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') continue;

      // Convert disabled to bypass
      if ('disabled' in mod) {
        if (typeof mod.disabled === 'boolean') {
          mod.bypass = mod.disabled;
          issues.push('v2 denormalization: module[' + i + ']: converted disabled=' + mod.disabled + ' to bypass');
        }
        delete mod.disabled;
      }

      // Ensure module has an ID
      if (!('id' in mod)) {
        mod.id = i;
        issues.push('v2 denormalization: module[' + i + ']: assigned ID ' + i);
      }
    }

    // Restore expander links if stored
    if (patch._expanderLinks) {
      var expanderLinks = patch._expanderLinks;
      for (var li = 0; li < modules.length; li++) {
        var lm = modules[li];
        if (!lm || typeof lm !== 'object') continue;
        var lid = NS.getInt64(lm, 'id');
        var linkData = expanderLinks[lid] || expanderLinks[String(lid)];
        if (linkData) {
          if (linkData.leftModuleId !== undefined && linkData.leftModuleId !== 0) {
            lm.leftModuleId = typeof linkData.leftModuleId === 'number' ? linkData.leftModuleId : NS.parseStringKeyToInt(String(linkData.leftModuleId));
          }
          if (linkData.rightModuleId !== undefined && linkData.rightModuleId !== 0) {
            lm.rightModuleId = typeof linkData.rightModuleId === 'number' ? linkData.rightModuleId : NS.parseStringKeyToInt(String(linkData.rightModuleId));
          }
        }
      }
    }

    // Ensure cables array exists
    if (!patch.cables) {
      if (patch.wires) {
        patch.cables = patch.wires;
        delete patch.wires;
        issues.push('v2 denormalization: converted wires to cables');
      } else {
        patch.cables = [];
      }
    }

    // Ensure all cables have IDs
    if (Array.isArray(patch.cables)) {
      for (var ci = 0; ci < patch.cables.length; ci++) {
        var cable = patch.cables[ci];
        if (cable && typeof cable === 'object' && !('id' in cable)) {
          cable.id = ci;
        }
      }
    }

    // Set version
    patch.version = '2.6.6';

    // Clean up internal fields
    delete patch._idToIndex;
    delete patch._originalIndexToID;
  };

})(window.VRackConverter);
