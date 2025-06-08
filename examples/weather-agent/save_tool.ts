import * as prettier from "npm:prettier";

type WeatherData = {
    temperature: number;
    humidity: number;
    weather: string;
};

type SingleCityData = {
    city: string;
    weather: WeatherData | any;
    clothing: string | object | any[];
    activities: string | object | any[];
};

type MultiCityInput = {
    payload: {
        all_cities_data?: any[]; // Collection results
        summary?: {
            total_cities: number;
            processed_cities: number;
            failed_cities: number;
            mode: string;
            strategy: string;
        };
        format: "json" | "csv";
        // Legacy single city format for backward compatibility
        city?: string;
        weather?: WeatherData | any;
        clothing?: string | object | any[];
        activities?: string | object | any[];
    };
};

type Input = SingleCityData | MultiCityInput;

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

function extractCityDataFromResult(resultItem: any): any {
    // Handle collection result structure
    if (resultItem?.item?.output?.complete_data) {
        return resultItem.item.output.complete_data;
    }
    
    // Handle direct city data
    if (resultItem?.city) {
        return resultItem;
    }
    
    return null;
}

export async function run(input: Input): Promise<Output> {
    const data = (input as MultiCityInput).payload;

    // Check if this is multi-city collection data
    if (data.all_cities_data && Array.isArray(data.all_cities_data)) {
        // Process multiple cities
        const cities = data.all_cities_data
            .map(extractCityDataFromResult)
            .filter(city => city !== null);

        if (data.format === "csv") {
            // Generate CSV for multiple cities
            if (cities.length === 0) {
                const csvContent = "city,temperature,weather,clothing,activities,priority\n";
                const filename = "results.csv";
                await Deno.writeTextFile(filename, csvContent);
                return {
                    success: true,
                    format: "csv",
                    filename: filename
                };
            }

            const headers = "city,temperature,weather,clothing,activities,priority";
            const rows = cities.map(city => {
                const clothingStr = normalizeToString(city.clothing || []);
                const activitiesStr = normalizeToString(city.activities || []);
                
                const values = [
                    city.city || "Unknown",
                    String(city.temperature || 0),
                    String(city.weather || "Unknown"),
                    clothingStr,
                    activitiesStr,
                    String(city.priority || "medium")
                ];

                return values.map(value => {
                    const stringValue = String(value);
                    if (stringValue.includes(",") || stringValue.includes('"')) {
                        return `"${stringValue.replace(/"/g, '""')}"`;
                    }
                    return stringValue;
                }).join(",");
            });

            const csvContent = `${headers}\n${rows.join("\n")}`;
            const filename = "results.csv";
            await Deno.writeTextFile(filename, csvContent);
            return {
                success: true,
                format: "csv",
                filename: filename
            };
        } else {
            // Generate JSON for multiple cities
            const jsonResult = {
                summary: {
                    total_cities: data.summary?.total_cities || cities.length,
                    processed_cities: data.summary?.processed_cities || cities.length,
                    failed_cities: data.summary?.failed_cities || 0,
                    mode: data.summary?.mode || "parallel",
                    strategy: data.summary?.strategy || "best_effort",
                    timestamp: new Date().toISOString()
                },
                cities: cities.map(city => ({
                    city: city.city || "Unknown",
                    priority: city.priority || "medium",
                    temperature: city.temperature || 0,
                    humidity: city.humidity || 0,
                    weather: city.weather || "Unknown",
                    activities: city.activities || [],
                    clothing: city.clothing || [],
                    timestamp: city.timestamp || new Date().toISOString()
                }))
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
    } else {
        // Handle legacy single city format
        let temperature: number;
        let weather: string;

        if (typeof data.weather === 'object' && data.weather !== null) {
            temperature = data.weather.temperature || 0;
            weather = data.weather.description || data.weather.weather || 'Unknown';
        } else {
            temperature = 0;
            weather = String(data.weather || 'Unknown');
        }

        const clothingStr = normalizeToString(data.clothing);
        const activitiesStr = normalizeToString(data.activities);

        const result = {
            city: data.city,
            temperature: temperature,
            weather: weather,
            clothing: clothingStr,
            activities: activitiesStr,
        };

        if (data.format === "csv") {
            const headers = Object.keys(result).join(",");
            const values = Object.values(result).map(value => {
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
            const jsonResult = {
                city: data.city,
                temperature: temperature,
                weather: weather,
                clothing: data.clothing,
                activities: data.activities,
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
}
