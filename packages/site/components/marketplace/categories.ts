/** Category glyphs for the rail, in the daemon's fixed order (labels come from `lib/marketplace-view`). */
import {
  Blocks,
  Bot,
  ChartBar,
  Code2,
  Coins,
  Compass,
  Globe,
  Megaphone,
  MessagesSquare,
  Palette,
  Search,
  Shield,
  SquareCheck,
} from "@compozy/ui/icons";

import type { MarketplaceCategoryId } from "@/lib/marketplace-cards";

export const CATEGORY_ICONS: Record<MarketplaceCategoryId, typeof Compass> = {
  productivity: SquareCheck,
  communication: MessagesSquare,
  design: Palette,
  development: Code2,
  data: ChartBar,
  cloud: Globe,
  finance: Coins,
  "sales-marketing": Megaphone,
  research: Search,
  security: Shield,
  "ai-agents": Bot,
  more: Blocks,
};

export const ALL_ICON = Compass;
