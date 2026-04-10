/**
 * app.js - UI Controller for VRackConverter
 */
window.VRackConverter = window.VRackConverter || {};

(function(NS) {
'use strict';

var currentInput = {
  data: null,
  filename: null,
  format: null,
  patch: null
};

var lastResult = null;

// DOM elements
var dropZone, fileInput, inputInfo, formatBadge, fileName, patchSummary;
var convertSection, sourceLabel, targetFormat, convertBtn, metamoduleCheckbox;
var outputSection, resultBadge, resultSummary, issuesPanel, issuesList, downloadBtn;
var errorSection, errorMessage;

document.addEventListener('DOMContentLoaded', function() {
  // Cache DOM elements
  dropZone = document.getElementById('drop-zone');
  fileInput = document.getElementById('file-input');
  inputInfo = document.getElementById('input-info');
  formatBadge = document.getElementById('format-badge');
  fileName = document.getElementById('file-name');
  patchSummary = document.getElementById('patch-summary');
  convertSection = document.getElementById('convert-section');
  sourceLabel = document.getElementById('source-label');
  targetFormat = document.getElementById('target-format');
  convertBtn = document.getElementById('convert-btn');
  metamoduleCheckbox = document.getElementById('metamodule-checkbox');
  outputSection = document.getElementById('output-section');
  resultBadge = document.getElementById('result-badge');
  resultSummary = document.getElementById('result-summary');
  issuesPanel = document.getElementById('issues-panel');
  issuesList = document.getElementById('issues-list');
  downloadBtn = document.getElementById('download-btn');
  errorSection = document.getElementById('error-section');
  errorMessage = document.getElementById('error-message');

  // Event listeners
  dropZone.addEventListener('click', function() { fileInput.click(); });
  dropZone.addEventListener('keydown', function(e) { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileInput.click(); } });

  fileInput.addEventListener('change', function(e) {
    if (e.target.files && e.target.files.length > 0) {
      handleFile(e.target.files[0]);
    }
  });

  // Drag and drop
  dropZone.addEventListener('dragover', function(e) { e.preventDefault(); e.stopPropagation(); dropZone.classList.add('dragover'); });
  dropZone.addEventListener('dragleave', function(e) { e.preventDefault(); e.stopPropagation(); dropZone.classList.remove('dragover'); });
  dropZone.addEventListener('drop', function(e) {
    e.preventDefault(); e.stopPropagation();
    dropZone.classList.remove('dragover');

    // Try directory drop (webkitGetAsEntry for .mrk directories)
    if (e.dataTransfer.items && e.dataTransfer.items.length > 0) {
      var item = e.dataTransfer.items[0];
      if (item.webkitGetAsEntry) {
        var entry = item.webkitGetAsEntry();
        if (entry && entry.isDirectory) {
          handleDirectoryDrop(entry);
          return;
        }
      }
    }

    // Regular file drop (includes .zip files)
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      var file = e.dataTransfer.files[0];
      // If the dropped file is a .zip, handle it as a zip
      if (file.name.toLowerCase().endsWith('.zip')) {
        handleFile(file);
        return;
      }
      handleFile(file);
    }
  });

  targetFormat.addEventListener('change', updateConvertButton);
  convertBtn.addEventListener('click', doConvert);
  downloadBtn.addEventListener('click', doDownload);
});

function handleDirectoryDrop(dirEntry) {
  // Read patch.vcv from the dropped .mrk directory
  var reader = dirEntry.createReader();
  reader.readEntries(function(entries) {
    var patchEntry = null;
    for (var i = 0; i < entries.length; i++) {
      if (entries[i].name === 'patch.vcv' && entries[i].isFile) {
        patchEntry = entries[i];
        break;
      }
    }
    if (!patchEntry) {
      showError('No patch.vcv found in the dropped directory. Is this a valid .mrk bundle?');
      return;
    }
    patchEntry.file(function(file) {
      // Use the directory name as the filename for format detection
      var overriddenName = dirEntry.name + '/patch.vcv';
      handleFile(file, overriddenName);
    });
  }, function(err) {
    showError(
      'Cannot read .mrk folder directly when opened from file://. ' +
      'Zip the .mrk folder and drop the .zip file instead, ' +
      'or serve this app via a local HTTP server (e.g. python3 -m http.server).'
    );
  });
}

function handleFile(file, overrideName) {
  var name = overrideName || file.name;

  // Handle .zip files (e.g. zipped .mrk folder)
  if (name.toLowerCase().endsWith('.zip')) {
    var zipReader = new FileReader();
    zipReader.onload = function(e) {
      var zipData = new Uint8Array(e.target.result);
      try {
        var extracted = extractPatchFromZip(zipData);
        if (!extracted) {
          showError('No patch.vcv found inside the .zip file. Make sure the .zip contains an .mrk folder with patch.vcv inside.');
          return;
        }
        processPatchData(extracted.data, extracted.name);
      } catch(err) {
        showError('Failed to read .zip file: ' + (err.message || err));
      }
    };
    zipReader.readAsArrayBuffer(file);
    return;
  }

  var reader = new FileReader();
  reader.onload = function(e) {
    var data = new Uint8Array(e.target.result);
    processPatchData(data, name);
  };
  reader.readAsArrayBuffer(file);
}

/**
 * Extract patch.vcv from a zip file. Looks for patch.vcv at root,
 * inside a single top-level directory, or anywhere in the archive.
 */
function extractPatchFromZip(zipData) {
  var fflate = window.fflate;
  if (!fflate) throw new Error('Zip support library not loaded');

  var files = fflate.unzipSync(zipData);

  // Strategy 1: patch.vcv at root
  if (files['patch.vcv']) {
    return { data: files['patch.vcv'], name: 'patch.mrk/patch.vcv' };
  }

  // Strategy 2: patch.vcv inside a single top-level directory (e.g. "mymrack/patch.vcv")
  for (var path in files) {
    if (files.hasOwnProperty(path)) {
      var parts = path.split('/');
      if (parts.length === 2 && parts[1] === 'patch.vcv') {
        return { data: files[path], name: parts[0] + '.mrk/patch.vcv' };
      }
    }
  }

  // Strategy 3: find patch.vcv anywhere
  for (var path2 in files) {
    if (files.hasOwnProperty(path2)) {
      var segments = path2.split('/');
      if (segments[segments.length - 1] === 'patch.vcv') {
        return { data: files[path2], name: segments[0] + '.mrk/patch.vcv' };
      }
    }
  }

  return null;
}

function processPatchData(data, name) {

    // Detect format
    var format = NS.detectFormat(name, data);

    // Try to parse and get more info
    var patch = null;
    try {
      var jsonBytes = NS.readPatchData(data, format);
      patch = NS.fromJSON(jsonBytes);
    } catch(ex) {
      // Can't parse, but still show format
    }

    currentInput = { data: data, filename: name, format: format, patch: patch };

    // Update UI
    showInputInfo(name, format, patch);
    showConvertSection(format);
    hideOutput();
    hideError();
}

function showInputInfo(name, format, patch) {
  inputInfo.classList.remove('hidden');

  var badgeClass = format.replace('v0.6', 'v06');
  formatBadge.textContent = NS.getFormatDisplayName(format);
  formatBadge.className = 'format-badge ' + badgeClass;

  fileName.textContent = name;

  if (patch) {
    var moduleCount = Array.isArray(patch.modules) ? patch.modules.length : 0;
    var cables = patch.cables || patch.wires;
    var cableCount = Array.isArray(cables) ? cables.length : 0;
    var cableWord = cableCount === 1 ? 'cable' : 'cables';
    patchSummary.textContent = moduleCount + ' modules, ' + cableCount + ' ' + cableWord;
  } else {
    patchSummary.textContent = '';
  }
}

function showConvertSection(sourceFormat) {
  convertSection.classList.remove('hidden');
  sourceLabel.textContent = NS.getFormatDisplayName(sourceFormat);

  // Populate target formats
  var targets = NS.getSupportedTargets(sourceFormat);
  targetFormat.innerHTML = '<option value="">Select target format...</option>';
  for (var i = 0; i < targets.length; i++) {
    var opt = document.createElement('option');
    opt.value = targets[i];
    opt.textContent = NS.getFormatDisplayName(targets[i]);
    targetFormat.appendChild(opt);
  }

  // Auto-select first target
  if (targets.length > 0) {
    targetFormat.value = targets[0];
  }

  updateConvertButton();
}

function updateConvertButton() {
  convertBtn.disabled = !targetFormat.value;
  // Only show MetaModule option when target is VCV v2
  var optionsRow = metamoduleCheckbox.closest('.options-row');
  if (optionsRow) {
    if (targetFormat.value === NS.FORMAT_V2) {
      optionsRow.classList.remove('hidden');
    } else {
      optionsRow.classList.add('hidden');
      metamoduleCheckbox.checked = false;
    }
  }
}

function doConvert() {
  if (!currentInput.data || !targetFormat.value) return;

  var result = NS.convertPatch(currentInput.data, currentInput.filename, targetFormat.value, {
    metamodule: metamoduleCheckbox.checked,
    sourceFormat: currentInput.format
  });

  lastResult = result;

  if (result.error) {
    showError(result.error);
    outputSection.classList.add('hidden');
    return;
  }

  if (result.skipped) {
    showError('Patch is already in the target format. No conversion needed.');
    outputSection.classList.add('hidden');
    return;
  }

  // Show output
  outputSection.classList.remove('hidden');
  errorSection.classList.add('hidden');

  var badgeClass = result.targetFormat.replace('v0.6', 'v06');
  resultBadge.textContent = NS.getFormatDisplayName(result.targetFormat);
  resultBadge.className = 'format-badge ' + badgeClass;

  var outPatch = null;
  try { outPatch = NS.fromJSON(result.outputData); } catch(e) {}
  if (outPatch) {
    var mc = Array.isArray(outPatch.modules) ? outPatch.modules.length : 0;
    var cc = Array.isArray(outPatch.cables || outPatch.wires) ? (outPatch.cables || outPatch.wires).length : 0;
    resultSummary.textContent = mc + ' modules, ' + cc + ' cables';
  }

  // Show issues
  if (result.issues && result.issues.length > 0) {
    issuesPanel.classList.remove('hidden');
    issuesList.innerHTML = '';
    for (var i = 0; i < result.issues.length; i++) {
      var li = document.createElement('li');
      li.textContent = result.issues[i];
      issuesList.appendChild(li);
    }
  } else {
    issuesPanel.classList.add('hidden');
  }
}

function doDownload() {
  if (!lastResult || !lastResult.outputData) return;

  var blob = new Blob([lastResult.outputData], { type: 'application/octet-stream' });
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url;
  a.download = lastResult.outputFilename || 'converted.vcv';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function showError(msg) {
  errorSection.classList.remove('hidden');
  errorMessage.textContent = msg;
}

function hideError() {
  errorSection.classList.add('hidden');
}

function hideOutput() {
  outputSection.classList.add('hidden');
  lastResult = null;
}

})(window.VRackConverter);
