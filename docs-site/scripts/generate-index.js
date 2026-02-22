#!/usr/bin/env node
/**
 * Generate Index Page
 *
 * Creates a landing page (index.mdx) for the docs site that links
 * to ADR and spec sections with counts.
 */

const fs = require('fs');
const path = require('path');

const ADRS_SOURCE = path.join(__dirname, '../../docs/adrs');
const SPECS_SOURCE = path.join(__dirname, '../../docs/openspec/specs');
const STATIC_DOCS = path.join(__dirname, '../docs-static/guides');
const DOCS_DEST = path.join(__dirname, '../../docs-generated');

// Read project title from docusaurus.config.ts
const configPath = path.join(__dirname, '../docusaurus.config.ts');
let projectTitle = 'Architecture Documentation';
if (fs.existsSync(configPath)) {
  const configContent = fs.readFileSync(configPath, 'utf-8');
  const titleMatch = configContent.match(/PROJECT_TITLE\s*=\s*['"]([^'"]+)['"]/);
  if (titleMatch) projectTitle = titleMatch[1];
}

function countAdrs() {
  if (!fs.existsSync(ADRS_SOURCE)) return 0;
  return fs.readdirSync(ADRS_SOURCE)
    .filter(f => f.endsWith('.md') && f !== '0000-template.md' && f !== 'README.md')
    .length;
}

function countSpecs() {
  if (!fs.existsSync(SPECS_SOURCE)) return 0;
  return fs.readdirSync(SPECS_SOURCE)
    .filter(d => {
      const dirPath = path.join(SPECS_SOURCE, d);
      return fs.statSync(dirPath).isDirectory() && fs.existsSync(path.join(dirPath, 'spec.md'));
    })
    .length;
}

function generateSpecsIndex() {
  if (!fs.existsSync(SPECS_SOURCE)) return;

  const specsDest = path.join(DOCS_DEST, 'specs');
  fs.mkdirSync(specsDest, { recursive: true });

  const domains = fs.readdirSync(SPECS_SOURCE)
    .filter(d => fs.statSync(path.join(SPECS_SOURCE, d)).isDirectory())
    .sort();

  const rows = [];
  for (const domain of domains) {
    const domainPath = path.join(SPECS_SOURCE, domain);
    const hasSpec = fs.existsSync(path.join(domainPath, 'spec.md'));
    const hasDesign = fs.existsSync(path.join(domainPath, 'design.md'));

    if (!hasSpec && !hasDesign) continue;

    // Extract title from spec.md H1 heading, stripping SPEC-XXXX: prefix
    let label = domain.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
    if (hasSpec) {
      const content = fs.readFileSync(path.join(domainPath, 'spec.md'), 'utf-8');
      const titleMatch = content.match(/^#\s+SPEC-\d+:\s+(.+)$/m);
      if (titleMatch) label = titleMatch[1].trim();
    }

    let docs;
    if (hasSpec && hasDesign) {
      docs = `[Specification](./${domain}/spec) / [Design](./${domain}/design)`;
    } else if (hasSpec) {
      docs = `[Specification](./${domain})`;
    } else {
      docs = `[Design](./${domain})`;
    }

    rows.push(`| ${label} | ${docs} |`);
  }

  if (rows.length === 0) return;

  const content = `---
title: "Specifications"
sidebar_label: "Overview"
sidebar_position: 0
---

# Specifications

| Component | Documents |
|-----------|-----------|
${rows.join('\n')}
`;

  fs.writeFileSync(path.join(specsDest, 'index.mdx'), content);
  console.log('  Generated specs index page');
}

function hasStaticGuides() {
  if (!fs.existsSync(STATIC_DOCS)) return false;
  return fs.readdirSync(STATIC_DOCS).some(f => f.endsWith('.md'));
}

function generate() {
  // The root index page is now handled by src/pages/index.tsx (a standalone
  // Docusaurus page). We must NOT write docs-generated/index.mdx with slug:/
  // or Docusaurus will report a duplicate route conflict with routeBasePath:'/'.
  fs.mkdirSync(DOCS_DEST, { recursive: true });

  generateSpecsIndex();
}

console.log('Generating index page...');
generate();

module.exports = { generate };
