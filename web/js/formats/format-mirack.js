/**
 * format-mirack.js - Port of Go mirack.go
 *
 * MiRack format handler. MiRack uses separate AudioInterface + AudioInterfaceIn
 * modules that must be merged into V2's single audio module. Also handles
 * MiRack's colorIndex palette and module name mappings.
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
  'use strict';

  // ============================================================================
  // MiRack Color Palette
  // ============================================================================

  /**
   * miRackColorPalette defines the 6 colors available in MiRack by colorIndex.
   * Order: yellow (0), red (1), green (2), teal (3), orange (4), purple (5)
   * Values are RGB bytes (0-255).
   */
  var miRackColorPalette = [
    {name: 'yellow', r: 255, g: 181, b: 0},    // colorIndex 0: #ffb500
    {name: 'red', r: 242, g: 56, b: 74},       // colorIndex 1: #f2384a
    {name: 'green', r: 0, g: 181, b: 110},     // colorIndex 2: #00b56e
    {name: 'teal', r: 54, g: 149, b: 239},     // colorIndex 3: #3695ef
    {name: 'orange', r: 255, g: 181, b: 56},   // colorIndex 4: #ffb538
    {name: 'purple', r: 140, g: 74, b: 181}    // colorIndex 5: #8c4ab5
  ];

  /**
   * miRackToV2ModuleMap maps MiRack model names to VCV Rack V2 model names.
   * Only modules that have different names need to be listed.
   * Note: Audio modules are handled by merge logic, not by this map.
   * V2 audio modules: AudioInterface (8-ch), AudioInterface2 (2-ch), AudioInterface16 (16-ch).
   */
  var miRackToV2ModuleMap = {
    // Audio modules - merged separately, these entries are for reference only
    'AudioInterface': 'AudioInterface',
    'AudioInterfaceIn': 'AudioInterface',
    'AudioInterface8': 'AudioInterface',
    'AudioInterfaceIn8': 'AudioInterface',
    'AudioInterface16': 'AudioInterface16',
    'AudioInterfaceIn16': 'AudioInterface16',
    // MIDI modules
    'MIDIBasicInterfaceOut': 'CV-MIDI',
    'MIDICCInterface': 'MIDICCToCVInterface',
    'MIDICCInterfaceOut': 'CV-CC',
    'MIDITriggerInterface': 'MIDITriggerToCVInterface',
    'MIDITriggerInterfaceOut': 'CV-Gate',
    // Polyphony modules
    'PolyMerger': 'Merge',
    'PolySplitter': 'Split',
    'PolySummer': 'Sum'
  };

  /**
   * v2ToMiRackModuleMap is the reverse mapping for V2 -> MiRack conversion.
   * Built from miRackToV2ModuleMap, excluding audio modules.
   */
  var v2ToMiRackModuleMap = (function() {
    var map = {};
    for (var mirack in miRackToV2ModuleMap) {
      if (miRackToV2ModuleMap.hasOwnProperty(mirack)) {
        var v2 = miRackToV2ModuleMap[mirack];
        // Skip audio modules in the reverse map - they're handled specially by merge/split
        if (!isMiRackAudioOutputModule(mirack) && !isMiRackAudioInputModule(mirack)) {
          map[v2] = mirack;
        }
      }
    }
    return map;
  })();

  // ============================================================================
  // Color Conversion Functions
  // ============================================================================

  /**
   * miRackColorIndexToHex converts a MiRack colorIndex to hex string "#rrggbb".
   * @param {number} index - The colorIndex (0-5)
   * @returns {string} Hex color string
   */
  function miRackColorIndexToHex(index) {
    if (index < 0 || index >= miRackColorPalette.length) {
      return '#ffffff'; // Default to white for invalid index
    }
    var c = miRackColorPalette[index];
    return NS.rgbToHex(c.r, c.g, c.b);
  }

  /**
   * rgbToMiRackColorIndex finds the nearest MiRack colorIndex for given RGB values.
   * Uses Euclidean distance in RGB space to find the closest match.
   * @param {number} r - Red component (0-255)
   * @param {number} g - Green component (0-255)
   * @param {number} b - Blue component (0-255)
   * @returns {number} The nearest colorIndex (0-5)
   */
  function rgbToMiRackColorIndex(r, g, b) {
    var bestIndex = 0;
    var bestDistance = 255 * 255 * 3; // Max possible distance

    for (var i = 0; i < miRackColorPalette.length; i++) {
      var c = miRackColorPalette[i];
      // Calculate Euclidean distance in RGB space
      var dr = r - c.r;
      var dg = g - c.g;
      var db = b - c.b;
      var distance = dr * dr + dg * dg + db * db;

      if (distance < bestDistance) {
        bestDistance = distance;
        bestIndex = i;
      }
    }

    return bestIndex;
  }

  /**
   * convertMiRackColorIndexToHex converts MiRack colorIndex to hex during normalization.
   * @param {Object} cable - The cable/wire object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  function convertMiRackColorIndexToHex(cable, issues) {
    // Handle colorIndex field (MiRack-specific)
    if ('colorIndex' in cable) {
      var idx = 0;
      var colorIndex = cable.colorIndex;
      if (typeof colorIndex === 'number') {
        idx = colorIndex | 0; // Convert to integer
      }
      // Convert colorIndex to hex
      cable.color = miRackColorIndexToHex(idx);
      delete cable.colorIndex;
      // Don't log - this is too verbose
    }
    // Also handle "color" field if it contains an integer
    if ('color' in cable) {
      var color = cable.color;
      if (typeof color === 'number') {
        cable.color = miRackColorIndexToHex(color | 0);
      }
    }
  }

  /**
   * convertHexToMiRackColorIndex converts hex to MiRack colorIndex during denormalization.
   * @param {Object} wire - The wire object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  function convertHexToMiRackColorIndex(wire, issues) {
    if (typeof wire.color === 'string') {
      // Parse hex to RGB, then find nearest MiRack colorIndex
      var rgb = NS.hexToRGB(wire.color);
      if (rgb) {
        wire.colorIndex = rgbToMiRackColorIndex(rgb.r, rgb.g, rgb.b);
      }
      delete wire.color;
    }
  }

  // ============================================================================
  // Audio Module Detection Functions
  // ============================================================================

  /**
   * isPolyphonyModule checks if a V2 model is a polyphony module in Fundamental plugin.
   * @param {string} model - The model name
   * @returns {boolean} True if model is Merge, Split, or Sum
   */
  function isPolyphonyModule(model) {
    return model === 'Merge' || model === 'Split' || model === 'Sum';
  }

  /**
   * isMiRackAudioOutputModule checks if a model is a MiRack audio output module.
   * Also recognizes V2 model names after module name mapping.
   * @param {string} model - The model name
   * @returns {boolean} True if model is an audio output module
   */
  function isMiRackAudioOutputModule(model) {
    return model === 'AudioInterface' ||
           model === 'AudioInterface2' ||
           model === 'AudioInterface8' ||
           model === 'AudioInterface16';
  }

  /**
   * isMiRackAudioInputModule checks if a model is a MiRack audio input module.
   * Also recognizes V2 model names after module name mapping.
   * @param {string} model - The model name
   * @returns {boolean} True if model is an audio input module
   */
  function isMiRackAudioInputModule(model) {
    return model === 'AudioInterfaceIn' ||
           model === 'AudioInterfaceIn8' ||
           model === 'AudioInterfaceIn16';
  }

  /**
   * getChannelCountFromAudioModel extracts the channel count from an audio module model name.
   * Returns "2" for default (AudioInterface), or the number from the model name.
   * @param {string} model - The model name
   * @returns {string} "2", "8", or "16"
   */
  function getChannelCountFromAudioModel(model) {
    if (model === 'AudioInterface8' || model === 'AudioInterfaceIn8') {
      return '8';
    }
    if (model === 'AudioInterface16' || model === 'AudioInterfaceIn16') {
      return '16';
    }
    return '2'; // Default to 2-channel for AudioInterface/AudioInterfaceIn
  }

  /**
   * getV2AudioModelName returns the V2 model name for a given channel count.
   * V2 audio modules: AudioInterface (8-ch), AudioInterface2 (2-ch), AudioInterface16 (16-ch).
   * @param {string} channelCount - "2", "8", or "16"
   * @returns {string} The V2 model name
   */
  function getV2AudioModelName(channelCount) {
    if (channelCount === '2') {
      return 'AudioInterface2';
    }
    if (channelCount === '8') {
      return 'AudioInterface';
    }
    if (channelCount === '16') {
      return 'AudioInterface16';
    }
    return 'AudioInterface'; // Default to 8-channel
  }

  /**
   * getMiRackAudioOutputModelName returns the MiRack output model name for a given channel count.
   * @param {string} channelCount - "2", "8", or "16"
   * @returns {string} The MiRack output model name
   */
  function getMiRackAudioOutputModelName(channelCount) {
    if (channelCount === '8') {
      return 'AudioInterface8';
    }
    if (channelCount === '16') {
      return 'AudioInterface16';
    }
    return 'AudioInterface';
  }

  /**
   * getMiRackAudioInputModelName returns the MiRack input model name for a given channel count.
   * @param {string} channelCount - "2", "8", or "16"
   * @returns {string} The MiRack input model name
   */
  function getMiRackAudioInputModelName(channelCount) {
    if (channelCount === '8') {
      return 'AudioInterfaceIn8';
    }
    if (channelCount === '16') {
      return 'AudioInterfaceIn16';
    }
    return 'AudioInterfaceIn';
  }

  // ============================================================================
  // Audio Module Validation
  // ============================================================================

  /**
   * validateAudioModuleCount checks if the patch has valid audio module configuration for MiRack.
   * MiRack only supports ONE audio output and ONE audio input module total (across all channel counts).
   * @param {Array} modules - The modules array
   * @returns {Object} {valid: boolean, skipReason: string}
   */
  function validateAudioModuleCount(modules) {
    if (!Array.isArray(modules)) {
      return {valid: false, skipReason: 'no modules array'};
    }

    var outputCount = 0;
    var inputCount = 0;

    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') {
        continue;
      }

      var model = mod.model;
      if (typeof model === 'string') {
        if (isMiRackAudioOutputModule(model)) {
          outputCount++;
        } else if (isMiRackAudioInputModule(model)) {
          inputCount++;
        }
      }
    }

    if (outputCount > 1) {
      return {valid: false, skipReason: 'patch has ' + outputCount + ' audio output modules (MiRack only supports 1)'};
    }
    if (inputCount > 1) {
      return {valid: false, skipReason: 'patch has ' + inputCount + ' audio input modules (MiRack only supports 1)'};
    }
    return {valid: true, skipReason: ''};
  }

  /**
   * NS.validateMiRackAudioModuleCount is the exposed validation function.
   * @param {Array} modules - The modules array
   * @returns {Object} {valid: boolean, skipReason: string}
   */
  NS.validateMiRackAudioModuleCount = function(modules) {
    return validateAudioModuleCount(modules);
  };

  // ============================================================================
  // Audio Module Pair Finding
  // ============================================================================

  /**
   * findAudioModulePairs finds matching audio input/output module pairs in the modules array.
   * Returns a list of pairs that should be merged.
   * Uses the maximum channel count when input/output have different channel counts.
   * @param {Array} modules - The modules array
   * @returns {Array} Array of pair objects with outputModule, inputModule, indices, IDs, channelCount, v2ModelName
   */
  function findAudioModulePairs(modules) {
    var pairs = [];
    var pairedInputs = {}; // Track input modules already paired by index

    // First pass: find output modules and their matching input modules
    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') {
        continue;
      }

      var model = mod.model;
      if (typeof model !== 'string' || !isMiRackAudioOutputModule(model)) {
        continue;
      }

      var outputID = NS.getInt64(mod, 'id');
      var channelCount = getChannelCountFromAudioModel(model);

      // Look for the matching input module
      var inputMod = null;
      var inputIdx = -1;
      var inputID = 0;

      for (var j = 0; j < modules.length; j++) {
        if (pairedInputs[j]) {
          continue; // Already paired with another output
        }

        var inMod = modules[j];
        if (!inMod || typeof inMod !== 'object') {
          continue;
        }

        var inModel = inMod.model;
        if (typeof inModel !== 'string' || !isMiRackAudioInputModule(inModel)) {
          continue;
        }

        // Pair with any unpaired input module
        // Use the MAXIMUM channel count between output and input
        // This handles mismatched channel counts (e.g., 2-channel out + 8-channel in)
        inputMod = inMod;
        inputIdx = j;
        inputID = NS.getInt64(inMod, 'id');
        pairedInputs[j] = true;
        break;
      }

      // Determine the final channel count for the merged module
      // Use the maximum of output and input channel counts to accommodate both
      var finalChannelCount = channelCount;
      if (inputMod !== null) {
        var inModel = inputMod.model;
        var inChannelCount = getChannelCountFromAudioModel(inModel);
        // Use the larger channel count
        if (inChannelCount === '16' || (inChannelCount === '8' && finalChannelCount !== '16')) {
          finalChannelCount = inChannelCount;
        }
      }

      // Create a pair even if we didn't find a matching input.
      // We'll use key existence in the metadata to track input module existence.
      // This is critical because inputModuleID can be 0, which is a valid ID.
      pairs.push({
        outputModule: mod,
        inputModule: inputMod,
        outputIndex: i,
        inputIndex: inputIdx,
        outputModuleID: outputID,
        inputModuleID: inputID,
        channelCount: finalChannelCount,
        v2ModelName: getV2AudioModelName(finalChannelCount)
      });
    }

    return pairs;
  }

  // ============================================================================
  // Audio Module Merging (MiRack -> V2)
  // ============================================================================

  /**
   * mergeAudioModules merges MiRack's separate audio input/output modules into V2's single module.
   * Called AFTER wire-to-cable conversion in NormalizeMiRack, so it works with module IDs, not indices.
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  function mergeAudioModules(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      return;
    }

    var pairs = findAudioModulePairs(modules);
    if (pairs.length === 0) {
      return; // No audio modules to merge
    }

    // Build mapping for roundtrip (inputModuleID -> outputModuleID)
    // Also track which modules to remove
    var inputToOutput = {};
    var modulesToRemove = {};
    var moduleIDToNewID = {}; // For cable remapping

    for (var pi = 0; pi < pairs.length; pi++) {
      var pair = pairs[pi];

      // Create the merged module
      var mergedModule = {};

      // Copy all properties from output module first
      for (var key in pair.outputModule) {
        if (pair.outputModule.hasOwnProperty(key)) {
          mergedModule[key] = pair.outputModule[key];
        }
      }

      // Set the V2 model name
      mergedModule.model = pair.v2ModelName;

      // Store metadata for roundtrip
      // _mergedAudioModule stores the info needed to split back
      // Use key existence to indicate whether modules existed (handles ID 0 correctly)
      var metadata = {};
      if (pair.outputModule !== null) {
        metadata.outputModuleID = pair.outputModuleID;
      }
      if (pair.inputModule !== null) {
        metadata.inputModuleID = pair.inputModuleID;
      }
      // Store original positions for roundtrip
      if (Array.isArray(pair.outputModule.pos)) {
        metadata.outputModulePos = pair.outputModule.pos.slice();
      }
      if (pair.inputModule !== null && Array.isArray(pair.inputModule.pos)) {
        metadata.inputModulePos = pair.inputModule.pos.slice();
      }
      mergedModule._mergedAudioModule = metadata;

      // Update position to output module's position
      if (Array.isArray(pair.outputModule.pos)) {
        mergedModule.pos = pair.outputModule.pos.slice();
      }

      // Mark modules for removal
      modulesToRemove[pair.outputIndex] = true;
      if (pair.inputModule !== null) {
        modulesToRemove[pair.inputIndex] = true;
      }

      // Store mappings for cable remapping
      // Both input and output cables should reference the merged module (using output module's ID)
      var mergedID = pair.outputModuleID;
      moduleIDToNewID[pair.outputModuleID] = mergedID;
      if (pair.inputModule !== null) {
        moduleIDToNewID[pair.inputModuleID] = mergedID;
        inputToOutput[pair.inputModuleID] = pair.outputModuleID;
      }

      // Replace the output module with the merged module in the array
      modules[pair.outputIndex] = mergedModule;
    }

    // Remove input modules (those marked but not replaced)
    // We need to rebuild the modules array
    var newModules = [];
    for (var mi = 0; mi < modules.length; mi++) {
      if (modulesToRemove[mi]) {
        // Check if this was replaced (output module) or just removed (input)
        var mod = modules[mi];
        if (mod && typeof mod === 'object' && mod._mergedAudioModule) {
          // This is the merged module, keep it
          newModules.push(mod);
        }
        // Otherwise skip (this is the input module being removed)
      } else {
        newModules.push(modules[mi]);
      }
    }
    patch.modules = newModules;

    // Keep _originalIndexToID for roundtrip conversion
    // It contains the mapping from original indices (before merge) to module IDs,
    // which is needed for denormalization

    // Store the mapping for roundtrip splitting
    var inputToOutputKeys = Object.keys(inputToOutput);
    if (inputToOutputKeys.length > 0) {
      patch._audioInputToOutput = inputToOutput;
    }

    // Update cable references (wires are already converted to cables at this point)
    // At this point, cables use module IDs (not array indices)
    if (Array.isArray(patch.cables)) {
      for (var ci = 0; ci < patch.cables.length; ci++) {
        var cable = patch.cables[ci];
        if (!cable || typeof cable !== 'object') {
          continue;
        }

        var outputID = NS.getInt64(cable, 'outputModuleId');
        var inputID = NS.getInt64(cable, 'inputModuleId');

        // Check if this cable's output comes from the input module
        // (AudioInterfaceIn's output going to another module)
        var wasFromInputModuleOutput = false;
        var cableToOutputModule = false;
        for (var pj = 0; pj < pairs.length; pj++) {
          var pair = pairs[pj];
          if (pair.inputModule === null) {
            continue;
          }
          // Check based on original wire indices (stored before conversion)
          var originalOutputIdx = NS.getInt64(cable, '_originalOutputIdx');
          var originalInputIdx = NS.getInt64(cable, '_originalInputIdx');

          // Cable from AudioInterfaceIn's output
          if (originalOutputIdx === pair.inputIndex) {
            wasFromInputModuleOutput = true;
          }
          // Cable going TO AudioInterface's input (from another module)
          if (originalInputIdx === pair.outputIndex && originalOutputIdx !== originalInputIdx) {
            cableToOutputModule = true;
          }
        }

        // Clean up original indices
        delete cable._originalOutputIdx;
        delete cable._originalInputIdx;

        // Update output module ID if it was merged
        var newOutputID = moduleIDToNewID[outputID];
        if (newOutputID !== undefined) {
          cable.outputModuleId = newOutputID;
          // Mark this cable as originating from input module's output
          if (wasFromInputModuleOutput) {
            cable._cableFromInputModule = true;
          }
        }

        // Update input module ID if it was merged
        var newInputID = moduleIDToNewID[inputID];
        if (newInputID !== undefined) {
          cable.inputModuleId = newInputID;
          // Mark this cable as going TO the output module's input (AudioInterface input)
          if (cableToOutputModule) {
            cable._cableToOutputModule = true;
          }
        }
      }
    }
  }

  // ============================================================================
  // Audio Module Splitting (V2 -> MiRack)
  // ============================================================================

  /**
   * detectRequiredChannelCount analyzes cables to determine required audio channel count.
   * Checks input and output ports SEPARATELY, returns max rounded up to available sizes.
   * @param {number} moduleID - The module ID to analyze
   * @param {Object} patch - The patch object
   * @returns {Object} {channelCount: string, error: string}
   */
  function detectRequiredChannelCount(moduleID, patch) {
    if (!Array.isArray(patch.cables)) {
      return {channelCount: '2', error: null}; // Default to 2-channel if no cables
    }

    var maxInputPort = -1;
    var maxOutputPort = -1;

    for (var ci = 0; ci < patch.cables.length; ci++) {
      var cable = patch.cables[ci];
      if (!cable || typeof cable !== 'object') {
        continue;
      }

      var outputModuleID = NS.getInt64(cable, 'outputModuleId');
      var inputModuleID = NS.getInt64(cable, 'inputModuleId');

      // Check output port (cable from this module)
      if (outputModuleID === moduleID) {
        var outputPort = NS.getInt64(cable, 'outputId');
        if (outputPort > maxOutputPort) {
          maxOutputPort = outputPort;
        }
      }

      // Check input port (cable to this module)
      if (inputModuleID === moduleID) {
        var inputPort = NS.getInt64(cable, 'inputId');
        if (inputPort > maxInputPort) {
          maxInputPort = inputPort;
        }
      }
    }

    // Determine required channels from max port numbers
    // Port numbering is 0-based: port 0 = channel 1, port 7 = channel 8
    var requiredChannels = 0;
    if (maxOutputPort >= 0 && maxOutputPort > requiredChannels) {
      requiredChannels = maxOutputPort + 1;
    }
    if (maxInputPort >= 0 && maxInputPort > requiredChannels) {
      requiredChannels = maxInputPort + 1;
    }

    // Default to 2-channel if no cables connected
    if (requiredChannels === 0) {
      return {channelCount: '2', error: null};
    }

    // Round up to available module sizes
    if (requiredChannels <= 2) {
      return {channelCount: '2', error: null};
    } else if (requiredChannels <= 8) {
      return {channelCount: '8', error: null};
    } else if (requiredChannels <= 16) {
      return {channelCount: '16', error: null};
    }

    // Exceeds MiRack's 16-channel limit
    return {channelCount: '', error: 'audio requires ' + requiredChannels + ' channels, exceeds MiRack\'s 16-channel limit'};
  }

  /**
   * updateCablesForSplit updates cable references after splitting audio modules.
   * Cables referencing the merged module are redirected to the appropriate split module.
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Object} moduleIDToOutputID - Mapping from merged module ID to output module ID
   * @param {Object} moduleIDToInputID - Mapping from merged module ID to input module ID
   */
  function updateCablesForSplit(patch, moduleIDToOutputID, moduleIDToInputID) {
    if (!Array.isArray(patch.cables)) {
      return;
    }

    for (var ci = 0; ci < patch.cables.length; ci++) {
      var cable = patch.cables[ci];
      if (!cable || typeof cable !== 'object') {
        continue;
      }

      var oldOutputID = NS.getInt64(cable, 'outputModuleId');
      var oldInputID = NS.getInt64(cable, 'inputModuleId');

      // Check cable markers
      var wasFromInputModule = false;
      if (cable._cableFromInputModule === true) {
        wasFromInputModule = true;
        delete cable._cableFromInputModule;
      }

      var toOutputModule = false;
      if (cable._cableToOutputModule === true) {
        toOutputModule = true;
        delete cable._cableToOutputModule;
      }

      // Handle self-connecting cables (both ends on the same merged module)
      if (oldOutputID === oldInputID) {
        var hasOutputMapping = moduleIDToOutputID.hasOwnProperty(oldOutputID);
        var hasInputMapping = moduleIDToInputID.hasOwnProperty(oldInputID);
        if (hasOutputMapping && hasInputMapping) {
          // Self-connecting cable: route from INPUT module to OUTPUT module
          cable.outputModuleId = moduleIDToInputID[oldInputID];
          cable.inputModuleId = moduleIDToOutputID[oldOutputID];
          continue;
        }
      }

      // Handle cables from the input module (AudioInterfaceIn output -> other modules)
      // These should route from the INPUT module (AudioInterfaceIn), not the OUTPUT module (AudioInterface)
      if (wasFromInputModule) {
        // Check if this output ID maps to an input module (i.e., was from a merged audio module)
        var inputID = moduleIDToInputID[oldOutputID];
        if (inputID !== undefined) {
          cable.outputModuleId = inputID;
        }
      } else {
        // Regular cables or from output module: update normally
        var newOutputID = moduleIDToOutputID[oldOutputID];
        if (newOutputID !== undefined) {
          cable.outputModuleId = newOutputID;
        }
      }

      // Handle cables going TO the output module (other modules -> AudioInterface input)
      // These should route to the OUTPUT module (AudioInterface), not the INPUT module (AudioInterfaceIn)
      if (toOutputModule) {
        // Use the output module ID, not the input module ID
        var outID = moduleIDToOutputID[oldInputID];
        if (outID !== undefined) {
          cable.inputModuleId = outID;
        }
      } else {
        // Regular input side update
        var newInputID = moduleIDToInputID[oldInputID];
        if (newInputID !== undefined) {
          cable.inputModuleId = newInputID;
        }
      }
    }
  }

  /**
   * splitAudioModulesRoundtrip splits merged audio modules using stored metadata.
   * Used when a MiRack patch was converted to V2 and is being converted back.
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  function splitAudioModulesRoundtrip(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      return;
    }

    var newModules = [];
    var moduleIDToOutputID = {};
    var moduleIDToInputID = {};

    for (var mi = 0; mi < modules.length; mi++) {
      var mod = modules[mi];
      if (!mod || typeof mod !== 'object') {
        newModules.push(mod);
        continue;
      }

      var mergedData = mod._mergedAudioModule;
      if (!mergedData || typeof mergedData !== 'object') {
        newModules.push(mod);
        continue;
      }

      // This is a merged module, split it back
      // Use key existence to determine which modules existed (handles ID 0 correctly)
      var hasOutput = mergedData.hasOwnProperty('outputModuleID');
      var hasInput = mergedData.hasOwnProperty('inputModuleID');

      var outputID = NS.getInt64(mergedData, 'outputModuleID');
      var inputID = NS.getInt64(mergedData, 'inputModuleID');
      var mergedID = NS.getInt64(mod, 'id');

      // Determine channel count from model name
      var model = mod.model;
      var channelCount = '2';
      if (model === 'AudioInterface8') {
        channelCount = '8';
      } else if (model === 'AudioInterface16') {
        channelCount = '16';
      }

      // Create output module (if it existed originally)
      var outputModule = null;
      if (hasOutput) {
        outputModule = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key) && key !== '_mergedAudioModule' && key !== 'model' && key !== 'pos') {
            outputModule[key] = mod[key];
          }
        }
        outputModule.id = outputID;
        outputModule.model = getMiRackAudioOutputModelName(channelCount);
        // Restore original position if stored
        if (Array.isArray(mergedData.outputModulePos)) {
          outputModule.pos = mergedData.outputModulePos.slice();
        } else if (Array.isArray(mod.pos)) {
          outputModule.pos = mod.pos.slice();
        }
      }

      // Create input module (if it existed originally)
      // Key existence handles ID 0 correctly - if inputModuleID key exists, module existed
      var inputModule = null;
      if (hasInput) {
        inputModule = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key) && key !== '_mergedAudioModule' && key !== 'model' && key !== 'id' && key !== 'pos') {
            inputModule[key] = mod[key];
          }
        }
        inputModule.id = inputID;
        inputModule.model = getMiRackAudioInputModelName(channelCount);
        // Restore original position if stored
        if (Array.isArray(mergedData.inputModulePos)) {
          inputModule.pos = mergedData.inputModulePos.slice();
        }
      }

      // Store mappings for cable remapping
      if (hasOutput) {
        moduleIDToOutputID[mergedID] = outputID;
      }
      if (hasInput) {
        moduleIDToInputID[mergedID] = inputID;
      }

      // Add modules based on key existence (consistent pattern)
      if (hasOutput) {
        newModules.push(outputModule);
      }
      if (hasInput) {
        newModules.push(inputModule);
      }

      // Clean up metadata
      delete patch._audioInputToOutput;
    }

    patch.modules = newModules;

    // Remove _originalIndexToID since the module indices have changed after splitting
    // DenormalizeV06Style will build a fresh mapping from the new module array
    delete patch._originalIndexToID;

    // Update cable references
    updateCablesForSplit(patch, moduleIDToOutputID, moduleIDToInputID);

    // Rebuild the _idToIndex mapping to reflect the new module array
    // This is needed for DenormalizeV06Style to correctly convert module IDs to array indices
    var newIDToIndex = {};
    for (var ni = 0; ni < newModules.length; ni++) {
      var nm = newModules[ni];
      if (nm && typeof nm === 'object') {
        // Check if "id" key exists and is not null (to distinguish "no ID" from "ID is 0")
        if (nm.hasOwnProperty('id') && nm.id != null) {
          var id = NS.getInt64(nm, 'id');
          if (id >= 0) {
            newIDToIndex[id] = ni;
          }
        }
      }
    }
    patch._idToIndex = newIDToIndex;
  }

  /**
   * splitAudioModulesNative splits V2 audio modules based on cable usage analysis.
   * Used when converting native V2 patches (not from MiRack originally).
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  function splitAudioModulesNative(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      return;
    }

    // Analyze each V2 audio module to determine if it needs splitting
    var newModules = [];
    var moduleIDToOutputID = {};
    var moduleIDToInputID = {};

    for (var mi = 0; mi < modules.length; mi++) {
      var mod = modules[mi];
      if (!mod || typeof mod !== 'object') {
        newModules.push(mod);
        continue;
      }

      var model = mod.model;
      // Handle plain "AudioInterface" (for native V2 patches) and explicit channel variants
      if (model !== 'AudioInterface' && model !== 'AudioInterface2' && model !== 'AudioInterface8' && model !== 'AudioInterface16') {
        newModules.push(mod);
        continue;
      }

      var moduleID = NS.getInt64(mod, 'id');

      // Detect required channel count from cable usage
      var result = detectRequiredChannelCount(moduleID, patch);
      var channelCount = result.channelCount;
      if (result.error) {
        // Store error for later reporting, skip this module
        issues.push('V2 -> MiRack: ' + result.error);
        newModules.push(mod);
        continue;
      }

      // Analyze cable usage to determine which modules to create
      var hasOutput = false;
      var hasInput = false;
      var hasSelfConnection = false;

      if (Array.isArray(patch.cables)) {
        for (var ci = 0; ci < patch.cables.length; ci++) {
          var cable = patch.cables[ci];
          if (!cable || typeof cable !== 'object') {
            continue;
          }

          var outputID = NS.getInt64(cable, 'outputModuleId');
          var inputID = NS.getInt64(cable, 'inputModuleId');

          if (outputID === moduleID && inputID === moduleID) {
            hasSelfConnection = true;
          } else if (outputID === moduleID) {
            hasOutput = true;
          } else if (inputID === moduleID) {
            hasInput = true;
          }
        }
      }

      // Decide which modules to create based on usage
      var createOutput = hasOutput || hasSelfConnection || (!hasInput && !hasOutput);
      var createInput = hasInput || hasSelfConnection;

      var moduleIdx = 0;
      var moduleIdy = 0;
      if (Array.isArray(mod.pos) && mod.pos.length >= 2) {
        if (typeof mod.pos[0] === 'number') {
          moduleIdx = mod.pos[0];
        }
        if (typeof mod.pos[1] === 'number') {
          moduleIdy = mod.pos[1];
        }
      }

      // Generate new IDs (use moduleID for output, moduleID-1 for input)
      var outputID = moduleID;
      var inputID = moduleID - 1;

      if (createOutput && createInput) {
        // Split into two modules
        var outputModule = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key)) {
            outputModule[key] = mod[key];
          }
        }
        outputModule.id = outputID;
        outputModule.model = getMiRackAudioOutputModelName(channelCount);
        outputModule.pos = [moduleIdx, moduleIdy];

        var inputModule = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key)) {
            inputModule[key] = mod[key];
          }
        }
        inputModule.id = inputID;
        inputModule.model = getMiRackAudioInputModelName(channelCount);
        inputModule.pos = [moduleIdx - 3, moduleIdy];

        moduleIDToOutputID[moduleID] = outputID;
        moduleIDToInputID[moduleID] = inputID;

        newModules.push(outputModule);
        newModules.push(inputModule);

      } else if (createOutput) {
        // Only output needed
        var outMod = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key)) {
            outMod[key] = mod[key];
          }
        }
        outMod.id = outputID;
        outMod.model = getMiRackAudioOutputModelName(channelCount);

        moduleIDToOutputID[moduleID] = outputID;
        newModules.push(outMod);

      } else if (createInput) {
        // Only input needed
        var inMod = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key)) {
            inMod[key] = mod[key];
          }
        }
        inMod.id = inputID;
        inMod.model = getMiRackAudioInputModelName(channelCount);

        moduleIDToInputID[moduleID] = inputID;
        newModules.push(inMod);

      } else {
        // Default: create both
        var defOut = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key)) {
            defOut[key] = mod[key];
          }
        }
        defOut.id = outputID;
        defOut.model = getMiRackAudioOutputModelName(channelCount);
        defOut.pos = [moduleIdx, moduleIdy];

        var defIn = {};
        for (var key in mod) {
          if (mod.hasOwnProperty(key)) {
            defIn[key] = mod[key];
          }
        }
        defIn.id = inputID;
        defIn.model = getMiRackAudioInputModelName(channelCount);
        defIn.pos = [moduleIdx - 3, moduleIdy];

        moduleIDToOutputID[moduleID] = outputID;
        moduleIDToInputID[moduleID] = inputID;

        newModules.push(defOut);
        newModules.push(defIn);
      }
    }

    if (newModules.length > 0) {
      patch.modules = newModules;
    }

    // Remove _originalIndexToID since the module indices have changed after splitting
    // DenormalizeV06Style will build a fresh mapping from the new module array
    delete patch._originalIndexToID;

    // Update cable references
    updateCablesForSplit(patch, moduleIDToOutputID, moduleIDToInputID);

    // Rebuild the _idToIndex mapping to reflect the new module array
    // This is needed for DenormalizeV06Style to correctly convert module IDs to array indices
    var newIDToIndex = {};
    for (var ni = 0; ni < newModules.length; ni++) {
      var nm = newModules[ni];
      if (nm && typeof nm === 'object') {
        // Check if "id" key exists and is not null (to distinguish "no ID" from "ID is 0")
        if (nm.hasOwnProperty('id') && nm.id != null) {
          var id = NS.getInt64(nm, 'id');
          if (id >= 0) {
            newIDToIndex[id] = ni;
          }
        }
      }
    }
    patch._idToIndex = newIDToIndex;
  }

  /**
   * splitAudioModules splits V2's single audio module into MiRack's separate input/output modules.
   * Called before module name mapping in DenormalizeMiRack.
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  function splitAudioModules(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      return;
    }

    // Check if we have roundtrip metadata
    var hasRoundtripData = false;
    for (var mi = 0; mi < modules.length; mi++) {
      var mod = modules[mi];
      if (mod && typeof mod === 'object' && mod._mergedAudioModule) {
        hasRoundtripData = true;
        break;
      }
    }

    if (hasRoundtripData) {
      // Roundtrip case: use stored metadata to split exactly back
      return splitAudioModulesRoundtrip(patch, issues);
    }

    // Native V2 case: analyze cables to determine what modules to create
    return splitAudioModulesNative(patch, issues);
  }

  // ============================================================================
  // Main Normalize/Denormalize Functions
  // ============================================================================

  /**
   * NS.normalizeMiRack converts a MiRack patch to the internal v2 format.
   *
   * MiRack-specific behavior:
   * - Module name mappings (MiRack -> V2 model names)
   * - Audio module merging (separate AudioInterface + AudioInterfaceIn -> single AudioInterfaceX)
   * - NO plugin conversion (all modules already use Core plugin)
   * - Array indices -> Module IDs for cables
   * - wires -> cables
   * - paramId -> id in parameters
   * - disabled -> bypass
   * - colorIndex -> hex (for cables)
   *
   * Port of Go NormalizeMiRack().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.normalizeMiRack = function(patch, issues) {
    var modules = NS.getModules(patch);
    if (!modules) {
      issues.push('MiRack normalization: no modules found');
      return;
    }

    // Pass 1: Build index-to-ID mapping BEFORE any module modifications
    // This is critical because mergeAudioModules will remove modules from the array,
    // which would cause wire indices to point to wrong modules.
    var indexToID = {};
    var nextID = 0;

    for (var i = 0; i < modules.length; i++) {
      var mod = modules[i];
      if (!mod || typeof mod !== 'object') {
        continue;
      }

      // Get or assign module ID
      var moduleID;
      if ('id' in mod) {
        moduleID = NS.getInt64(mod, 'id');
        if (moduleID < 0) {
          moduleID = nextID;
          nextID++;
          mod.id = moduleID;
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
    }
    // Store for later use
    patch._originalIndexToID = indexToID;

    // Pass 2: Convert wires to cables using the ORIGINAL module array
    // This must happen BEFORE module merging, because merging removes modules
    // and would cause wire indices to become invalid.
    if ('wires' in patch) {
      patch.cables = patch.wires;
      delete patch.wires;

      var cables = patch.cables;
      if (Array.isArray(cables)) {
        var validCables = [];
        for (var ci = 0; ci < cables.length; ci++) {
          var cable = cables[ci];
          if (!cable || typeof cable !== 'object') {
            continue;
          }

          // Get wire indices (these are array indices in the ORIGINAL array)
          var outputModuleIdx = NS.getInt64(cable, 'outputModuleId');
          var inputModuleIdx = NS.getInt64(cable, 'inputModuleId');

          // Convert array indices to module IDs using ORIGINAL mapping
          var outputModuleID = indexToID[outputModuleIdx];
          var inputModuleID = indexToID[inputModuleIdx];

          if (outputModuleID === undefined || inputModuleID === undefined) {
            continue;
          }

          cable.outputModuleId = outputModuleID;
          cable.inputModuleId = inputModuleID;

          // Store original wire indices for later use in merge
          cable._originalOutputIdx = outputModuleIdx;
          cable._originalInputIdx = inputModuleIdx;

          // Apply color conversion
          convertMiRackColorIndexToHex(cable, issues);

          validCables.push(cable);
        }
        patch.cables = validCables;
      }
    }

    // Pass 3: Merge separate audio input/output modules into single V2 audio module
    // Now works with cables (using module IDs) instead of wires (using indices)
    mergeAudioModules(patch, issues);

    // Pass 4: Apply MiRack -> V2 module name mappings (for non-audio modules)
    // Polyphony modules (Merge, Split, Sum) also need plugin -> Fundamental
    // Skip audio modules - they're already handled by the merge logic
    modules = NS.getModules(patch); // Re-fetch after merge
    if (Array.isArray(modules)) {
      for (var mi = 0; mi < modules.length; mi++) {
        var mod = modules[mi];
        if (!mod || typeof mod !== 'object') {
          continue;
        }
        var model = mod.model;
        if (typeof model !== 'string') {
          continue;
        }

        // Skip audio modules - they're already merged with correct names
        if (isMiRackAudioOutputModule(model) || isMiRackAudioInputModule(model)) {
          continue;
        }

        var v2Model = miRackToV2ModuleMap[model];
        if (v2Model) {
          mod.model = v2Model;
          // Polyphony modules are in Fundamental plugin in V2, not Core
          if (isPolyphonyModule(v2Model)) {
            mod.plugin = 'Fundamental';
          }
        }
      }
    }

    // Pass 5: Complete remaining V06-style normalization
    // We've already handled wires->cables and module IDs, so we just need:
    // - paramId->id conversion
    // - disabled->bypass conversion
    // - Remove format-specific fields
    if (Array.isArray(modules)) {
      for (var i = 0; i < modules.length; i++) {
        var mod = modules[i];
        if (!mod || typeof mod !== 'object') {
          continue;
        }
        // Convert paramId to id in parameters
        NS.convertParamIDToID(mod, i, issues);
        // Convert disabled to bypass (v2 format)
        NS.convertDisabledToBypass(mod, issues);
        // Remove format-specific fields not used in v2
        delete mod.sumPolyInputs;
      }
    }

    // Store expander links (leftModuleId/rightModuleId) for V2 roundtrip.
    var expanderLinks = {};
    var hasExpanderLinks = false;
    for (var ej = 0; ej < modules.length; ej++) {
      var em = modules[ej];
      if (!em || typeof em !== 'object') {
        continue;
      }
      var id = NS.getInt64(em, 'id');
      if (id >= 0) {
        var links = {};
        if ('leftModuleId' in em && em.leftModuleId != null) {
          links.leftModuleId = NS.getInt64(em, 'leftModuleId');
        }
        if ('rightModuleId' in em && em.rightModuleId != null) {
          links.rightModuleId = NS.getInt64(em, 'rightModuleId');
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

    // Ensure version is set
    patch.version = '2.6.6';

    // Pass 6: Handle Notes module text field conversion
    // MiRack stores notes text in module-level "text" field
    // V2 stores notes text in data.text field
    if (Array.isArray(modules)) {
      for (var ni = 0; ni < modules.length; ni++) {
        var mod = modules[ni];
        if (!mod || typeof mod !== 'object') {
          continue;
        }
        if (mod.model === 'Notes') {
          // Move module-level "text" to "data.text" for V2 format
          if (typeof mod.text === 'string') {
            if (!mod.data || typeof mod.data !== 'object') {
              mod.data = {};
            }
            mod.data.text = mod.text;
            delete mod.text;
          }
        }
      }
    }
  };

  /**
   * NS.denormalizeMiRack converts the internal v2 format to MiRack format.
   *
   * MiRack-specific behavior:
   * - Module name mappings (V2 -> MiRack model names)
   * - Audio module splitting (single AudioInterfaceX -> separate AudioInterface + AudioInterfaceInX)
   * - NO plugin conversion (all modules stay Core, NOT Fundamental!)
   * - Module IDs -> Array indices for cables
   * - cables -> wires
   * - bypass -> disabled
   * - id -> paramId in parameters
   * - hex -> colorIndex (for cables)
   *
   * Port of Go DenormalizeMiRack().
   *
   * @param {Object} patch - The patch object (mutated in place)
   * @param {Array} issues - Array to push warning messages to
   */
  NS.denormalizeMiRack = function(patch, issues) {
    // Pass 1: Split V2's single audio module into MiRack's separate input/output modules
    // This must happen BEFORE V06-style denormalization so cable references are correct
    splitAudioModules(patch, issues);

    // Pass 2: Standard V06-style denormalization
    var config = {
      formatName: 'MiRack',
      hasFundamental: false,
      convertColor: convertHexToMiRackColorIndex,
      normalizePlugin: function(plugin, model) {
        return {plugin: plugin, modified: false};
      },
      denormalizePlugin: function(plugin, model) {
        return {plugin: plugin, modified: false};
      }
    };
    NS.denormalizeV06Style(patch, config, issues);

    // Pass 3: Apply V2 -> MiRack module name mappings
    // Polyphony modules also need plugin -> Core (MiRack doesn't have Fundamental)
    var modules = NS.getModules(patch);
    if (Array.isArray(modules)) {
      for (var i = 0; i < modules.length; i++) {
        var mod = modules[i];
        if (!mod || typeof mod !== 'object') {
          continue;
        }
        var model = mod.model;
        if (typeof model !== 'string') {
          continue;
        }

        var mirackModel = v2ToMiRackModuleMap[model];
        if (mirackModel) {
          mod.model = mirackModel;
          // Polyphony modules are in Core plugin in MiRack, not Fundamental
          if (isPolyphonyModule(model)) {
            mod.plugin = 'Core';
          }
        }
      }
    }

    // Pass 4: Handle Notes module text field conversion
    // V2 stores notes text in data.text field
    // MiRack stores notes text in module-level "text" field
    if (Array.isArray(modules)) {
      for (var ni = 0; ni < modules.length; ni++) {
        var mod = modules[ni];
        if (!mod || typeof mod !== 'object') {
          continue;
        }
        if (mod.model === 'Notes') {
          // Move "data.text" to module-level "text" for MiRack format
          if (mod.data && typeof mod.data === 'object' && typeof mod.data.text === 'string') {
            mod.text = mod.data.text;
            delete mod.data.text;
            // Remove data object if it's now empty
            var dataKeys = Object.keys(mod.data);
            if (dataKeys.length === 0) {
              delete mod.data;
            }
          }
        }
      }
    }

    // Clean up internal markers from wires/cables
    if (Array.isArray(patch.wires)) {
      for (var wi = 0; wi < patch.wires.length; wi++) {
        var wire = patch.wires[wi];
        if (wire && typeof wire === 'object') {
          delete wire._fromInputModuleOutput;
          delete wire._fromInputModuleInput;
        }
      }
    }
  };

})(window.VRackConverter);
