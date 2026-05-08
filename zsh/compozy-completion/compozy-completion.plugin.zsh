#!/usr/bin/env zsh

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

_compozy() {
  local -a comps
  local tasks_path
  local -a task_slugs
  local task_path

  if (( CURRENT == 1 )); then
    comps=(tasks)
    compadd -Q -- "$comps[@]"
    return 0
  fi

  if (( CURRENT == 2 )) && [[ $words[2] == "tasks" ]]; then
    comps=(run)
    compadd -Q -- "$comps[@]"
    return 0
  fi

  if (( CURRENT >= 3 )) && [[ $words[2] == "tasks" ]] && [[ $words[3] == "run" ]]; then
    tasks_path="$(_compozy_tasks_workspace)"

    if [[ -n "$tasks_path" && -d "$tasks_path" ]]; then
      task_slugs=()
      for task_path in "$tasks_path"/*(N); do
        [[ -e "$task_path" ]] || continue
        task_slugs+=("${task_path:t}")
      done

      if (( ${#task_slugs} > 0 )); then
        compadd -Q -a task_slugs
        return 0
      fi
    fi
  fi

  return 1
}

compdef _compozy compozy
