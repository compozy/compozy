import { BrowserWindow, dialog, type OpenDialogOptions, type WebContents } from "electron";

import { isAbsoluteDirectoryPath } from "./product-contract";

/** Opens the OS folder chooser for the renderer that asked; resolves `null` on cancel. */
export interface DirectoryDialog {
  pick(owner: WebContents, defaultPath: string | undefined): Promise<string | null>;
}

/**
 * The native folder dialog, sheet-attached to the asking window. macOS offers
 * "New Folder" inside the dialog; other platforms ignore that property.
 */
export const electronDirectoryDialog: DirectoryDialog = {
  async pick(owner, defaultPath) {
    const options: OpenDialogOptions = {
      properties: ["openDirectory", "createDirectory"],
      ...(defaultPath ? { defaultPath } : {}),
    };
    const window = BrowserWindow.fromWebContents(owner);
    const result = window
      ? await dialog.showOpenDialog(window, options)
      : await dialog.showOpenDialog(options);
    const [path] = result.filePaths;
    if (result.canceled || !isAbsoluteDirectoryPath(path)) return null;
    return path;
  },
};
