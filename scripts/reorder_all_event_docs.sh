#!/bin/bash

# Script to reorder and renumber event documentation files in workflow, task, agent, and tool directories
# to match the order specified in docs/core-spec/events/3. glossary.md.

# Function to extract base name (e.g., "execute.md" from "02_execute.md" or "runtime_status.md" from "runtime_status.md")
get_base_name() {
  local filename="$1"
  if [[ "$filename" =~ ^[0-9]{2}_(.*) ]]; then
    echo "${BASH_REMATCH[1]}"
  else
    echo "$filename"
  fi
}

# --- Workflow Events ---
TARGET_DIR_WORKFLOW="docs/core-spec/events/workflow"
# Current filenames as they appear in glossary links (without directory prefix)
# Order: ExecuteWorkflow, PauseWorkflow, ResumeWorkflow, CancelWorkflow, WorkflowExecutionStarted, WorkflowExecutionPaused, WorkflowExecutionResumed, WorkflowExecutionCompleted, WorkflowExecutionFailed, WorkflowExecutionCancelled, WorkflowExecutionTimedOut
current_files_workflow=(
  "02_execute.md"
  "03_pause.md"
  "04_resume.md"
  "01_cancel.md"
  "10_execution_started.md"
  "08_execution_paused.md"
  "09_execution_resumed.md"
  "06_execution_completed.md"
  "07_execution_failed.md"
  "05_execution_cancelled.md"
  "11_execution_timed_out.md"
)
# Target filenames (newly numbered and ordered)
target_files_workflow=(
  "01_execute.md"
  "02_pause.md"
  "03_resume.md"
  "04_cancel.md"
  "05_execution_started.md"
  "06_execution_paused.md"
  "07_execution_resumed.md"
  "08_execution_completed.md"
  "09_execution_failed.md"
  "10_execution_cancelled.md"
  "11_execution_timed_out.md"
)

# --- Task Events ---
TARGET_DIR_TASK="docs/core-spec/events/task"
# Order: TriggerSpecificTask, ExecuteTask, ResumeWaitingTask, TaskScheduled, TaskExecutionStarted, WaitingStarted, WaitingEnded, WaitingTimedOut, TaskExecutionCompleted, TaskExecutionFailed, TaskRetryScheduled, TaskCompletionResult, TaskFailureResult
current_files_task=(
  "01_trigger_specific.md"
  "03_execute.md"
  "06_resume_waiting.md"
  "02_scheduled.md"
  "04_execution_started.md"
  "14_waiting_started.md"
  "13_waiting_ended.md"
  "15_waiting_timed_out.md"
  "08_execution_completed.md"
  "10_execution_failed.md"
  "12_retry_scheduled.md"
  "09_completion_result.md"
  "11_failure_result.md"
)
target_files_task=(
  "01_trigger_specific.md"
  "02_execute.md"
  "03_resume_waiting.md"
  "04_scheduled.md"
  "05_execution_started.md"
  "06_waiting_started.md"
  "07_waiting_ended.md"
  "08_waiting_timed_out.md"
  "09_execution_completed.md"
  "10_execution_failed.md"
  "11_retry_scheduled.md"
  "12_completion_result.md"
  "13_failure_result.md"
)

# --- Agent Events ---
TARGET_DIR_AGENT="docs/core-spec/events/agent"
# Order: TriggerAgent, AgentInvocationStarted, AgentInvocationCompleted, AgentInvocationFailed, AgentDispatchFailed, AgentRuntimeStatus, AgentInvocationResult
current_files_agent=(
  "01_trigger.md"
  "02_invocation_started.md"
  "04_invocation_completed.md"
  "05_invocation_failed.md"
  "06_dispatch_failed.md"
  "runtime_status.md"
  "03_invocation_result.md"
)
target_files_agent=(
  "01_trigger.md"
  "02_invocation_started.md"
  "03_invocation_completed.md"
  "04_invocation_failed.md"
  "05_dispatch_failed.md"
  "06_runtime_status.md"
  "07_invocation_result.md"
)

# --- Tool Events ---
TARGET_DIR_TOOL="docs/core-spec/events/tool"
# Order: ExecuteTool, ToolInvocationStarted, ToolInvocationCompleted, ToolInvocationFailed, ToolExecutionResult
current_files_tool=(
  "01_execute.md"
  "02_invocation_started.md"
  "04_invocation_completed.md"
  "05_invocation_failed.md"
  "03_execution_result.md"
)
target_files_tool=(
  "01_execute.md"
  "02_invocation_started.md"
  "03_invocation_completed.md"
  "04_invocation_failed.md"
  "05_execution_result.md"
)

# Function to process a directory
# Arguments: 1=dir_path, 2=current_files_array_name (string), 3=target_files_array_name (string)
process_directory() {
  local dir_path="$1"
  local current_files_array_name="$2"
  local target_files_array_name="$3"
  local temp_suffix="_TEMP_REORDER"

  # Create indirect references to arrays
  # Requires Bash 4.0 for `[@]` with indirect expansion; otherwise, use eval or pass element by element
  # For wider compatibility, we'll use eval for array iteration, carefully.

  if [ ! -d "$dir_path" ]; then
    echo "Warning: Directory $dir_path not found. Skipping."
    return
  fi

  echo "Processing directory: $dir_path"
  pushd "$dir_path" > /dev/null || { echo "Error: Could not cd to $dir_path"; return; }

  # Pass 1: Rename current files to temporary names
  echo "Pass 1: Renaming to temporary names..."
  eval "current_files_list=(\"\${${current_files_array_name}[@]}\")"
  for current_file in "${current_files_list[@]}"; do
    if [ -f "$current_file" ]; then
      base_name=$(get_base_name "$current_file")
      temp_name="${base_name}${temp_suffix}"
      if [ "$current_file" != "$temp_name" ]; then
          mv "$current_file" "$temp_name"
          echo "  Renamed $current_file to $temp_name"
      else
          echo "  Skipping rename for $current_file (already suitable for temp or no change)"
      fi
    else
      echo "  Warning: File $current_file not found in $dir_path. Skipping."
    fi
  done

  # Pass 2: Rename temporary files to target names
  echo "Pass 2: Renaming temporary names to final target names..."
  eval "current_files_list_for_indices=(\"\${${current_files_array_name}[@]}\")" # To get indices
  eval "target_files_list=(\"\${${target_files_array_name}[@]}\")"

  for i in "${!current_files_list_for_indices[@]}"; do
    current_original_file="${current_files_list_for_indices[$i]}"
    target_file="${target_files_list[$i]}"
    base_name_original=$(get_base_name "$current_original_file")
    temp_name_to_find="${base_name_original}${temp_suffix}"

    if [ -f "$temp_name_to_find" ]; then
      if [ "$temp_name_to_find" != "$target_file" ]; then
          mv "$temp_name_to_find" "$target_file"
          echo "  Renamed $temp_name_to_find to $target_file"
      else
          echo "  Skipping rename for $temp_name_to_find (already target name)"
      fi
    else
      if [ -f "$current_original_file" ] && [ "$current_original_file" == "$target_file" ]; then
          echo "  File $current_original_file is already correctly named as $target_file. Skipping."
      elif ! [ -f "$target_file" ]; then
          echo "  Warning: Temporary file $temp_name_to_find (from original $current_original_file) not found for target $target_file. It might have been handled or was missing."
      fi
    fi
  done

  popd > /dev/null || exit
  echo "Finished processing $dir_path"
  echo ""
}

# Process all categories
process_directory "$TARGET_DIR_WORKFLOW" "current_files_workflow" "target_files_workflow"
process_directory "$TARGET_DIR_TASK" "current_files_task" "target_files_task"
process_directory "$TARGET_DIR_AGENT" "current_files_agent" "target_files_agent"
process_directory "$TARGET_DIR_TOOL" "current_files_tool" "target_files_tool"

echo "All event documentation reordering complete."
