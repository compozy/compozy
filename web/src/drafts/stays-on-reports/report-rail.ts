import { INSTANCES } from "../stays-on-agents-shared/fixtures";
import {
  COMPOZY_ROW,
  GLOBAL_ROW,
  RELOADDEV_ROW,
} from "../stays-on-agents-shared/sessions-fixtures";

export const RAIL = {
  groups: [
    {
      name: "compozy",
      count: 3,
      rows: ["s4", "s2"],
      instances: COMPOZY_ROW,
      badges: true,
      rowProps: { currentId: INSTANCES.triageCompozy.id },
    },
    { name: "reloaddev", count: 1, rows: ["s1"], instances: RELOADDEV_ROW },
  ],
  top: GLOBAL_ROW,
};
