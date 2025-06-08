import * as prettier from "npm:prettier";

type Input = {
    payload: any; // Accept any payload structure
};

type Output = {
    success: boolean;
    format: string;
    filename: string;
};

export async function run(input: Input): Promise<Output> {
    const data = input.payload;
    const format = data.format || "json";

    if (format === "csv") {
        // For CSV, try to flatten the data structure
        const csvContent = JSON.stringify(data, null, 2);
        const filename = "results.csv";
        await Deno.writeTextFile(filename, csvContent);
        return {
            success: true,
            format: "csv",
            filename: filename
        };
    } else {
        // Generate JSON content (default) - save exactly what we received
        const formatted = await prettier.format(JSON.stringify(data), {
            parser: "json",
        });
        const filename = "results.json";
        await Deno.writeTextFile(filename, formatted);
        return {
            success: true,
            format: "json",
            filename: filename
        };
    }
}
