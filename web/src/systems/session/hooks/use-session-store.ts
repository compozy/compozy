import { create } from "zustand";
import { createSessionStore, type SessionStore } from "../stores/session-store";

export const useSessionStore = create<SessionStore>(createSessionStore);
