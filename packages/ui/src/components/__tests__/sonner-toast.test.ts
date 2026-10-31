import { beforeEach, describe, expect, it, vi } from "vitest";

import { toast as exportedToast } from "../../index";
import { toast } from "../sonner-toast";

const sonnerToast = vi.hoisted(() =>
  Object.assign(
    vi.fn(() => "neutral-id"),
    {
      success: vi.fn(() => "success-id"),
      info: vi.fn(() => "info-id"),
      warning: vi.fn(() => "warning-id"),
      error: vi.fn(() => "error-id"),
      custom: vi.fn((_render: (id: string | number) => unknown, _options?: unknown) => "custom-id"),
      dismiss: vi.fn((_id?: string | number) => "dismissed-id"),
      getToasts: vi.fn((): Array<{ id: string | number }> => []),
    }
  )
);

vi.mock("sonner", () => ({ toast: sonnerToast }));

describe("toast facade", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards an error with its options and returns sonner's id (UT-013)", () => {
    const options = {
      description: "d",
      duration: 4000,
      action: { label: "Retry", onClick: vi.fn() },
    };

    const id = toast.error("Couldn't restart the session", options);

    expect(sonnerToast.error).toHaveBeenCalledExactlyOnceWith(
      "Couldn't restart the session",
      options
    );
    expect(id).toBe("error-id");
  });

  it.each(["success", "info", "warning"] as const)(
    "forwards %s to the same-named sonner method (UT-014)",
    method => {
      const options = { description: "d", id: "a" };

      const id = toast[method]("Saved", options);

      expect(sonnerToast[method]).toHaveBeenCalledExactlyOnceWith("Saved", options);
      expect(id).toBe(`${method}-id`);
    }
  );

  it("forwards no options argument when the caller passes none", () => {
    toast.error("Saved");

    expect(sonnerToast.error).toHaveBeenCalledExactlyOnceWith("Saved");
  });

  it("sends message to sonner's default toast (UT-014)", () => {
    const id = toast.message("Deleted note", { description: "d" });

    expect(sonnerToast).toHaveBeenCalledExactlyOnceWith("Deleted note", { description: "d" });
    expect(id).toBe("neutral-id");
  });

  it("lists the ids of on-screen toasts (UT-015)", () => {
    vi.mocked(sonnerToast.getToasts).mockReturnValue([{ id: "a" }, { id: 2 }] as ReturnType<
      typeof sonnerToast.getToasts
    >);

    expect(toast.activeIds()).toEqual(["a", 2]);
  });

  it("dismisses every toast without an id and one toast with an id (UT-016)", () => {
    toast.dismiss();
    expect(sonnerToast.dismiss.mock.calls[0]?.[0]).toBeUndefined();

    toast.dismiss("a");
    expect(sonnerToast.dismiss).toHaveBeenLastCalledWith("a");
  });

  it("hands the custom render its id (UT-017)", () => {
    const render = vi.fn((id: string | number) => `notice ${id}`);

    const id = toast.custom(render, { id: "att-1" });

    const passedRender = sonnerToast.custom.mock.calls[0]?.[0];
    expect(passedRender?.("att-1")).toBe("notice att-1");
    expect(render).toHaveBeenCalledExactlyOnceWith("att-1");
    expect(id).toBe("custom-id");
  });

  it("forwards non-dismissible options unchanged (UT-018)", () => {
    const options = {
      id: "att-overflow",
      closeButton: false,
      dismissible: false,
      duration: Infinity,
    };

    toast.custom(() => "overflow", options);

    expect(sonnerToast.custom.mock.calls[0]?.[1]).toEqual(options);
  });

  it("lets a spy on the @compozy/ui export replace the call consumers make (UT-019)", () => {
    const spy = vi.spyOn(exportedToast, "error").mockReturnValue("spied-id");

    expect(toast.error("x")).toBe("spied-id");

    expect(spy).toHaveBeenCalledExactlyOnceWith("x");
    expect(sonnerToast.error).not.toHaveBeenCalled();
    spy.mockRestore();
  });
});
