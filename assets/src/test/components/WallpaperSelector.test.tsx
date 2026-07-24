import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { WallpaperSelector } from "../../components/WallpaperSelector";

vi.mock("../../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "modal.wallpaper": "Wallpaper",
      "modal.gradient": "Gradient",
      "modal.solid": "Solid",
      "modal.image": "Image",
      "modal.none": "None",
      "modal.preview": "Preview",
      "modal.image_url_placeholder": "Enter image URL",
      "modal.gradient_": "None",
      "modal.gradient_sunset": "Sunset",
      "modal.gradient_ocean": "Ocean",
      "modal.gradient_night": "Night",
      "modal.gradient_forest": "Forest",
      "modal.gradient_dawn": "Dawn",
      "modal.gradient_aurora": "Aurora",
      "modal.gradient_coral": "Coral",
      "modal.gradient_mint": "Mint",
      "upload_fail": "Upload failed",
      "add": "Add",
      "upload_image": "Upload",
      "upload_progress": "Uploading...",
    };
    return translations[key] || key;
  },
}));

const mockFetchWallpaperByUrl = vi.fn();
const mockListWallpapers = vi.fn().mockResolvedValue([]);
const mockUploadWallpaper = vi.fn();
const mockDeleteWallpaper = vi.fn();

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    fetchWallpaperByUrl: mockFetchWallpaperByUrl,
    listWallpapers: mockListWallpapers,
    uploadWallpaper: mockUploadWallpaper,
    deleteWallpaper: mockDeleteWallpaper,
  }),
  APIClient: class {},
}));

describe("WallpaperSelector", () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockListWallpapers.mockResolvedValue([]);
  });

  it("应该渲染壁纸选择器", () => {
    const { container } = render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    expect(container.querySelector(".form-control")).toBeTruthy();
  });

  it("应该显示标签", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    expect(screen.getByText("Wallpaper")).toBeTruthy();
  });

  it("应该渲染三个 Tab", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    expect(screen.getByText("Gradient")).toBeTruthy();
    expect(screen.getByText("Solid")).toBeTruthy();
    expect(screen.getByText("Image")).toBeTruthy();
  });

  it("默认应该显示渐变选择", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    const gradientTab = screen.getByText("Gradient").closest("button");
    expect(gradientTab?.className).toContain("tab-active");
  });

  it("渐变值应该显示渐变选择面板", () => {
    render(
      <WallpaperSelector
        value="linear-gradient(135deg, #f97316 0%, #ec4899 50%, #8b5cf6 100%)"
        onChange={mockOnChange}
      />
    );

    const tabs = screen.getAllByRole("button");
    const gradientTab = screen.getByText("Gradient").closest("button");
    expect(gradientTab?.className).toContain("tab-active");
  });

  it("颜色值应该显示颜色选择面板", () => {
    render(
      <WallpaperSelector
        value="#121212"
        onChange={mockOnChange}
      />
    );

    const solidTab = screen.getByText("Solid").closest("button");
    expect(solidTab?.className).toContain("tab-active");
  });

  it("图片 URL 应该显示图片输入面板", () => {
    render(
      <WallpaperSelector
        value="https://example.com/image.jpg"
        onChange={mockOnChange}
      />
    );

    const imageTab = screen.getByText("Image").closest("button");
    expect(imageTab?.className).toContain("tab-active");
  });

  it("点击颜色 Tab 应该切换到颜色面板", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Solid"));
    const solidTab = screen.getByText("Solid").closest("button");
    expect(solidTab?.className).toContain("tab-active");
  });

  it("点击图片 Tab 应该切换到图片面板", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Image"));
    const imageTab = screen.getByText("Image").closest("button");
    expect(imageTab?.className).toContain("tab-active");

    expect(screen.getByPlaceholderText("Enter image URL")).toBeTruthy();
  });

  it("选择渐变时应该调用 onChange", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    const sunsetButton = buttons.find(b => b.title === "Sunset");
    if (sunsetButton) {
      fireEvent.click(sunsetButton);
      expect(mockOnChange).toHaveBeenCalled();
    }
  });

  it("选择颜色时应该调用 onChange", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Solid"));

    const colorButtons = screen.getAllByRole("button");
    const firstColorButton = colorButtons.find(b => b.type === "button" && !b.className.includes("tab"));
    if (firstColorButton) {
      fireEvent.click(firstColorButton);
      expect(mockOnChange).toHaveBeenCalled();
    }
  });

  it("输入图片 URL 时不应该自动调用 onChange，点击 Add 按钮后才调用", async () => {
    mockFetchWallpaperByUrl.mockResolvedValue({ filename: "456_test.png" });

    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Image"));

    const input = screen.getByPlaceholderText("Enter image URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "https://example.com/test.jpg" } });

    // typing alone does NOT call onChange
    expect(mockOnChange).not.toHaveBeenCalled();

    // click the Add button
    const addButton = screen.getByText("Add");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalledWith("local:456_test.png");
    });
  });

  it("空图片 URL 不应该调用 onChange", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Image"));

    const input = screen.getByPlaceholderText("Enter image URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "   " } });

    expect(mockOnChange).not.toHaveBeenCalled();
  });

  it("fetchWallpaperByUrl 失败时显示错误，不调用 onChange", async () => {
    mockFetchWallpaperByUrl.mockRejectedValue(new Error("Network error"));

    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Image"));

    const input = screen.getByPlaceholderText("Enter image URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "https://bad.com/image.jpg" } });

    const addButton = screen.getByText("Add");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(screen.getByText("Upload failed")).toBeTruthy();
    });

    // onChange should NOT be called with the raw URL
    expect(mockOnChange).not.toHaveBeenCalled();
  });

  it("仅输入 URL 不点击 Add 不应调用 onChange", () => {
    render(
      <WallpaperSelector
        value=""
        onChange={mockOnChange}
      />
    );

    fireEvent.click(screen.getByText("Image"));

    const input = screen.getByPlaceholderText("Enter image URL") as HTMLInputElement;
    fireEvent.input(input, { target: { value: "https://example.com/photo.jpg" } });

    // no onChange call at all — decoupled from input
    expect(mockOnChange).not.toHaveBeenCalled();
  });
});