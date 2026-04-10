#!/usr/bin/env node
/**
 * build.js - Bundle all JS, CSS, and libraries into a single self-contained HTML file.
 *
 * Usage: node build.js
 * Output: build/vrackconverter.html
 */

var fs = require('fs');
var path = require('path');

var WEB_DIR = path.join(__dirname, 'web');
var BUILD_DIR = path.join(__dirname, 'build');

// Files to inline in order
var JS_FILES = [
  'js/lib/fzstd.js',
  'js/lib/fflate.js',
  'js/patch.js',
  'js/archive.js',
  'js/formats/format-common.js',
  'js/formats/format-v2.js',
  'js/formats/format-v06.js',
  'js/formats/format-mirack.js',
  'js/formats/format-cardinal.js',
  'js/converter.js',
  'js/app.js'
];

var CSS_FILE = 'css/style.css';

// Read index.html template
var indexHtml = fs.readFileSync(path.join(WEB_DIR, 'index.html'), 'utf8');

// Read CSS
var css = fs.readFileSync(path.join(WEB_DIR, CSS_FILE), 'utf8');

// Read and concatenate all JS files
var jsParts = [];
for (var i = 0; i < JS_FILES.length; i++) {
  var filePath = path.join(WEB_DIR, JS_FILES[i]);
  var content = fs.readFileSync(filePath, 'utf8');
  jsParts.push('// --- ' + JS_FILES[i] + ' ---\n' + content);
}
var allJs = jsParts.join('\n\n');

// Build single HTML: replace <link> and <script> tags with inlined content
var output = indexHtml;

// Replace CSS link with inline style
output = output.replace(
  /<link rel="stylesheet" href="css\/style\.css">/,
  '<style>\n' + css + '\n</style>'
);

// Replace all script tags with a single inline script
var scriptPattern = /<!-- Dependencies -->[\s\S]*?<script src="js\/app\.js"><\/script>/;
var inlineScript = '<script>\n' + allJs + '\n</script>';
output = output.replace(scriptPattern, inlineScript);

// Ensure build directory exists
if (!fs.existsSync(BUILD_DIR)) {
  fs.mkdirSync(BUILD_DIR, { recursive: true });
}

// Write output
var outputPath = path.join(BUILD_DIR, 'vrackconverter.html');
fs.writeFileSync(outputPath, output);

var sizeKB = Math.round(fs.statSync(outputPath).size / 1024);
console.log('Built: ' + outputPath + ' (' + sizeKB + ' KB)');
console.log('Open this file directly in a browser - no server needed.');
