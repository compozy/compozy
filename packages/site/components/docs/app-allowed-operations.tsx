import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@compozy/ui";
import manifest from "../../../../openapi/sdk-samples.json";
import { appAllowedOperations, type SDKSampleManifest } from "@/lib/sdk-samples";

export function AppAllowedOperations({
  samples = manifest as SDKSampleManifest,
}: {
  samples?: SDKSampleManifest;
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Allowed operation</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {appAllowedOperations(samples).map(operation => (
          <TableRow key={operation}>
            <TableCell>
              <code>{operation}</code>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
