/** A half-hour step of the day as `09:30`. */
export function formatClock(hour: number) {
  const whole = Math.floor(hour);
  return `${String(whole).padStart(2, "0")}:${hour % 1 === 0 ? "00" : "30"}`;
}
