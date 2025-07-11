#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

/**
 * Script to validate documentation navigation integrity
 * Checks meta.json files against actual MDX files to identify broken links
 */

const DOCS_ROOT = 'docs/content/docs/core';

// Colors for console output
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  bold: '\x1b[1m'
};

function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function findFiles(dir, extension, results = []) {
  if (!fs.existsSync(dir)) return results;
  
  const files = fs.readdirSync(dir);
  for (const file of files) {
    const filePath = path.join(dir, file);
    const stat = fs.statSync(filePath);
    
    if (stat.isDirectory()) {
      findFiles(filePath, extension, results);
    } else if (file.endsWith(extension)) {
      results.push(filePath);
    }
  }
  return results;
}

function findMetaFiles() {
  return findFiles(DOCS_ROOT, 'meta.json');
}

function validateSection(metaFilePath) {
  const sectionDir = path.dirname(metaFilePath);
  const sectionName = path.relative(DOCS_ROOT, sectionDir) || 'root';
  
  let meta;
  try {
    meta = JSON.parse(fs.readFileSync(metaFilePath, 'utf8'));
  } catch (error) {
    return {
      section: sectionName,
      metaFile: metaFilePath,
      error: `Failed to parse meta.json: ${error.message}`,
      missingFiles: [],
      extraFiles: [],
      valid: false
    };
  }

  const pages = meta.pages || [];
  const isRootSection = meta.root === true;
  
  let actualFiles = [];
  const missingFiles = [];
  let extraFiles = [];

  if (isRootSection) {
    // For root section, find directories that have meta.json files
    const actualDirectories = fs.existsSync(sectionDir) 
      ? fs.readdirSync(sectionDir)
          .filter(item => {
            const itemPath = path.join(sectionDir, item);
            const stat = fs.statSync(itemPath);
            return stat.isDirectory() && fs.existsSync(path.join(itemPath, 'meta.json'));
          })
      : [];
    
    // Also add MDX files in the root
    const rootMdxFiles = fs.existsSync(sectionDir) 
      ? fs.readdirSync(sectionDir)
          .filter(file => file.endsWith('.mdx'))
          .map(file => path.basename(file, '.mdx'))
      : [];
    
    actualFiles = [...actualDirectories, ...rootMdxFiles];
    
    // For extraFiles, only include directories that exist but aren't in navigation
    extraFiles = [...actualDirectories.filter(dir => !pages.includes(dir))];
  } else {
    // For regular sections, find MDX files
    actualFiles = fs.existsSync(sectionDir) 
      ? fs.readdirSync(sectionDir)
          .filter(file => file.endsWith('.mdx'))
          .map(file => path.basename(file, '.mdx'))
      : [];
    extraFiles = [...actualFiles];
  }

  // Check each page reference
  for (const page of pages) {
    // Skip separator entries
    if (page.startsWith('---') && page.endsWith('---')) {
      continue;
    }

    let exists = false;
    if (isRootSection) {
      // For root, check if it's a directory with meta.json or an MDX file
      const dirPath = path.join(sectionDir, page);
      const mdxPath = path.join(sectionDir, `${page}.mdx`);
      exists = (fs.existsSync(dirPath) && fs.statSync(dirPath).isDirectory() && fs.existsSync(path.join(dirPath, 'meta.json'))) || fs.existsSync(mdxPath);
    } else {
      // For regular sections, check for MDX file
      const mdxPath = path.join(sectionDir, `${page}.mdx`);
      exists = fs.existsSync(mdxPath);
    }

    if (!exists) {
      missingFiles.push(page);
    } else {
      // Remove from extraFiles since it's referenced
      const index = extraFiles.indexOf(page);
      if (index > -1) {
        extraFiles.splice(index, 1);
      }
    }
  }

  return {
    section: sectionName,
    metaFile: metaFilePath,
    title: meta.title,
    pages: pages.filter(p => !p.startsWith('---')),
    actualFiles,
    missingFiles,
    extraFiles,
    valid: missingFiles.length === 0,
    error: null
  };
}

function generateReport() {
  log('🔍 Validating Documentation Navigation...', 'cyan');
  log('', 'reset');

  const metaFiles = findMetaFiles();
  const results = metaFiles.map(validateSection);
  
  const totalSections = results.length;
  const validSections = results.filter(r => r.valid).length;
  const totalMissingFiles = results.reduce((sum, r) => sum + r.missingFiles.length, 0);
  const totalExtraFiles = results.reduce((sum, r) => sum + r.extraFiles.length, 0);

  // Summary
  log('📊 SUMMARY', 'bold');
  log(`Total Sections: ${totalSections}`, 'blue');
  log(`Valid Sections: ${validSections}`, validSections === totalSections ? 'green' : 'yellow');
  log(`Broken Sections: ${totalSections - validSections}`, totalSections === validSections ? 'green' : 'red');
  log(`Missing Files: ${totalMissingFiles}`, totalMissingFiles === 0 ? 'green' : 'red');
  log(`Unreferenced Files: ${totalExtraFiles}`, totalExtraFiles === 0 ? 'green' : 'yellow');
  log('', 'reset');

  // Detailed results
  log('📋 DETAILED RESULTS', 'bold');
  log('', 'reset');

  for (const result of results) {
    if (result.error) {
      log(`❌ ${result.section}`, 'red');
      log(`   Error: ${result.error}`, 'red');
      continue;
    }

    if (result.valid && result.extraFiles.length === 0) {
      log(`✅ ${result.section} (${result.title})`, 'green');
    } else {
      log(`⚠️  ${result.section} (${result.title})`, 'yellow');
    }

    if (result.missingFiles.length > 0) {
      log(`   Missing files:`, 'red');
      result.missingFiles.forEach(file => {
        log(`     - ${file}.mdx`, 'red');
      });
    }

    if (result.extraFiles.length > 0) {
      log(`   Unreferenced files:`, 'yellow');
      result.extraFiles.forEach(file => {
        log(`     - ${file}.mdx`, 'yellow');
      });
    }

    if (result.missingFiles.length > 0 || result.extraFiles.length > 0) {
      log('', 'reset');
    }
  }

  // Generate fixes
  if (totalMissingFiles > 0 || totalExtraFiles > 0) {
    log('🔧 RECOMMENDED FIXES', 'bold');
    log('', 'reset');

    for (const result of results) {
      if (result.missingFiles.length > 0 || result.extraFiles.length > 0) {
        log(`Section: ${result.section}`, 'cyan');
        log(`File: ${result.metaFile}`, 'blue');
        
        if (result.missingFiles.length > 0) {
          log('Remove these missing file references:', 'red');
          result.missingFiles.forEach(file => {
            log(`  - "${file}"`, 'red');
          });
        }

        if (result.extraFiles.length > 0) {
          log('Consider adding these existing files:', 'yellow');
          result.extraFiles.forEach(file => {
            log(`  + "${file}"`, 'yellow');
          });
        }

        log('', 'reset');
      }
    }
  }

  return {
    totalSections,
    validSections,
    totalMissingFiles,
    totalExtraFiles,
    results
  };
}

// Generate JSON report for automated processing
function generateJsonReport(results) {
  const timestamp = new Date().toISOString();
  const report = {
    timestamp,
    summary: {
      totalSections: results.totalSections,
      validSections: results.validSections,
      brokenSections: results.totalSections - results.validSections,
      totalMissingFiles: results.totalMissingFiles,
      totalExtraFiles: results.totalExtraFiles
    },
    sections: results.results
  };

  const reportPath = path.join(process.cwd(), 'docs-navigation-report.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
  log(`📄 JSON report saved to: ${reportPath}`, 'green');
}

// Main execution
if (require.main === module) {
  try {
    const results = generateReport();
    generateJsonReport(results);
    
    // Exit with error code if there are issues
    process.exit(results.totalMissingFiles > 0 ? 1 : 0);
  } catch (error) {
    log(`❌ Validation failed: ${error.message}`, 'red');
    process.exit(1);
  }
}

module.exports = { generateReport, validateSection };
