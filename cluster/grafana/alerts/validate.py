#!/usr/bin/env python3
"""
Simple YAML structure validation for alerting rules.
This script checks basic YAML syntax and alert structure without external dependencies.
"""

import json
import re

def validate_yaml_structure(filename):
    """Validate basic YAML structure and alert rules."""
    try:
        with open(filename, 'r') as f:
            content = f.read()
        
        # Basic checks for all files
        assert 'groups:' in content, "Missing 'groups:' key"
        assert 'name:' in content, "Missing group 'name:' key" 
        assert 'rules:' in content, "Missing 'rules:' key"
        assert 'alert:' in content, "Missing 'alert:' key"
        assert 'expr:' in content, "Missing 'expr:' key"
        assert 'severity:' in content, "Missing 'severity:' label"
        
        # Memory-specific checks only for memory-alerts.yaml
        if 'memory-alerts.yaml' in filename:
            # Check for critical memory alerts
            critical_alerts = [
                'MemorySystemDown',
                'MemoryTokenLimitReached',
                'MemoryCircuitBreakerTripped'
            ]
            
            for alert in critical_alerts:
                assert alert in content, f"Missing critical alert: {alert}"
            
            # Check for memory metrics usage
            memory_metrics = [
                'compozy_memory_health_status',
                'compozy_memory_tokens_used_gauge',
                'compozy_memory_operation_duration_seconds'
            ]
            
            for metric in memory_metrics:
                assert metric in content, f"Missing memory metric: {metric}"
        
        print(f"✓ {filename} validation passed")
        return True
        
    except Exception as e:
        print(f"✗ {filename} validation failed: {e}")
        return False

def main():
    """Validate all alert files."""
    import os
    script_dir = os.path.dirname(os.path.abspath(__file__))
    files = [
        os.path.join(script_dir, 'memory-alerts.yaml'), 
        os.path.join(script_dir, 'schedule-alerts.yaml')
    ]
    
    all_valid = True
    for filename in files:
        try:
            valid = validate_yaml_structure(filename)
            all_valid = all_valid and valid
        except FileNotFoundError:
            print(f"✗ File not found: {filename}")
            all_valid = False
    
    if all_valid:
        print("✓ All alert files validated successfully")
        exit(0)
    else:
        print("✗ Some validations failed")
        exit(1)

if __name__ == "__main__":
    main()
