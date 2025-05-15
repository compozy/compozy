#!/bin/bash

# Script to reorder workflow event documentation files.
# Commands will be numbered first, then state events, both alphabetically.

TARGET_DIR="docs/core-spec/events/workflow"

# Check if target directory exists
if [ ! -d "$TARGET_DIR" ]; then
  echo "Error: Directory $TARGET_DIR not found."
  exit 1
fi

# Change to the target directory
cd "$TARGET_DIR" || exit

echo "Reordering files in $TARGET_DIR..."

# Rename existing files to temporary names to avoid conflicts.
# Order: cancel, execute, pause, resume (Commands)
# Order: cancelled, completed, failed, paused, resumed, started, timed_out (State Events)

# Commands
if [ -f "01_cancel.md" ]; then mv "01_cancel.md" "01_cancel.md_TEMP_SCRIPT"; fi
if [ -f "02_execute.md" ]; then mv "02_execute.md" "02_execute.md_TEMP_SCRIPT"; fi
if [ -f "11_pause.md" ]; then mv "11_pause.md" "03_pause.md_TEMP_SCRIPT"; fi
if [ -f "10_resume.md" ]; then mv "10_resume.md" "04_resume.md_TEMP_SCRIPT"; fi

# State Events
if [ -f "03_execution_cancelled.md" ]; then mv "03_execution_cancelled.md" "05_execution_cancelled.md_TEMP_SCRIPT"; fi
if [ -f "04_execution_completed.md" ]; then mv "04_execution_completed.md" "06_execution_completed.md_TEMP_SCRIPT"; fi
if [ -f "05_execution_failed.md" ]; then mv "05_execution_failed.md" "07_execution_failed.md_TEMP_SCRIPT"; fi
if [ -f "08_execution_paused.md" ]; then mv "08_execution_paused.md" "08_execution_paused.md_TEMP_SCRIPT"; fi
if [ -f "09_execution_resumed.md" ]; then mv "09_execution_resumed.md" "09_execution_resumed.md_TEMP_SCRIPT"; fi
if [ -f "06_execution_started.md" ]; then mv "06_execution_started.md" "10_execution_started.md_TEMP_SCRIPT"; fi
if [ -f "07_execution_timed_out.md" ]; then mv "07_execution_timed_out.md" "11_execution_timed_out.md_TEMP_SCRIPT"; fi

# Rename temporary files to their final names
for tempfile in *_TEMP_SCRIPT; do
  if [ -f "$tempfile" ]; then
    finalname="${tempfile%_TEMP_SCRIPT}"
    mv "$tempfile" "$finalname"
    echo "Renamed $tempfile to $finalname"
  fi
done

echo "File reordering complete."

# Go back to the original directory
cd - > /dev/null
