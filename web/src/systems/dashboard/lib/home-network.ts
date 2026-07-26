/** Presentational row model for the home network panel. */
export interface HomeNetworkRow {
  key: string;
  label: string;
  value: string;
  tone?: "success" | "warning";
  mono?: boolean;
}
