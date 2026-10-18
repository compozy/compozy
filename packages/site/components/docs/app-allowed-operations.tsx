import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
  TableCaption,
} from "@compozy/ui";
import manifest from "../../../../openapi/sdk-samples.json";
import { appAllowedOperations, type SDKSampleManifest } from "@/lib/sdk-samples";

export function AppAllowedOperations({
  samples = manifest as SDKSampleManifest,
}: {
  samples?: SDKSampleManifest;
}) {
  return (
    <>
      <p>Operations allowed by the API contract used to build this documentation.</p>
      {appAllowedOperations(samples).map(([resource, operations]) => (
        <Table key={resource}>
          <TableCaption>{resource}</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Allowed operation</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {operations.map(operation => (
              <TableRow key={operation}>
                <TableCell>
                  <code>{operation}</code>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ))}
    </>
  );
}
