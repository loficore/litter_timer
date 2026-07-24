import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { WallpaperModal } from "../../components/WallpaperModal";

vi.mock("../../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "modal.select_wallpaper": "Select Wallpaper",
      "modal.solid": "Solid",
      "modal.image": "Image",
      "modal.color_picker": "Color Picker",
      "modal.current_wallpaper": "Current Wallpaper",
      "modal.image_url": "Image URL",
      "modal.enter_url": "Enter URL",
      "modal.add": "Add",
      "modal.upload_progress": "Uploading...",
      "modal.upload_image": "Upload",
      "modal.upload_fail": "Fetch failed",
      "modal.local_images": "Local Images",
      "modal.no_images": "No images",
      "modal.delete_image": "Delete",
      "modal.delete_confirm": "Confirm Delete",
      "modal.delete_confirm_desc": "Are you sure?",
      "modal.cancel": "Cancel",
      "modal.confirm": "Confirm",
    };
    return translations[key] || key;
  },
}));

const mockFetchWallpaperByUrl = vi.fn();
const mockListWallpapers = vi.fn();

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    fetchWallpaperByUrl: mockFetchWallpaperByUrl,
    listWallpapers: mockListWallpapers,
    uploadWallpaper: vi.fn(),
    deleteWallpaper: vi.fn(),
  }),
}));

describe("WallpaperModal", () => {
  const mockOnChange = vi.fn();
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockListWallpapers.mockResolvedValue([]);
  });

  it("fetches wallpaper by URL on Add click and calls onChange with local: prefix, never raw URL", async () => {
    mockFetchWallpaperByUrl.mockResolvedValue({ filename: "123_test.png" });

    render(
      <WallpaperModal
        isOpen={true}
        value=""
        onChange={mockOnChange}
        onClose={mockOnClose}
      />
    );

    // Switch to Image tab
    fireEvent.click(screen.getByText("Image"));

    // Type a URL
    const input = screen.getByPlaceholderText("Enter URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "https://example.com/bg.jpg" } });

    // Click Add
    const addButton = screen.getByText("Add");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(mockFetchWallpaperByUrl).toHaveBeenCalledWith("https://example.com/bg.jpg");
    });

    expect(mockOnChange).toHaveBeenCalledWith("local:123_test.png");
    // MUST NOT call onChange with raw URL
    expect(mockOnChange).not.toHaveBeenCalledWith("https://example.com/bg.jpg");
  });

  it("shows error and does NOT call onChange with raw URL when fetchWallpaperByUrl fails", async () => {
    mockFetchWallpaperByUrl.mockRejectedValue(new Error("Network error"));

    render(
      <WallpaperModal
        isOpen={true}
        value=""
        onChange={mockOnChange}
        onClose={mockOnClose}
      />
    );

    fireEvent.click(screen.getByText("Image"));

    const input = screen.getByPlaceholderText("Enter URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "https://bad.example.com/bg.jpg" } });

    fireEvent.click(screen.getByText("Add"));

    await waitFor(() => {
      expect(screen.getByText("Fetch failed")).toBeTruthy();
    });

    // onChange must NOT be called with raw URL
    expect(mockOnChange).not.toHaveBeenCalledWith("https://bad.example.com/bg.jpg");
    expect(mockOnChange).not.toHaveBeenCalled();
  });

  it("does NOT call onChange when just typing in the URL input", () => {
    render(
      <WallpaperModal
        isOpen={true}
        value=""
        onChange={mockOnChange}
        onClose={mockOnClose}
      />
    );

    fireEvent.click(screen.getByText("Image"));

    const input = screen.getByPlaceholderText("Enter URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "https://example.com/test.jpg" } });

    // onChange should NOT be called just from typing
    expect(mockOnChange).not.toHaveBeenCalled();
  });
});