/**
 * format-v06.js - Port of Go v06.go
 *
 * VCV Rack v0.6 format handler. Uses V06StyleConfig with hasFundamental=true.
 * V0.6 files can be plain JSON or zstd tar archives with version "0.x.x".
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
  'use strict';

  /**
   * V0.6 plugin conversion: Fundamental -> Core during normalization.
   */
  function normalizeV06Plugin(plugin, model) {
    if (plugin === 'Fundamental') {
      return { plugin: 'Core', modified: true };
    }
    return { plugin: plugin, modified: false };
  }

  /**
   * V2 -> V0.6 plugin conversion: Core -> Fundamental for known fundamental modules.
   */
  function denormalizeV06Plugin(plugin, model) {
    if (plugin === 'Core' && NS.fundamentalModules[model]) {
      return { plugin: 'Fundamental', modified: true };
    }
    return { plugin: plugin, modified: false };
  }

  /**
   * Normalize a VCV Rack v0.6 patch to internal v2 format.
   * Port of Go NormalizeV06().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.normalizeV06 = function(patch, issues) {
    var config = {
      formatName: 'v0.6',
      hasFundamental: true,
      convertColor: null,
      normalizePlugin: normalizeV06Plugin,
      denormalizePlugin: denormalizeV06Plugin
    };
    NS.normalizeV06Style(patch, config, issues);
    // V0.6 AudioInterface is compatible with V2's AudioInterface (no change needed)
  };

  /**
   * Denormalize from internal v2 format to VCV Rack v0.6 format.
   * Port of Go DenormalizeV06().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.denormalizeV06 = function(patch, issues) {
    var config = {
      formatName: 'v0.6',
      hasFundamental: true,
      convertColor: null,
      normalizePlugin: normalizeV06Plugin,
      denormalizePlugin: denormalizeV06Plugin
    };
    NS.denormalizeV06Style(patch, config, issues);
  };

})(window.VRackConverter);
