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
    virt_name: string;
    size_bytes: number;
    size_human: string;
    role: string;
    disk_id: string;
    reads: number;
    writes: number;
    errors: number;
}

export interface ArrayStatus {
    state: string;
    num_disks: number;
    num_invalid: number;
    synced: boolean;
    synced_time: string;
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

export interface PoolDevice {
    path: string;
    model: string;
    size: number;
    size_human: string;
}

export interface CachePool {
    name: string;
    devices: PoolDevice[];
    mount_point: string;
    fs_type: string;
    total_bytes: number;
    used_bytes: number;
    free_bytes: number;
    used_pct: number;
    status: string;
}

export interface MoverConfig {
    schedule: string;
    age_threshold: string;
    enabled: boolean;
}

export interface MoverStatus {
    running: boolean;
    last_run: string;
    next_run: string;
    bytes_moved: number;
    files_moved: number;
    progress: number;
    config: MoverConfig;
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

    cache: {
        pools: () => fetchApi<CachePool[]>('/api/cache/pools'),
        pool: (name: string) => fetchApi<CachePool>(`/api/cache/pools/${name}`),
        createPool: (data: { name: string; devices: string[]; fs_type: string }) =>
            fetchApi<CachePool>('/api/cache/pools', {
                method: 'POST',
                body: JSON.stringify(data),
            }),
        deletePool: (name: string) =>
            fetchApi<string>(`/api/cache/pools/${name}`, { method: 'DELETE' }),
        mover: () => fetchApi<MoverStatus>('/api/cache/mover'),
        runMover: () => fetchApi<string>('/api/cache/mover/run', { method: 'POST' }),
        updateMoverConfig: (config: MoverConfig) =>
            fetchApi<string>('/api/cache/mover/config', {
                method: 'PUT',
                body: JSON.stringify(config),
            }),
    },

    system: {
        module: () => fetchApi<{ loaded: boolean }>('/api/system/module'),
    },
};
