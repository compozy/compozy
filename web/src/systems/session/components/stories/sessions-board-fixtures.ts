import { fn } from "storybook/test";

import { sessionsBoardViewFixture } from "../../mocks/sessions-board-view-fixtures";

/** The shared board view model, with Storybook actions on its retry and load-more handlers. */
export function sessionsBoardView(
  overrides?: Parameters<typeof sessionsBoardViewFixture>[0],
  archive?: Parameters<typeof sessionsBoardViewFixture>[1],
  later?: Parameters<typeof sessionsBoardViewFixture>[3]
) {
  return sessionsBoardViewFixture(overrides, archive, fn, later);
}
