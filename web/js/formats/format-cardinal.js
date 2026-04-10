/**
 * format-cardinal.js - Cardinal format handler
 *
 * Cardinal uses the same .vcv file format as VCV Rack v2.
 * The only difference is module slug remapping:
 * - Cardinal uses its own audio/MIDI modules under the "Cardinal" plugin
 * - VCV Rack v2 uses "Core" plugin for these modules
 *
 * Detection: Cardinal patches contain modules with plugin="Cardinal"
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
'use strict';

// Cardinal -> V2 module mapping
var cardinalToV2Map = {
  'HostAudio2':    { v2Model: 'AudioInterface2',  v2Plugin: 'Core' },
  'HostAudio8':    { v2Model: 'AudioInterface',    v2Plugin: 'Core' },
  'HostMIDI':      { v2Model: 'MIDIToCVInterface',       v2Plugin: 'Core' },
  'HostMIDICC':    { v2Model: 'MIDICCToCVInterface',     v2Plugin: 'Core' },
  'HostMIDIGate':  { v2Model: 'MIDITriggerToCVInterface', v2Plugin: 'Core' },
  'HostCV':        null, // No V2 equivalent
  'HostTime':      null, // No V2 equivalent
  'HostParameters': null,
  'HostParametersMap': null,
  'HostMIDIMap':   null,
  'ExpanderInputMIDI': null,
  'ExpanderOutputMIDI': null,
  'Carla':         null,
  'Ildaeil':       null,
  'SassyScope':    null,
  'TextEditor':    null,
  'AIDA-X':        null,
  'AudioFile':     null,
  'AudioToCVPitch': null,
  'Blank':         null,
  'glBars':        null,
  'DearImGui':     null,
  'DearImGuiColorTextEditor': null
};

// V2 -> Cardinal module mapping (reverse of above, only for mappable modules)
var v2ToCardinalMap = {
  'AudioInterface2':            { cardinalModel: 'HostAudio2',  cardinalPlugin: 'Cardinal' },
  'AudioInterface':             { cardinalModel: 'HostAudio8',  cardinalPlugin: 'Cardinal' },
  'MIDIToCVInterface':          { cardinalModel: 'HostMIDI',    cardinalPlugin: 'Cardinal' },
  'MIDICCToCVInterface':        { cardinalModel: 'HostMIDICC',  cardinalPlugin: 'Cardinal' },
  'MIDITriggerToCVInterface':   { cardinalModel: 'HostMIDIGate', cardinalPlugin: 'Cardinal' }
};

/**
 * Detect if a patch is Cardinal format by checking for Cardinal plugin modules.
 * Must be called AFTER extracting JSON from the .vcv file.
 *
 * @param {Object} patch - The parsed patch object
 * @returns {boolean} True if the patch contains Cardinal modules
 */
NS.detectCardinalByContent = function(patch) {
  var modules = NS.getModules(patch);
  if (!modules) return false;
  for (var i = 0; i < modules.length; i++) {
    var mod = modules[i];
    if (mod && typeof mod === 'object' && mod.plugin === 'Cardinal') {
      return true;
    }
  }
  return false;
};

/**
 * Normalize a Cardinal patch to v2 internal format.
 * Remaps Cardinal module slugs to V2 equivalents, then applies V2 normalization.
 */
NS.normalizeCardinal = function(patch, issues) {
  var modules = NS.getModules(patch);
  if (!modules) {
    issues.push('Cardinal normalization: no modules array found');
    return;
  }

  // Pass 1: Remap Cardinal modules to V2 equivalents
  for (var i = 0; i < modules.length; i++) {
    var mod = modules[i];
    if (!mod || typeof mod !== 'object') continue;
    if (mod.plugin !== 'Cardinal') continue;

    var mapping = cardinalToV2Map[mod.model];
    if (mapping) {
      issues.push('Cardinal normalization: module[' + i + ']: Cardinal/' + mod.model + ' -> ' + mapping.v2Plugin + '/' + mapping.v2Model);
      mod.plugin = mapping.v2Plugin;
      mod.model = mapping.v2Model;
    }
    // Cardinal-specific modules with no V2 equivalent: leave as-is with a warning
    // (they'll be passed through but won't function in VCV Rack)
  }

  // Pass 2: Apply standard V2 normalization
  NS.normalizeV2(patch, issues);
};

/**
 * Denormalize from v2 internal format to Cardinal format.
 * Applies V2 denormalization first, then remaps module slugs.
 */
NS.denormalizeCardinal = function(patch, issues) {
  // Pass 1: Apply standard V2 denormalization
  NS.denormalizeV2(patch, issues);

  // Pass 2: Remap V2 modules to Cardinal equivalents
  var modules = NS.getModules(patch);
  if (!modules) return;

  for (var i = 0; i < modules.length; i++) {
    var mod = modules[i];
    if (!mod || typeof mod !== 'object') continue;

    var mapping = v2ToCardinalMap[mod.model];
    if (mapping) {
      issues.push('Cardinal denormalization: module[' + i + ']: Core/' + mod.model + ' -> Cardinal/' + mapping.cardinalModel);
      mod.plugin = mapping.cardinalPlugin;
      mod.model = mapping.cardinalModel;
    }
  }
};

})(window.VRackConverter);
