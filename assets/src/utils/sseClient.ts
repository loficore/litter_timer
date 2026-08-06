import type { TimerState } from '../types/api';
import { logInfo, logError } from './logger';

export type EventCallback = (state: TimerState) => void;
export type ErrorCallback = (error: Event) => void;
export type ConnectCallback = () => void;

/**
 * SSE 客户端，用于连接到后端 SSE 服务器并接收计时器状态更新
 */
export class SSEClient {
    private eventSource: EventSource | null = null;
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;
    private reconnectDelay = 1000;
    private onEvent: EventCallback | null = null;
    private onError: ErrorCallback | null = null;
    private onConnect: ConnectCallback | null = null;
    private closed = false;
    private baseUrl: string;

    /**
     * 构造函数，接受 SSE 服务器的基础 URL
     * @param {string} baseUrl SSE 服务器的基础 URL，例如 http://localhost:8013
     */
    constructor(baseUrl: string) {
        this.baseUrl = baseUrl;
    }

    /**
     * 连接到 SSE 服务器并设置事件回调
     * @param {EventCallback} onEvent - 当接收到新的计时器状态时调用的回调函数
     * @param {ErrorCallback} [onError] - 当连接发生错误时调用的可选回调函数
     * @param {ConnectCallback} [onConnect] - 当连接成功建立时调用的可选回调函数
     */
    connect(onEvent: EventCallback, onError?: ErrorCallback, onConnect?: ConnectCallback): void {
        this.onEvent = onEvent;
        this.onError = onError || null;
        this.onConnect = onConnect || null;
        this.createConnection();
    }

    private createConnection(): void {
        // SSE bypasses the Vite proxy (EventSource doesn't support upgrade).
        // In dev mode, VITE_API_URL points directly to the Go backend.
        // In production (WebView), window.location.origin serves both API and SSE.
        const sseBase = import.meta.env.VITE_API_URL || this.baseUrl;
        this.eventSource = new EventSource(`${sseBase}/api/events`);

        this.eventSource.onopen = () => {
            logInfo('SSE connection opened');
            this.reconnectAttempts = 0;
            // 连接成功时立即通知
            if (this.onConnect) {
                this.onConnect();
            }
        };

        this.eventSource.onmessage = (event: MessageEvent) => {
            const dataStr = String(event.data);
            if (dataStr.startsWith(':')) {
                logInfo('SSE heartbeat received');
                return;
            }
            logInfo(`SSE received: ${dataStr}`);
            try {
                const data = JSON.parse(dataStr) as Partial<TimerState>;
                if (this.onEvent && data && Object.keys(data).length > 0) {
                    this.onEvent(data as TimerState);
                }
            } catch (e) {
                const err = e instanceof Error ? e : new Error(String(e));
                logError('Failed to parse SSE data', err);
            }
        };

        this.eventSource.addEventListener('ping', () => {
            logInfo('SSE ping received');
        });

        this.eventSource.onerror = (error: Event) => {
            logError('SSE connection error');

            if (this.onError) {
                this.onError(error);
            }

            this.handleReconnect();
        };
    };

    private handleReconnect(): void {
        if (this.closed) {
            return;
        }

        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            logError('Max reconnection attempts reached');
            this.close();
            return;
        }

        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

        logInfo(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})...`);

        setTimeout(() => {
            if (this.closed) {
                return;
            }
            this.createConnection();
        }, delay);
    }

    /**
     * 关闭 SSE 连接
     */
    close(): void {
        this.closed = true;
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
        this.reconnectAttempts = 0;
    }

    /**
     * 检查 SSE 连接是否处于打开状态
     * @returns {boolean} 如果连接打开则返回 true，否则返回 false
     */
    isConnected(): boolean {
        return this.eventSource !== null && this.eventSource.readyState === EventSource.OPEN;
    }
}
