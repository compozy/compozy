import { createContext } from "react";

import type { SessionCreateStore } from "../stores/session-create-store";

const SessionCreateContext = createContext<SessionCreateStore | null>(null);

export { SessionCreateContext };
