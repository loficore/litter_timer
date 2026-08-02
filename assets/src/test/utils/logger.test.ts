import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { logInfo, logError, logSuccess, logPerf, logLifecycle, logOperation, logNetwork } from "../../utils/logger";

describe("logger utils", () => {
  let mockFetch: ReturnType<typeof vi.fn>;
  let mockConsoleLog: ReturnType<typeof vi.fn>;
  let mockConsoleError: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch = vi.fn().mockResolvedValue({ ok: true });
    mockConsoleLog = vi.fn();
    mockConsoleError = vi.fn();
    vi.stubGlobal("fetch", mockFetch);
    vi.stubGlobal("console", {
      ...console,
      log: mockConsoleLog,
      error: mockConsoleError,
    });
    vi.stubGlobal("window", {
      webui: undefined,
      location: { search: "" },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  describe("logInfo", () => {
    it("应该记录日志到 console", () => {
      logInfo("test message");
      expect(mockConsoleLog).toHaveBeenCalled();
    });

    it("应该发送到后端", () => {
      logInfo("test message");

      expect(mockFetch).toHaveBeenCalledWith("/api/log", expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: expect.stringContaining("test message"),
      }));
    });
  });

  describe("logError", () => {
    it("应该记录错误消息和堆栈", () => {
      const error = new Error("test error");
      logError("Operation failed", error);
      expect(mockConsoleError).toHaveBeenCalled();
    });

    it("应该处理无错误对象的场景", () => {
      logError("Simple error message");
      expect(mockConsoleError).toHaveBeenCalled();
    });
  });

  describe("logToBackend routing", () => {
    it("with wails bridge: routes via wails.log, skips fetch", () => {
      const wailsLogMock = vi.fn();
      vi.stubGlobal("window", {
        wails: { log: wailsLogMock },
        webui: undefined,
        location: { search: "" },
      });

      logError("wails test error");

      expect(wailsLogMock).toHaveBeenCalledWith("error", "wails test error");
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("without wails bridge: falls back to fetch /api/log", () => {
      // beforeEach stubs window without wails — desktop fallback
      logInfo("desktop test message");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/log",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: expect.stringContaining("desktop test message"),
        })
      );
    });
  });

  describe("logSuccess", () => {
    it("应该记录成功日志", () => {
      logSuccess("Operation completed");
      expect(mockConsoleLog).toHaveBeenCalled();
    });
  });

  describe("logPerf", () => {
    it("WebView 环境应该记录 perf", () => {
      vi.stubGlobal("window", {
        webui: { call: vi.fn() },
        location: { search: "" },
      });

      logPerf("test scope", { duration: 100 });
      expect(mockConsoleLog).toHaveBeenCalled();
    });
  });

  describe("便捷函数", () => {
    it("logLifecycle 应该使用 lifecycle 类别", () => {
      logLifecycle("app started");
      expect(mockConsoleLog).toHaveBeenCalled();
    });

    it("logOperation 应该使用 operation 类别", () => {
      logOperation("user clicked");
      expect(mockConsoleLog).toHaveBeenCalled();
    });

    it("logNetwork 应该使用 network 类别", () => {
      logNetwork("api called");
      expect(mockConsoleLog).toHaveBeenCalled();
    });
  });
});
