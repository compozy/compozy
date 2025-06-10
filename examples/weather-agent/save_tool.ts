import * as prettier from "npm:prettier";

type Input = {
    payload: {
        city: string;
        weather: any;
        clothing: any;
        activities: any;
        detailed_analysis: any;
    };
    format: "json" | "txt";
};

type Output = {
    success: boolean;
    format: string;
    filename: string;
};

export async function run(input: Input): Promise<Output> {
    const data = input.payload;

    if (input.format === "txt") {
        const txtContent = `Weather Report for ${data.city}
============================================
Weather: ${JSON.stringify(data.weather, null, 2)}
Activities: ${JSON.stringify(data.activities, null, 2)}
Clothing Recommendations: ${JSON.stringify(data.clothing, null, 2)}
Detailed Analysis: ${JSON.stringify(data.detailed_analysis, null, 2)}
Generated at: ${new Date().toISOString()}
`;

        const filename = "results.txt";
        await Deno.writeTextFile(filename, txtContent);
        return {
            success: true,
            format: "txt",
            filename: filename,
        };
    } else {
        const formatted = await prettier.format(JSON.stringify(data), { parser: "json" });
        const filename = "results.json";
        await Deno.writeTextFile(filename, formatted);
        return {
            success: true,
            format: "json",
            filename: filename,
        };
    }
}
