/**
 * SSE 连接管理 Hook
 * 统一管理 SSE 连接、自动重连和事件处理
 *
 * On Android (Wails): SSE is not available — Wails uses event bindings instead.
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { SSEClient } from "../utils/sseClient";
import { getAPIClient, type TimerState } from "../utils/apiClientSingleton";
import { logInfo, logError } from "../utils/logger";

const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

/**
 * useSSE 返回的连接状态与事件控制方法。
 */
export interface UseSSEReturn {
  /** SSE 连接是否已建立。 */
  isConnected: boolean;
  /** 建立 SSE 连接；Android 环境下不会执行连接操作。 */
  connect: () => void;
  /** 关闭当前 SSE 连接。 */
  disconnect: () => void;
  /** 最近一次接收到的计时器状态，没有消息时为 null。 */
  lastState: TimerState | null;
}

/**
 * 管理 SSE 连接、连接状态和计时器状态事件。
 *
 * @param onMessage - 收到计时器状态时调用的回调。
 * @param onError - 连接发生错误时调用的回调。
 * @returns SSE 连接状态、连接控制方法及最近状态。
 */
export const useSSE = (
  onMessage?: (data: TimerState) => void,
  onError?: (error: unknown) => void
): UseSSEReturn => {
  const sseClientRef = useRef<SSEClient | null>(null);
  const apiClientRef = useRef(getAPIClient());
  const [isConnected, setIsConnected] = useState(false);
  const [lastState, setLastState] = useState<TimerState | null>(null);

  const disconnect = useCallback(() => {
    if (sseClientRef.current) {
      sseClientRef.current.close();
      sseClientRef.current = null;
    }
    setIsConnected(false);
  }, []);

  const connect = useCallback(() => {
    if (isAndroid) return;

    if (sseClientRef.current?.isConnected()) {
      return;
    }

    const baseUrl = apiClientRef.current.baseUrl;
    sseClientRef.current = new SSEClient(baseUrl);

    sseClientRef.current.connect(
      (timerState) => {
        setLastState(timerState);
        onMessage?.(timerState);
      },
      (error) => {
        setIsConnected(false);
        let errorMsg = "未知错误";
        if (error instanceof Error) {
          errorMsg = error.message;
        } else if (typeof error === "string") {
          errorMsg = error;
        }
        logError(`SSE 连接错误: ${errorMsg}`);
        onError?.(error);
      },
      // 连接成功时的回调
      () => {
        setIsConnected(true);
        logInfo("SSE 连接已建立");
      }
    );
  }, [onMessage, onError]);

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    isConnected,
    connect,
    disconnect,
    lastState,
  };
};
