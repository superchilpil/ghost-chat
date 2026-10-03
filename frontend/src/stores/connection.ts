import type { Platform } from '@bindings/ghost-chat/internal/chat/models.js';

import { create } from 'zustand';

type ConnectedMap = Partial<Record<Platform, boolean>>;
type InputMap = Partial<Record<Platform, string>>;
type TransportMap = Partial<Record<Platform, string>>;

interface ConnectionState {
    connected: ConnectedMap;
    inputs: InputMap;
    transports: TransportMap;
    setConnected: (platform: Platform, value: boolean) => void;
    setInput: (platform: Platform, value: string) => void;
    setTransport: (platform: Platform, value: string) => void;
}

export const useConnectionStore = create<ConnectionState>((set) => ({
    connected: {},
    inputs: {},
    transports: {},

    setConnected: (platform, value) =>
        set((s) => ({
            connected: { ...s.connected, [platform]: value },
        })),

    setInput: (platform, value) =>
        set((s) => ({
            inputs: { ...s.inputs, [platform]: value },
        })),

    setTransport: (platform, value) =>
        set((s) => ({
            transports: { ...s.transports, [platform]: value },
        })),
}));
