import prettier from "prettier";

interface WeatherData {
    temperature: number;
    humidity: number;
    weather: string;
}

interface ClothingRecommendation {
    item: string;
    reason: string;
}

interface Activity {
    name: string;
    suitability: string;
    reason: string;
}

interface ActivityAnalysis {
    indoor: Activity[];
    outdoor: Activity[];
    recommendations: string[];
}

interface ValidationItem {
    item: string;
    validated: boolean;
    reason: string;
}

type Input = {
    payload: {
        city: string;
        weather: WeatherData;
        clothing: ClothingRecommendation[];
        activities: Activity[];
        detailed_analysis: ActivityAnalysis;
        validated_items: ValidationItem[];
    };
    format: "json" | "txt";
};

type Output = {
    success: boolean;
    format: string;
    filename: string;
};

export async function saveDataTool(input: Input): Promise<Output> {
    // Input validation
    if (!input || typeof input !== 'object') {
        throw new Error('Invalid input: input must be an object');
    }
    
    if (!input.payload || typeof input.payload !== 'object') {
        throw new Error('Invalid input: payload must be an object');
    }
    
    if (!input.format || !['json', 'txt'].includes(input.format)) {
        throw new Error('Invalid input: format must be either "json" or "txt"');
    }
    
    if (typeof input.payload.city !== 'string' || input.payload.city.trim() === '') {
        throw new Error('Invalid input: payload.city must be a non-empty string');
    }

    const data = input.payload;

    if (input.format === "txt") {
        const txtContent = `Weather Report for ${data.city}
============================================

WEATHER CONDITIONS:
${JSON.stringify(data.weather, null, 2)}

RECOMMENDED ACTIVITIES:
${JSON.stringify(data.activities, null, 2)}

CLOTHING RECOMMENDATIONS:
${JSON.stringify(data.clothing, null, 2)}

DETAILED ACTIVITY ANALYSIS:
${JSON.stringify(data.detailed_analysis, null, 2)}

CLOTHING VALIDATION RESULTS:
${JSON.stringify(data.validated_items, null, 2)}

============================================
Generated at: ${new Date().toISOString()}
`;

        const filename = "results.txt";
        try {
            await Bun.write(filename, txtContent);
            return {
                success: true,
                format: "txt",
                filename: filename,
            };
        } catch (error) {
            throw new Error(`Failed to write ${filename}: ${error instanceof Error ? error.message : 'Unknown error'}`);
        }
    } else {
        const formatted = await prettier.format(JSON.stringify(data), { parser: "json" });
        const filename = "results.json";
        try {
            await Bun.write(filename, formatted);
            return {
                success: true,
                format: "json",
                filename: filename,
            };
        } catch (error) {
            throw new Error(`Failed to write ${filename}: ${error instanceof Error ? error.message : 'Unknown error'}`);
        }
    }
}
