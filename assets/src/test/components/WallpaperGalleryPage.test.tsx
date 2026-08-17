import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { WallpaperGalleryPage } from "../../WallpaperGalleryPage";
import type { WallpaperListResult } from "../../types/api";

vi.mock("../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, string | number>) => {
    const translations: Record<string, string> = {
      "gallery.title": "Wallpaper Gallery",
      "gallery.empty": "No wallpapers yet. Upload one to get started.",
      "gallery.upload": "Upload",
      "gallery.size": "Size",
      "gallery.referenced_by": "Used by {count} item(s)",
      "gallery.delete_confirm": "Delete wallpaper?",
      "gallery.delete_confirm_desc": "The image file will be permanently removed.",
      "gallery.unbound_note": "References will be unbound automatically.",
      "modal.upload_fail": "Upload failed",
      "modal.cancel": "Cancel",
      "modal.confirm": "Confirm",
      "errors.operation_failed": "Operation failed",
      "button.delete": "Delete",
    };
    let text = translations[key] || key;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        text = text.replace(`{${k}}`, String(v));
      }
    }
    return text;
  },
}));

const mockListWallpapers = vi.fn();
const mockUploadWallpaper = vi.fn();
const mockDeleteWallpaper = vi.fn();

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    listWallpapers: mockListWallpapers,
    uploadWallpaper: mockUploadWallpaper,
    deleteWallpaper: mockDeleteWallpaper,
  }),
}));

const sampleWallpapers: WallpaperListResult[] = [
  { name: "a.png", size: 2048, refs: 1 },
  { name: "b.jpg", size: 102400, refs: 0 },
];

describe("WallpaperGalleryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWallpapers.mockResolvedValue(sampleWallpapers);
  });

  it("renders loading spinner while fetching", () => {
    let resolveList: (v: WallpaperListResult[]) => void;
    mockListWallpapers.mockReturnValue(
      new Promise((resolve) => {
        resolveList = resolve;
      })
    );
    render(<WallpaperGalleryPage />);
    expect(screen.getByTestId("gallery-loading")).toBeTruthy();
    resolveList!([{ name: "x.png", size: 1, refs: 0 }]);
  });

  it("renders empty state when no wallpapers", async () => {
    mockListWallpapers.mockResolvedValue([]);
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-empty")).toBeTruthy();
    });
    expect(screen.getByText("No wallpapers yet. Upload one to get started.")).toBeTruthy();
  });

  it("renders wallpaper grid with size and refs badge", async () => {
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    });
    // size formatting: (2048/1024).toFixed(0) = 2 KB; (102400/1024).toFixed(0) = 100 KB
    expect(screen.getByText("Size: 2 KB")).toBeTruthy();
    expect(screen.getByText("Size: 100 KB")).toBeTruthy();
    // refs badge only when refs > 0
    expect(screen.getByTestId("gallery-refs-a.png")).toBeTruthy();
    expect(screen.queryByTestId("gallery-refs-b.jpg")).toBeNull();
    // thumbnail src
    const img = document.querySelector('img[src="/api/wallpapers/a.png"]') as HTMLImageElement;
    expect(img).toBeTruthy();
  });

  it("shows confirm dialog with refs info when deleting a referenced wallpaper", async () => {
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("gallery-delete-a.png"));
    expect(screen.getByText("Delete wallpaper?")).toBeTruthy();
    // appears both in the grid badge and the confirm dialog
    expect(screen.getAllByText("Used by 1 item(s)").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("References will be unbound automatically.")).toBeTruthy();
  });

  it("refreshes list after successful delete", async () => {
    mockDeleteWallpaper.mockResolvedValue({ success: true });
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("gallery-delete-a.png"));
    fireEvent.click(screen.getByTestId("gallery-confirm-delete"));
    await waitFor(() => {
      expect(mockDeleteWallpaper).toHaveBeenCalledWith("a.png");
    });
    // refresh after delete
    await waitFor(() => {
      expect(mockListWallpapers).toHaveBeenCalledTimes(2);
    });
  });

  it("keeps item and shows error when delete fails", async () => {
    mockDeleteWallpaper.mockRejectedValue(new Error("boom"));
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("gallery-delete-a.png"));
    fireEvent.click(screen.getByTestId("gallery-confirm-delete"));
    await waitFor(() => {
      expect(screen.getByText("Operation failed")).toBeTruthy();
    });
    // item remains in the list
    expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    // no refresh happened after a failed delete
    expect(mockListWallpapers).toHaveBeenCalledTimes(1);
  });

  it("uploads a file and refreshes list on success", async () => {
    mockUploadWallpaper.mockResolvedValue({ filename: "new.png" });
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    });
    const input = screen.getByTestId("gallery-upload-input") as HTMLInputElement;
    const file = new File(["x"], "new.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => {
      expect(mockUploadWallpaper).toHaveBeenCalledWith(file);
    });
    await waitFor(() => {
      expect(mockListWallpapers).toHaveBeenCalledTimes(2);
    });
  });

  it("shows upload error and keeps list when upload fails", async () => {
    mockUploadWallpaper.mockRejectedValue(new Error("network"));
    render(<WallpaperGalleryPage />);
    await waitFor(() => {
      expect(screen.getByTestId("gallery-item-a.png")).toBeTruthy();
    });
    const input = screen.getByTestId("gallery-upload-input") as HTMLInputElement;
    const file = new File(["x"], "bad.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => {
      expect(screen.getByText("Upload failed")).toBeTruthy();
    });
    expect(mockListWallpapers).toHaveBeenCalledTimes(1);
  });
});
