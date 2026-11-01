/** Check-ins draft fixtures: how today went in each project. */
import type { PlaceActivity, Standing } from "./checkins-model";

const ACTIVITY: Record<string, PlaceActivity> = {
  compozy: {
    past: [
      { hour: 9, cell: "quiet" },
      { hour: 9.5, cell: "quiet" },
      { hour: 10, cell: "told" },
      { hour: 10.5, cell: "quiet" },
      { hour: 11, cell: "quiet" },
      { hour: 11.5, cell: "quiet" },
      { hour: 13, cell: "quiet" },
      { hour: 13.5, cell: "told" },
      { hour: 14, cell: "quiet" },
    ],
    recent: [
      { time: "14:00", title: "All quiet", description: "Nothing new, so it didn't bother you." },
      {
        time: "13:30",
        title: "Told you about 2 things",
        description: "Token migration is stuck. A review has been waiting since Tuesday.",
        told: true,
      },
      { time: "10:30 to 13:00", title: "4 quiet check-ins" },
      {
        time: "10:00",
        title: "Told you about 1 thing",
        description: "Nightly build failed twice in a row.",
        told: true,
      },
    ],
  },
  advos: {
    past: [
      { hour: 9, cell: "quiet" },
      { hour: 10, cell: "quiet" },
      { hour: 11, cell: "quiet" },
      { hour: 13, cell: "quiet" },
      { hour: 14, cell: "quiet" },
    ],
    recent: [{ time: "09:00 to 14:00", title: "5 quiet check-ins" }],
  },
};

const FAILED: PlaceActivity = {
  past: Array.from({ length: 10 }, (_, index) => ({
    hour: 9 + index / 2,
    cell: "failed" as const,
  })),
  recent: [
    {
      time: "13:30",
      title: "Couldn't finish",
      description: "It lost its connection partway through.",
      failed: true,
    },
    { time: "09:00 to 13:00", title: "9 more tries failed", failed: true },
  ],
};

export function activityFor(place: string, standing: Standing) {
  return standing === "failed" ? FAILED : (ACTIVITY[place] ?? ACTIVITY.compozy!);
}
