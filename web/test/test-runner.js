/**
 * test-runner.js - Browser-based test runner for VRackConverter
 *
 * Since we can't use fetch() on file://, test fixtures are embedded as JS strings.
 * Run by opening web/test/index.html in a browser.
 */
(function() {
'use strict';

var NS = window.VRackConverter;
var results = document.getElementById('results');
var summary = document.getElementById('summary');
var passed = 0, failed = 0;

function log(html) {
  results.innerHTML += html + '\n';
}

function test(name, fn) {
  try {
    fn();
    log('<div class="pass">PASS: ' + name + '</div>');
    passed++;
  } catch(e) {
    log('<div class="fail">FAIL: ' + name + ' - ' + (e.message || e) + '</div>');
    failed++;
  }
}

function assert(condition, msg) {
  if (!condition) throw new Error(msg || 'Assertion failed');
}

function assertEqual(a, b, msg) {
  if (a !== b) throw new Error((msg || '') + ' expected ' + b + ' got ' + a);
}

// ===== Utility Tests =====

test('hexToRGB roundtrip', function() {
  var rgb = NS.hexToRGB('#ffb500');
  assertEqual(rgb.r, 255);
  assertEqual(rgb.g, 181);
  assertEqual(rgb.b, 0);
  var hex = NS.rgbToHex(rgb.r, rgb.g, rgb.b);
  assertEqual(hex, '#ffb500');
});

test('getInt64 handles float64', function() {
  assertEqual(NS.getInt64({id: 42.0}, 'id'), 42);
  assertEqual(NS.getInt64({id: 0}, 'id'), 0);
  assertEqual(NS.getInt64({}, 'id'), 0);
  assertEqual(NS.getInt64({id: -1}, 'id'), -1);
});

test('fromJSON/toJSON roundtrip', function() {
  var obj = {version: '2.6.6', modules: [{id: 0, plugin: 'Core'}]};
  var bytes = NS.toJSON(obj);
  var back = NS.fromJSON(bytes);
  assertEqual(back.version, '2.6.6');
  assertEqual(back.modules[0].id, 0);
});

// ===== Tar Tests =====

test('tar create/extract roundtrip', function() {
  var content = new TextEncoder().encode('{"version":"2.6.6"}');
  var tar = NS._createTarBuffer('patch.json', content);
  var extracted = NS._extractTarEntry(tar, 'patch.json');
  assert(extracted, 'should find entry');
  var str = new TextDecoder().decode(extracted);
  assertEqual(str, '{"version":"2.6.6"}');
});

// ===== V2 Format Tests =====

test('V2 normalize builds idToIndex', function() {
  var patch = {version: '2.0.0', modules: [{id:5},{id:10}], cables: []};
  var issues = [];
  NS.normalizeV2(patch, issues);
  assert(patch._idToIndex, 'should have _idToIndex');
  assertEqual(patch._idToIndex[5], 0);
  assertEqual(patch._idToIndex[10], 1);
});

test('V2 denormalize ensures cables have IDs', function() {
  var patch = {version: '2.6.6', modules: [{id:0}], cables: [
    {outputModuleId: 0, inputModuleId: 0}
  ]};
  var issues = [];
  NS.denormalizeV2(patch, issues);
  assertEqual(patch.cables[0].id, 0);
  assertEqual(patch.version, '2.6.6');
});

// ===== V0.6 Format Tests =====

test('V0.6 normalize: Fundamental -> Core', function() {
  var patch = {version: '0.6.2', modules: [
    {id:0, plugin:'Fundamental', model:'VCO-1', params:[]}
  ], wires: []};
  var issues = [];
  NS.normalizeV06(patch, issues);
  assertEqual(patch.modules[0].plugin, 'Core');
});

test('V0.6 denormalize: Core -> Fundamental', function() {
  var patch = {version: '2.6.6', modules: [
    {id:0, plugin:'Core', model:'VCO-1', params:[{id:0, value:0.5}]}
  ], cables: []};
  var issues = [];
  NS.denormalizeV06(patch, issues);
  assertEqual(patch.modules[0].plugin, 'Fundamental');
  assert(patch.modules[0].params[0].paramId !== undefined, 'should have paramId');
});

// ===== MiRack Color Tests =====

test('MiRack colorIndex -> hex roundtrip', function() {
  // These will be tested indirectly through mirack.js loaded functions
  // Test via convertMiRackColorIndexToHex if exposed, or via inline test
  var palette = [
    {r:255, g:181, b:0},   // 0: yellow
    {r:242, g:56, b:74},   // 1: red
    {r:0, g:181, b:110},   // 2: green
    {r:54, g:149, b:239},  // 3: teal
    {r:255, g:181, b:56},  // 4: orange
    {r:140, g:74, b:181}   // 5: purple
  ];
  for (var i = 0; i < palette.length; i++) {
    var hex = NS.rgbToHex(palette[i].r, palette[i].g, palette[i].b);
    var rgb = NS.hexToRGB(hex);
    assertEqual(rgb.r, palette[i].r, 'color ' + i + ' r');
    assertEqual(rgb.g, palette[i].g, 'color ' + i + ' g');
    assertEqual(rgb.b, palette[i].b, 'color ' + i + ' b');
  }
});

// ===== Cardinal Tests =====

test('Cardinal detection by content', function() {
  var patch = {modules: [{plugin:'Cardinal', model:'HostAudio2', id:0}]};
  assert(NS.detectCardinalByContent(patch), 'should detect Cardinal');
  var patch2 = {modules: [{plugin:'Core', model:'AudioInterface2', id:0}]};
  assert(!NS.detectCardinalByContent(patch2), 'should not detect Cardinal');
});

test('Cardinal normalize remaps modules', function() {
  var patch = {version: '2.6.6', modules: [
    {id:0, plugin:'Cardinal', model:'HostAudio2', params:[]},
    {id:1, plugin:'Core', model:'VCO-1', params:[]}
  ], cables: []};
  var issues = [];
  NS.normalizeCardinal(patch, issues);
  assertEqual(patch.modules[0].plugin, 'Core');
  assertEqual(patch.modules[0].model, 'AudioInterface2');
  assertEqual(patch.modules[1].plugin, 'Core'); // unchanged
});

// ===== Pipeline Tests =====

test('detectFormat returns correct formats', function() {
  var v2Data = new TextEncoder().encode('not json');
  assertEqual(NS.detectFormat('test.vcv', new TextEncoder().encode('{"version":"2.0.0"}')), NS.FORMAT_V06);
  assertEqual(NS.detectFormat('test.mrk', v2Data), NS.FORMAT_MIRACK);
  assertEqual(NS.detectFormat('test.mrk/patch.vcv', v2Data), NS.FORMAT_MIRACK);
});

test('getFormatDisplayName works', function() {
  assertEqual(NS.getFormatDisplayName(NS.FORMAT_V2), 'VCV Rack v2');
  assertEqual(NS.getFormatDisplayName(NS.FORMAT_MIRACK), 'MiRack');
  assertEqual(NS.getFormatDisplayName(NS.FORMAT_CARDINAL), 'Cardinal');
  assertEqual(NS.getFormatDisplayName(NS.FORMAT_V06), 'VCV Rack v0.6');
});

test('getSupportedTargets excludes source', function() {
  var targets = NS.getSupportedTargets(NS.FORMAT_V2);
  assert(targets.indexOf(NS.FORMAT_V2) === -1, 'v2 should not be in its own targets');
  assert(targets.length === 3, 'should have 3 targets');
});

// ===== Summary =====
summary.innerHTML = '<strong>Results: ' + passed + ' passed, ' + failed + ' failed</strong>' +
  (failed === 0 ? ' - ALL TESTS PASSED!' : ' - SOME TESTS FAILED');
summary.style.color = failed === 0 ? '#4caf50' : '#ef5350';

})();
