export async function run(input: { city: string }) {
  const cities: Record<
    string,
    { weather: string; temperature: number; humidity: number }
  > = {
    "new york": { weather: "Partly cloudy", temperature: 72, humidity: 65 },
    london: { weather: "Rainy", temperature: 65, humidity: 80 },
    tokyo: { weather: "Sunny", temperature: 80, humidity: 55 },
    paris: { weather: "Clear", temperature: 70, humidity: 60 },
    sydney: { weather: "Windy", temperature: 68, humidity: 45 },
    berlin: { weather: "Overcast", temperature: 62, humidity: 70 },
  };

  const normalizedCity = input.city?.toLowerCase();
  const weatherData = cities[normalizedCity] || {
    weather: "Unknown",
    temperature: 75,
    humidity: 50,
  };
  await new Promise((resolve) => setTimeout(resolve, 100));
  return weatherData;
}
