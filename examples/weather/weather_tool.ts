import { fetchWeatherApi } from "openmeteo";

interface WeatherData {
    temperature: number;
    humidity: number;
    weather: string;
}

interface GeocodingResult {
    latitude: number;
    longitude: number;
}

async function getCoordinates(city: string): Promise<GeocodingResult | null> {
    const geocodingUrl = `https://geocoding-api.open-meteo.com/v1/search?name=${
        encodeURIComponent(
            city,
        )
    }&count=1`;
    try {
        const geoResponse = await fetch(geocodingUrl);
        const geoData = await geoResponse.json();
        if (!geoData.results?.[0]) {
            return null;
        }
        const { latitude, longitude } = geoData.results[0];
        return { latitude, longitude };
    } catch (error) {
        console.error("Error fetching coordinates:", error);
        return null;
    }
}

function getWeatherDescription(weatherCode: number): string {
    const weatherMap: { [key: number]: string } = {
        0: "Clear sky",
        1: "Mainly clear",
        2: "Partly cloudy",
        3: "Overcast",
        45: "Fog",
        48: "Depositing rime fog",
        51: "Light drizzle",
        53: "Moderate drizzle",
        55: "Dense drizzle",
        56: "Light freezing drizzle",
        57: "Dense freezing drizzle",
        61: "Slight rain",
        63: "Moderate rain",
        65: "Heavy rain",
        66: "Light freezing rain",
        67: "Heavy freezing rain",
        71: "Slight snow fall",
        73: "Moderate snow fall",
        75: "Heavy snow fall",
        77: "Snow grains",
        80: "Slight rain showers",
        81: "Moderate rain showers",
        82: "Violent rain showers",
        85: "Slight snow showers",
        86: "Heavy snow showers",
        95: "Thunderstorm",
        96: "Thunderstorm with slight hail",
        99: "Thunderstorm with heavy hail",
    };
    return weatherMap[weatherCode] || "Unknown weather condition";
}

async function getWeatherData(
    latitude: number,
    longitude: number,
): Promise<WeatherData | null> {
    try {
        const params = {
            latitude: [latitude],
            longitude: [longitude],
            current: "temperature_2m,relative_humidity_2m,weather_code",
        };
        const responses = await fetchWeatherApi(
            "https://api.open-meteo.com/v1/forecast",
            params,
        );
        const response = responses[0];
        const current = response.current()!;
        const weatherCode = Math.round(current.variables(2)!.value());
        return {
            temperature: Math.round(current.variables(0)!.value()),
            humidity: Math.round(current.variables(1)!.value()),
            weather: getWeatherDescription(weatherCode),
        };
    } catch (error) {
        console.error("Error fetching weather:", error);
        return null;
    }
}

function getDefaultWeather(): WeatherData {
    return {
        temperature: 20,
        humidity: 50,
        weather: "Clear sky",
    };
}

export async function weatherTool(input: { city: string }): Promise<WeatherData> {
    // Input validation
    if (!input || typeof input.city !== 'string' || input.city.trim() === '') {
        throw new Error('Invalid input: city must be a non-empty string');
    }

    const cityName = input.city.trim();
    
    try {
        // Get coordinates for the city
        const coords = await getCoordinates(cityName);
        if (!coords) {
            return getDefaultWeather();
        }

        // Get weather data using coordinates
        const weather = await getWeatherData(coords.latitude, coords.longitude);
        if (!weather) {
            return getDefaultWeather();
        }

        return weather;
    } catch (_err) {
        return getDefaultWeather();
    }
}
