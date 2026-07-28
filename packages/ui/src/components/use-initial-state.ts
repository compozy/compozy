import * as React from "react";

function useInitialState<T>(initialValue: T): [T, React.Dispatch<React.SetStateAction<T>>] {
  return React.useState(initialValue);
}

export { useInitialState };
