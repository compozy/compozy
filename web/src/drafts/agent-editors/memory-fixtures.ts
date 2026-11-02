/** Memory draft fixtures: notes shaped like the file's dated lines. */
export interface Note {
  id: string;
  text: string;
  /** The project it was learned in; absent when the person wrote it. */
  learnedIn?: string;
  date?: string;
  /** The day it stops being used. */
  until?: string;
  expired?: boolean;
}

export const NOTES: readonly Note[] = [
  {
    id: "preview",
    learnedIn: "reloaddev",
    date: "Nov 2",
    until: "Dec 1",
    text: "Preview deploys need `make preview`. The normal deploy skips the staging data.",
  },
  {
    id: "dispatch",
    learnedIn: "compozy",
    date: "Oct 31",
    text: "Give each issue its own worker. If something urgent comes up, steer the worker that's already on it instead of queueing a follow-up.",
  },
  {
    id: "hearing",
    learnedIn: "advos",
    date: "Oct 28",
    text: "Hearing dates live in the calendar, not in tasks.",
  },
  { id: "short", text: "Keep status updates short. Two lines is usually enough." },
  {
    id: "ana",
    learnedIn: "compozy",
    date: "Oct 24",
    text: "Ana reviews anything that touches the design system. Ask her first.",
  },
  {
    id: "friday",
    learnedIn: "compozy",
    date: "Oct 22",
    text: "No releases on Friday afternoon. The team agreed after the v0.3 rollback.",
  },
  { id: "pt", text: "Talk to me in Brazilian Portuguese. Code and docs stay in English." },
  {
    id: "flaky",
    learnedIn: "reloaddev",
    date: "Oct 19",
    text: "The checkout test fails about once a week for no real reason. Run it again before raising an alarm.",
  },
];

export const EXPIRED_NOTES: readonly Note[] = [
  { ...NOTES[0]!, until: "Oct 1", expired: true },
  ...NOTES.slice(1),
];

export const THEIR_NOTE: Note = {
  id: "theirs",
  learnedIn: "compozy",
  date: "Today",
  text: "Migrate tokens depends on the theme split. Don't start it first.",
};

export const REVISIONS = [
  { id: "r3", when: "Today, 09:12", what: "Triage Lead added a note", keep: 8 },
  { id: "r2", when: "Oct 31, 18:40", what: "You edited a note", keep: 7 },
  { id: "r1", when: "Oct 28, 11:02", what: "Triage Lead added a note", keep: 6 },
] as const;

export const PUT_AWAY = [
  "Sprint planning moved to Tuesdays.",
  "The old staging host was retired in August.",
];

export const FILE_LIMITS = { lines: 200, size: 24 };
