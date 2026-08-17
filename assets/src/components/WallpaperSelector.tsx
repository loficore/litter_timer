/**
 * 壁纸选择控件组件
 * 内嵌在表单中使用，支持渐变、纯色、图片三种壁纸类型
 */

import { useState, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { getAPIClient } from "../utils/apiClientSingleton";
import { WALLPAPER_FALLBACK_GRADIENT, WALLPAPER_LOCAL_PREFIX, resolveWallpaperUrl } from "../utils/constants";

interface WallpaperSelectorProps {
  /** 当前壁纸值（渐变/颜色/图片 URL） */
  value: string;
  /** 壁纸变更回调 */
  onChange: (wallpaper: string) => void;
}

const GRADIENTS = [
  { id: "", value: "" },
  { id: "sunset", value: "linear-gradient(135deg, #f97316 0%, #ec4899 50%, #8b5cf6 100%)" },
  { id: "ocean", value: "linear-gradient(135deg, #0ea5e9 0%, #14b8a6 50%, #22c55e 100%)" },
  { id: "night", value: "linear-gradient(135deg, #1e1b4b 0%, #312e81 50%, #4c1d95 100%)" },
  { id: "forest", value: "linear-gradient(135deg, #14532d 0%, #166534 50%, #15803d 100%)" },
  { id: "dawn", value: "linear-gradient(135deg, #fdf4ff 0%, #f0abfc 50%, #c084fc 100%)" },
  { id: "aurora", value: "linear-gradient(135deg, #134e4a 0%, #0d9488 50%, #2dd4bf 100%)" },
  { id: "coral", value: "linear-gradient(135deg, #7f1d1d 0%, #be123c 50%, #f43f5e 100%)" },
  { id: "mint", value: "linear-gradient(135deg, #042f2e 0%, #134e4a 50%, #14b8a6 100%)" },
];

const COLORS = [
  "#121212",
  "#1e1e2e",
  "#2d1b4e",
  "#1b2838",
  "#0f172a",
  "#18181b",
];

export const WallpaperSelector: FunctionalComponent<WallpaperSelectorProps> = ({
  value,
  onChange,
}) => {
  const [wallpaperType, setWallpaperType] = useState<"gradient" | "color" | "image">("gradient");
  const [imageUrl, setImageUrl] = useState("");
  const [localImages, setLocalImages] = useState<{ name: string }[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [previewFailed, setPreviewFailed] = useState(false);

  const api = getAPIClient();

  const isLocal = value.startsWith(WALLPAPER_LOCAL_PREFIX);
  const isGradient = value.startsWith("linear");
  const isColor = !isGradient && value.startsWith("#");
  const isImage = !isGradient && !isColor && value.length > 0;

  useEffect(() => {
    setPreviewFailed(false);
    if (isGradient) {
      setWallpaperType("gradient");
    } else if (isColor) {
      setWallpaperType("color");
    } else if (isImage) {
      setWallpaperType("image");
      if (isLocal) {
        setImageUrl(value.slice(WALLPAPER_LOCAL_PREFIX.length));
      } else {
        setImageUrl(value);
      }
    } else {
      setWallpaperType("gradient");
    }
  }, [value, isGradient, isColor, isImage, isLocal]);

  useEffect(() => {
    if (wallpaperType === "image") {
      api.listWallpapers().then(setLocalImages).catch(() => setLocalImages([]));
    }
  }, [wallpaperType]);

  const handleGradientSelect = (gradientValue: string) => {
    onChange(gradientValue);
  };

  const handleColorSelect = (colorValue: string) => {
    onChange(colorValue);
  };

  const handleImageUrlChange = (url: string) => {
    setImageUrl(url);
  };

  const handleSubmitUrl = async () => {
    const url = imageUrl.trim();
    if (!url) return;
    setUploading(true);
    setUploadError(null);
    try {
      const result = await api.fetchWallpaperByUrl(url);
      onChange(`local:${result.filename}`);
      setLocalImages(prev => [...prev, { name: result.filename }]);
    } catch {
      setUploadError(t("modal.upload_fail"));
    } finally {
      setUploading(false);
    }
  };

  const handleUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    setUploading(true);
    setUploadError(null);
    try {
      const result = await api.uploadWallpaper(file);
      const localValue = `${WALLPAPER_LOCAL_PREFIX}${result.filename}`;
      onChange(localValue);
      setLocalImages((prev) => [...prev, { name: result.filename }]);
    } catch {
      setUploadError(t("modal.upload_fail"));
    } finally {
      setUploading(false);
      input.value = "";
    }
  };

  const handleSelectLocal = (filename: string) => {
    onChange(`${WALLPAPER_LOCAL_PREFIX}${filename}`);
  };

  return (
    <div className="form-control mb-6">
      <label className="label">
        <span className="label-text">{t("modal.wallpaper")}</span>
      </label>

      <div className="tabs tabs-boxed mb-3">
        <button
          type="button"
          className={`tab ${wallpaperType === "gradient" ? "tab-active" : ""}`}
          onClick={() => setWallpaperType("gradient")}
        >
          {t("modal.gradient")}
        </button>
        <button
          type="button"
          className={`tab ${wallpaperType === "color" ? "tab-active" : ""}`}
          onClick={() => setWallpaperType("color")}
        >
          {t("modal.solid")}
        </button>
        <button
          type="button"
          className={`tab ${wallpaperType === "image" ? "tab-active" : ""}`}
          onClick={() => setWallpaperType("image")}
        >
          {t("modal.image")}
        </button>
      </div>

      {wallpaperType === "gradient" && (
        <div className="grid grid-cols-4 gap-2">
          {GRADIENTS.map((g) => (
            <button
              key={g.id}
              type="button"
              className={`h-12 rounded-lg border-2 transition-all ${
                (value === g.value || (!value && !g.value))
                  ? "border-primary ring-2 ring-primary/30"
                  : "border-[color:color-mix(in_oklab,var(--my-outline)_56%,transparent)] hover:border-[color:color-mix(in_oklab,var(--my-outline)_78%,transparent)]"
              }`}
              style={g.value ? { background: g.value } : { background: "#2a2a2a" }}
              onClick={() => handleGradientSelect(g.value)}
              title={t(`modal.gradient_${g.id}`)}
            >
              {!g.value && (
                <span className="text-xs text-base-content/50">{t("modal.none")}</span>
              )}
            </button>
          ))}
        </div>
      )}

      {wallpaperType === "color" && (
        <div className="flex gap-2 flex-wrap">
          {COLORS.map((c) => (
            <button
              key={c}
              type="button"
              className={`w-10 h-10 rounded-full border-2 transition-all ${
                value === c
                  ? "border-primary ring-2 ring-primary/30"
                  : "border-[color:color-mix(in_oklab,var(--my-outline)_56%,transparent)] hover:scale-110"
              }`}
              style={{ backgroundColor: c }}
              onClick={() => handleColorSelect(c)}
            />
          ))}
          <input
            type="color"
            className="w-10 h-10 rounded-full cursor-pointer"
            value={value.startsWith("#") ? value : "#121212"}
            onChange={(e) => handleColorSelect((e.target as HTMLInputElement).value)}
          />
        </div>
      )}

      {wallpaperType === "image" && (
        <div className="space-y-2">
          <div className="flex gap-2">
            <input
              type="text"
              className="my-input flex-1 text-sm"
              placeholder={t("modal.image_url_placeholder")}
              value={isLocal ? "" : imageUrl}
              onInput={(e) => handleImageUrlChange((e.target as HTMLInputElement).value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  void handleSubmitUrl();
                }
              }}
            />
            <button
              type="button"
              className="btn btn-sm btn-outline"
              onClick={() => { void handleSubmitUrl(); }}
              disabled={uploading}
            >
              {t("modal.add")}
            </button>
          </div>
          <div className="flex items-center gap-2">
            <label className="btn btn-sm btn-outline cursor-pointer">
              {uploading ? t("modal.upload_progress") : t("modal.upload_image")}
              <input
                type="file"
                accept="image/*"
                className="hidden"
                onChange={(e) => { void handleUpload(e); }}
                disabled={uploading}
              />
            </label>
            {uploadError && <span className="text-xs text-red-400">{uploadError}</span>}
          </div>
          {value && !isGradient && !isColor && (
            <div className="h-20 rounded-lg overflow-hidden bg-[color:color-mix(in_oklab,var(--my-surface-strong)_86%,transparent)] border border-[color:color-mix(in_oklab,var(--my-outline)_42%,transparent)]">
              {previewFailed ? (
                <div
                  data-testid="selector-preview-fallback"
                  className="w-full h-full"
                  style={{ background: WALLPAPER_FALLBACK_GRADIENT }}
                />
              ) : (
                <img
                  data-testid="selector-preview-img"
                  src={resolveWallpaperUrl(value)}
                  alt={t("modal.preview")}
                  className="w-full h-full object-cover"
                  onError={() => setPreviewFailed(true)}
                />
              )}
            </div>
          )}
          {localImages.length > 0 && (
            <div>
              <div className="text-xs text-base-content/60 mb-1">{t("modal.local_images")}</div>
              <div className="grid grid-cols-3 gap-2 max-h-40 overflow-y-auto">
                {localImages.map((img) => (
                  <div
                    key={img.name}
                    className={`relative group rounded-lg overflow-hidden border-2 cursor-pointer ${
                      isLocal && value.slice(WALLPAPER_LOCAL_PREFIX.length) === img.name
                        ? "border-primary ring-2 ring-primary/30"
                        : "border-[color:color-mix(in_oklab,var(--my-outline)_56%,transparent)] hover:border-primary/50"
                    }`}
                    onClick={() => handleSelectLocal(img.name)}
                  >
                    <img
                      src={`/api/wallpapers/${img.name}`}
                      alt={img.name}
                      className="w-full h-16 object-cover"
                    />
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};