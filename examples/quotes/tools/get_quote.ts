export async function run() {
  const result = await fetch("https://api.gameofthronesquotes.xyz/v1/random");
  const json = await result.json();
  return { quote: json.sentence };
}
