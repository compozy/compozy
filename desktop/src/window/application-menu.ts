import { app, Menu, type MenuItemConstructorOptions } from "electron";

import type { ProductWindow } from "./product-window";

export function installApplicationMenu(product: () => ProductWindow | null): void {
  const template: MenuItemConstructorOptions[] = [];
  if (process.platform === "darwin") {
    template.push({
      label: app.name,
      submenu: [{ role: "about" }, { type: "separator" }, { role: "hide" }, { role: "quit" }],
    });
  }
  template.push({
    label: "View",
    submenu: [
      {
        label: "Zoom In",
        accelerator: "CommandOrControl+=",
        click: () => product()?.adjustZoom("in"),
      },
      {
        label: "Zoom Out",
        accelerator: "CommandOrControl+-",
        click: () => product()?.adjustZoom("out"),
      },
      {
        label: "Actual Size",
        accelerator: "CommandOrControl+0",
        click: () => product()?.adjustZoom("reset"),
      },
      { type: "separator" },
      { role: "reload" },
    ],
  });
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}
