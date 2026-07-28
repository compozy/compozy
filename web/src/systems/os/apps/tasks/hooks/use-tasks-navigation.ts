import { useOsShell } from "../../../hooks/use-os-shell";

export interface TasksNavigationLocation {
  pathname: string;
  search: object;
}

export function useTasksNavigation() {
  const { coordinator } = useOsShell();
  return (location: TasksNavigationLocation) => {
    void coordinator.userOpen({
      app: "tasks",
      route: { pathname: location.pathname, search: { ...location.search } },
    });
  };
}
