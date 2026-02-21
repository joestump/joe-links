#!/usr/bin/env node
/**
 * Build documentation content
 *
 * Orchestrates the transformation of OpenSpecs and ADRs
 * into Docusaurus-compatible MDX files, then copies any
 * hand-written static docs into the generated output.
 */

const fs = require('fs');
const path = require('path');

console.log('Building documentation content...\n');

// Build spec mapping first (needed by transforms)
require('./build-spec-mapping');

// Transform OpenSpecs
require('./transform-openspecs');

// Transform ADRs
require('./transform-adrs');

// Generate index page
require('./generate-index');

// Copy static (hand-written) docs into docs-generated
function copyStaticDocs() {
  const staticDir = path.join(__dirname, '../docs-static');
  const destDir = path.join(__dirname, '../../docs-generated');
  if (!fs.existsSync(staticDir)) return;
  fs.cpSync(staticDir, destDir, { recursive: true });
  console.log('  Copied static docs');
}

console.log('Copying static docs...');
copyStaticDocs();

console.log('\nDocumentation content build complete!');
