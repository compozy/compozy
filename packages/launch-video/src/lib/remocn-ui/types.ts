// Source: https://remocn.dev — copied from the registry; MIT.
export interface Step<S extends string = string> {
  at: number;
  state: S;
  duration?: number;
}
