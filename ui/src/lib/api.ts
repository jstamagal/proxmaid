// Proxmaid API client

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8484';

interface ApiResponse<T> {
    ok: boolean;
    data?: T;
    error?: string;
}

// Token management
export function getToken(): string | null {
    if (typeof window === 'undefined') return null;
    return localStorage.getItem('proxmaid_token');
}

export function setToken(token: string): void {
    localStorage.setItem('proxmaid_token', token);
}

export function clearToken(): void {
    localStorage.removeItem('proxmaid_token');
}

async function fetchApi<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>> {
    const token = getToken();
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
    };
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    const res = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers: {
            ...headers,
            ...options?.headers,
        },
    });

    // Handle 401 - redirect to login
    if (res.status === 401 && !path.startsWith('/api/auth/')) {
        clearToken();
        if (typeof window !== 'undefined') {
            window.location.href = '/login';
        }
        return { ok: false, error: 'authentication required' };
    }

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

export interface Share {
    name: string;
    path: string;
    cache_policy: string;
    alloc_method: string;
    export_smb: boolean;
    export_nfs: boolean;
    security_mode: string;
    included_disks: number[];
    excluded_disks: number[];
    min_free_space: string;
    recycle_bin: boolean;
    read_users: string[];
    write_users: string[];
}

export interface Container {
    id: string;
    name: string;
    image: string;
    state: string;
    status: string;
    ports: { host_port: number; container_port: number; protocol: string }[];
    mounts: { source: string; destination: string; mode: string }[];
    created: string;
    labels: Record<string, string>;
}

export interface ContainerStats {
    cpu_percent: number;
    mem_usage: number;
    mem_limit: number;
    net_in: number;
    net_out: number;
}

export interface UPSStatus {
    online: boolean;
    battery_pct: number;
    runtime_sec: number;
    load: number;
}

export interface LogEntry {
    timestamp: string;
    unit: string;
    priority: number;
    message: string;
}

export interface NotifyConfig {
    discord?: { enabled: boolean; webhook_url: string };
    pushover?: { enabled: boolean; user_key: string; app_token: string };
    email?: { enabled: boolean; host: string; port: number; username: string; password: string; from: string; to: string };
    apprise?: { enabled: boolean; urls: string[] };
}

export interface NotifyEvent {
    type: string;
    message: string;
    severity: string;
    timestamp: string;
}

export interface ScheduledTask {
    name: string;
    schedule: string;
    enabled: boolean;
    last_run: string;
    next_run: string;
}

export interface AppTemplate {
    name: string;
    repository: string;
    icon: string;
    category: string;
    description: string;
    ports: { container: number; host: number; protocol: string }[];
    volumes: { container: string; host: string; description: string }[];
    env: { name: string; value: string; description: string }[];
    network: string;
    webui: string;
}

export interface DockerNetwork {
    id: string;
    name: string;
    driver: string;
    scope: string;
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
        health: () => fetchApi<Record<string, SmartHealth>>('/api/disks/health'),
        powerState: (device: string) => fetchApi<{ device: string; state: string }>(`/api/disks/power-state?device=${device}`),
        wipe: (device: string) => fetchApi<string>('/api/disks/wipe', { method: 'POST', body: JSON.stringify({ device, confirm: true }) }),
        partition: (device: string) => fetchApi<string>('/api/disks/partition', { method: 'POST', body: JSON.stringify({ device, confirm: true }) }),
        format: (device: string, fs_type = 'xfs') => fetchApi<string>('/api/disks/format', { method: 'POST', body: JSON.stringify({ device, fs_type, confirm: true }) }),
        mount: (device: string, mount_point: string) => fetchApi<string>('/api/disks/mount', { method: 'POST', body: JSON.stringify({ device, mount_point }) }),
        unmount: (mount_point: string) => fetchApi<string>('/api/disks/unmount', { method: 'POST', body: JSON.stringify({ mount_point }) }),
        spindown: (device: string) => fetchApi<string>('/api/disks/spindown', { method: 'POST', body: JSON.stringify({ device }) }),
        identify: (device: string) => fetchApi<string>('/api/disks/identify', { method: 'POST', body: JSON.stringify({ device }) }),
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
        ups: () => fetchApi<UPSStatus>('/api/system/ups'),
        logs: (lines = 100, unit = '') => fetchApi<LogEntry[]>(`/api/system/logs?lines=${lines}${unit ? `&unit=${unit}` : ''}`),
        timezone: () => fetchApi<{ timezone: string }>('/api/system/timezone'),
        setTimezone: (timezone: string) => fetchApi<string>('/api/system/timezone', { method: 'PUT', body: JSON.stringify({ timezone }) }),
    },

    shares: {
        list: () => fetchApi<Share[]>('/api/shares'),
        get: (name: string) => fetchApi<Share>(`/api/shares/${name}`),
        create: (data: Partial<Share>) => fetchApi<Share>('/api/shares', { method: 'POST', body: JSON.stringify(data) }),
        update: (name: string, data: Partial<Share>) => fetchApi<Share>(`/api/shares/${name}`, { method: 'PUT', body: JSON.stringify(data) }),
        delete: (name: string) => fetchApi<string>(`/api/shares/${name}`, { method: 'DELETE' }),
    },

    users: {
        list: () => fetchApi<string[]>('/api/users'),
        create: (username: string, password: string) => fetchApi<string>('/api/users', { method: 'POST', body: JSON.stringify({ username, password }) }),
        delete: (name: string) => fetchApi<string>(`/api/users/${name}`, { method: 'DELETE' }),
    },

    apps: {
        list: () => fetchApi<Container[]>('/api/apps'),
        start: (id: string) => fetchApi<string>(`/api/apps/${id}/start`, { method: 'POST' }),
        stop: (id: string) => fetchApi<string>(`/api/apps/${id}/stop`, { method: 'POST' }),
        remove: (id: string) => fetchApi<string>(`/api/apps/${id}`, { method: 'DELETE' }),
        logs: (id: string, lines = 100) => fetchApi<string>(`/api/apps/${id}/logs?lines=${lines}`),
        stats: (id: string) => fetchApi<ContainerStats>(`/api/apps/${id}/stats`),
        composeUp: (path: string) => fetchApi<string>('/api/apps/compose/up', { method: 'POST', body: JSON.stringify({ path }) }),
        composeDown: (path: string) => fetchApi<string>('/api/apps/compose/down', { method: 'POST', body: JSON.stringify({ path }) }),
        templates: (category = '', search = '') =>
            fetchApi<AppTemplate[]>(`/api/apps/templates?category=${category}&search=${search}`),
        install: (template: AppTemplate, overrides: Record<string, string>) =>
            fetchApi<string>('/api/apps/install', { method: 'POST', body: JSON.stringify({ template, overrides }) }),
        updates: () => fetchApi<Record<string, boolean>>('/api/apps/updates'),
        networks: () => fetchApi<DockerNetwork[]>('/api/apps/networks'),
        createNetwork: (name: string, driver: string) =>
            fetchApi<string>('/api/apps/networks', { method: 'POST', body: JSON.stringify({ name, driver }) }),
    },

    tasks: {
        list: () => fetchApi<ScheduledTask[]>('/api/tasks'),
        update: (name: string, schedule: string, enabled: boolean) =>
            fetchApi<string>(`/api/tasks/${name}`, { method: 'PUT', body: JSON.stringify({ schedule, enabled }) }),
        trigger: (name: string) => fetchApi<string>(`/api/tasks/${name}/run`, { method: 'POST' }),
    },

    notifications: {
        config: () => fetchApi<NotifyConfig>('/api/notifications/config'),
        updateConfig: (config: NotifyConfig) => fetchApi<string>('/api/notifications/config', { method: 'PUT', body: JSON.stringify(config) }),
        test: () => fetchApi<string>('/api/notifications/test', { method: 'POST' }),
        history: () => fetchApi<NotifyEvent[]>('/api/notifications/history'),
    },

    auth: {
        login: (username: string, password: string) =>
            fetchApi<{ token: string }>('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
        logout: () => fetchApi<string>('/api/auth/logout', { method: 'POST' }),
        me: () => fetchApi<{ username: string; expires_at: string }>('/api/auth/me'),
    },
};
