#!/usr/bin/env python3

import os
import re

# List of test files that need fixing
test_files = [
    "cmd/mcp-server/integration_test.go",
    "cmd/mcp-server/main_integration_test.go",
    "cmd/mcp-server/main_test.go",
    "cmd/mcp-server/server_edge_cases_test.go",
]

def add_slog_import(filepath):
    """Add log/slog import to test files if not present"""
    with open(filepath, 'r') as f:
        content = f.read()
    
    # Check if slog is already imported
    if '"log/slog"' in content or '`log/slog`' in content:
        print(f"  {filepath}: slog already imported")
        return
    
    # Find the import block
    import_pattern = r'(import\s+\([^)]+\))'
    match = re.search(import_pattern, content, re.MULTILINE | re.DOTALL)
    
    if match:
        import_block = match.group(1)
        # Add slog import before the last closing parenthesis
        new_import_block = import_block.rstrip(')')
        new_import_block += '\n\t"log/slog"\n)'
        
        # Replace the old import block with the new one
        new_content = content.replace(import_block, new_import_block)
        
        with open(filepath, 'w') as f:
            f.write(new_content)
        
        print(f"  {filepath}: Added slog import")
    else:
        print(f"  {filepath}: Could not find import block")

print("Fixing imports in test files...")
for test_file in test_files:
    if os.path.exists(test_file):
        add_slog_import(test_file)
    else:
        print(f"  {test_file}: File not found")

print("\nImports fixed successfully!")