import * as prettier from "npm:prettier";

type Result = {
    city: string;
    temperature: number;
    weather: string;
    clothing: string;
    activities: string;
};

type Output = {
    success: boolean;
};

export async function run(input: Result): Promise<Output> {
    const result = {
        city: input.city,
        temperature: input.temperature,
        weather: input.weather,
        clothing: input.clothing,
        activities: input.activities,
    };
    const formatted = await prettier.format(JSON.stringify(result), {
        parser: "json",
    });
    await Deno.writeTextFile("results.json", formatted);
    return { success: true };
}
