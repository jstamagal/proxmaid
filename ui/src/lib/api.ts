// Proxmaid API client

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8484';

interface ApiResponse<T> {
    ok: boolean;
    data?: T;
    error?: string;
}

async function fetchApi<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>> {
    const res = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...options?.headers,
        },
    });
    return res.json();
}

// Types matching Go API structs
export interface DiskInfo {
    slot: number;
    status: string;
    device_name: string;
    size_bytes: number;
    role: string;
}

export interface ArrayStatus {
    state: string;
    num_disks: number;
    num_invalid: number;
    synced: boolean;
    resync_active: boolean;
    resync_pct: number;
    disks: DiskInfo[];
}

export interface SystemDisk {
    name: string;
    path: string;
    size: number;
    size_human: string;
    model: string;
    serial: string;
    type: string;
    mountpoint: string;
    fstype: string;
    rotational: boolean;
    disk_id: string;
}

export interface SmartHealth {
    device: string;
    healthy: boolean;
    temperature: number;
    power_on_hours: number;
    raw_output: string;
}

// API functions
export const api = {
    health: () => fetchApi<string>('/api/health'),

    array: {
        status: () => fetchApi<ArrayStatus>('/api/array/status'),
        start: () => fetchApi<string>('/api/array/start', { method: 'POST' }),
        stop: () => fetchApi<string>('/api/array/stop', { method: 'POST' }),
        check: (mode = 'CORRECT') => fetchApi<string>(`/api/array/check?mode=${mode}`, { method: 'POST' }),
    },

    disks: {
        list: () => fetchApi<SystemDisk[]>('/api/disks'),
        smart: (device: string) => fetchApi<SmartHealth>(`/api/disks/smart?device=${device}`),
    },

    system: {
        module: () => fetchApi<{ loaded: boolean }>('/api/system/module'),
    },
};
