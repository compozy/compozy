import { buildPublicSearchIndexes } from "@/lib/public-search-index";
import { createSearchAPI } from "fumadocs-core/search/server";

export const { GET } = createSearchAPI("advanced", {
  indexes: buildPublicSearchIndexes(),
});
