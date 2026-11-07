// The loading skeleton sits in the same containers as the loaded fleet, so a row or card
// changes only its content when the catalog arrives.
// Columns fill the window, so a wide one shows five teammates a row and a narrow one stays readable.
export const FLEET_CARDS_CLASS = "grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-3";
export const FLEET_ROWS_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";
export const FLEET_SKELETON_ROWS = 8;
export const FLEET_SKELETON_CARDS = 10;
