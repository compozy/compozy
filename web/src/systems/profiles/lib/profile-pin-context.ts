import { createContext } from "react";

/**
 * A concrete profile a subtree reads as, whatever the shell's view. A window opened on one profile's
 * item (an agent from the All profiles view) pins it, so every read inside resolves that item in its
 * own profile instead of the acting one (issue #1183). `null` follows the shell.
 */
export const ProfilePinContext = createContext<string | null>(null);
