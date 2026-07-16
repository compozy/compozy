export interface WorkPackageSearch {
  package_id?: string;
}

export function workPackageSearchSchema(search: Record<string, unknown>): WorkPackageSearch {
  const packageId = typeof search.package_id === "string" ? search.package_id.trim() : "";
  return packageId ? { package_id: packageId } : {};
}
