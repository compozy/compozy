import * as prettier from "npm:prettier";

type WeatherData = {
    temperature: number;
    humidity: number;
    weather: string;
};

type Result = {
    city: string;
    weather: WeatherData | any; // Can be the weather object or other format
    clothing: string | object | any[]; // Can be string, object, or array
    activities: string | object | any[]; // Can be string, object, or array
    format: "json" | "csv";
};

type Output = {
    success: boolean;
    format: string;
    filename: string;
};

function normalizeToString(value: string | object | any[]): string {
    if (typeof value === "string") {
        return value;
    }
    if (Array.isArray(value)) {
        return value.join(", ");
    }
    if (typeof value === "object" && value !== null) {
        // Handle objects with specific structure like { clothings: ["item1", "item2"] }
        if ("clothings" in value && Array.isArray(value.clothings)) {
            return value.clothings.join(", ");
        }
        // For other objects, try to extract meaningful values
        const values = Object.values(value).filter(v => v !== null && v !== undefined);
        if (values.length > 0) {
            return values.map(v => Array.isArray(v) ? v.join(", ") : String(v)).join(", ");
        }
        return JSON.stringify(value);
    }
    return String(value);
}

export async function run(input: Result): Promise<Output> {
    // Extract temperature and weather from the weather object if it's structured
    let temperature: number;
    let weather: string;

    if (typeof input.weather === 'object' && input.weather !== null) {
        // If weather is an object, extract temperature and weather fields
        temperature = input.weather.temperature || 0;
        weather = input.weather.weather || 'Unknown';
    } else {
        // Fallback for other formats
        temperature = 0;
        weather = String(input.weather || 'Unknown');
    }

    // Normalize clothing and activities to strings for CSV
    const clothingStr = normalizeToString(input.clothing);
    const activitiesStr = normalizeToString(input.activities);

    const result = {
        city: input.city,
        temperature: temperature,
        weather: weather,
        clothing: clothingStr,
        activities: activitiesStr,
    };

    if (input.format === "csv") {
        // Generate CSV content
        const headers = Object.keys(result).join(",");
        const values = Object.values(result).map(value => {
            // Escape values that contain commas or quotes
            const stringValue = String(value);
            if (stringValue.includes(",") || stringValue.includes('"')) {
                return `"${stringValue.replace(/"/g, '""')}"`;
            }
            return stringValue;
        }).join(",");

        const csvContent = `${headers}\n${values}`;
        const filename = "results.csv";
        await Deno.writeTextFile(filename, csvContent);
        return {
            success: true,
            format: "csv",
            filename: filename
        };
    } else {
        // Generate JSON content (default) - preserve original data types
        const jsonResult = {
            city: input.city,
            temperature: temperature,
            weather: weather,
            clothing: input.clothing, // Preserve original structure for JSON
            activities: input.activities, // Preserve original structure for JSON
        };

        const formatted = await prettier.format(JSON.stringify(jsonResult), {
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