import { fetchWeatherApi } from "npm:openmeteo";

interface WeatherData {
    temperature: string;
    humidity: number;
}

interface GeocodingResult {
    latitude: number;
    longitude: number;
}

async function getCoordinates(city: string): Promise<GeocodingResult | null> {
    const geocodingUrl = `https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(city)}&count=1`;
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

async function getWeatherData(latitude: number, longitude: number): Promise<WeatherData | null> {
    try {
        const params = {
            latitude: [latitude],
            longitude: [longitude],
            current: "temperature_2m,relative_humidity_2m,weather_code",
        };
        const responses = await fetchWeatherApi("https://api.open-meteo.com/v1/forecast", params);
        const response = responses[0];
        const current = response.current()!;
        return {
            temperature: `${Math.round(current.variables(0)!.value())}°C`,
            humidity: Math.round(current.variables(1)!.value()),
        };
    } catch (error) {
        console.error("Error fetching weather:", error);
        return null;
    }
}

function getDefaultWeather(): WeatherData {
    return {
        temperature: "20°C",
        humidity: 50,
    };
}

export async function run(input: { city: string }): Promise<WeatherData> {
    try {
        // Get coordinates for the city
        const coords = await getCoordinates(input.city);
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
