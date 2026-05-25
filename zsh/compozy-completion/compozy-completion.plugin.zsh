#!/usr/bin/env zsh

# Finds the nearest workspace `.compozy/tasks` directory by walking up from the
# current working directory and returns the first match.
_compozy_tasks_workspace() {
  local dir="$PWD"

  while [[ -n "$dir" ]]; do
    if [[ -d "$dir/.compozy/tasks" ]]; then
      print -r -- "$dir/.compozy/tasks"
      return 0
    fi

    [[ "$dir" == "/" ]] && break
    dir="${dir:h}"
  done

  return 1
}

# Returns a newline-separated list of task slugs found in the discovered
# `.compozy/tasks` directory, one slug per line.
_compozy_task_slugs() {
  local tasks_path
  local -a task_slugs
  local task_path

  tasks_path="$(_compozy_tasks_workspace)"
  if [[ -z "$tasks_path" || ! -d "$tasks_path" ]]; then
    return 1
  fi

  task_slugs=()
  for task_path in "$tasks_path"/*(N/); do
    task_slugs+=("${task_path:t}")
  done

  if (( ${#task_slugs[@]} == 0 )); then
    return 1
  fi

  print -l -- "${task_slugs[@]}"
}

# Provides zsh completion for `compozy` command flows and returns task slugs for
# completion of `compozy tasks run`.
_compozy() {
  local -a comps
  local tasks_path
  local -a task_slugs
  local task_path

  if (( CURRENT == 2 )); then
    comps=(tasks)
    compadd -Q -- "$comps[@]"
    return 0
  fi

  if (( CURRENT == 3 )) && [[ $words[2] == "tasks" ]]; then
    comps=(run)
    compadd -Q -- "$comps[@]"
    return 0
  fi

  if (( CURRENT >= 4 )) && [[ $words[2] == "tasks" ]] && [[ $words[3] == "run" ]]; then
    task_slugs=("${(@f)$(_compozy_task_slugs)}")
    if (( ${#task_slugs[@]} > 0 )); then
      compadd -Q -a task_slugs
      return 0
    fi
  fi

  return 1
}

compdef _compozy compozy
