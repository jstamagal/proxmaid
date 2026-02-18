'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import { api, type UPSStatus, type NotifyConfig, type ScheduledTask } from '@/lib/api';

export default function SettingsPage() {
    const [timezone, setTimezone] = useState('');
    const [ups, setUps] = useState<UPSStatus | null>(null);
    const [notifyConfig, setNotifyConfig] = useState<NotifyConfig>({});
    const [moduleLoaded, setModuleLoaded] = useState(false);
    const [tasks, setTasks] = useState<ScheduledTask[]>([]);
    const [loading, setLoading] = useState(true);

    const fetchData = async () => {
        try {
            const [tzRes, upsRes, modRes, notifyRes, tasksRes] = await Promise.all([
                api.system.timezone(),
                api.system.ups(),
                api.system.module(),
                api.notifications.config(),
                api.tasks.list(),
            ]);
            if (tzRes.ok && tzRes.data) setTimezone(tzRes.data.timezone);
            if (upsRes.ok && upsRes.data) setUps(upsRes.data);
            if (modRes.ok && modRes.data) setModuleLoaded(modRes.data.loaded);
            if (notifyRes.ok && notifyRes.data) setNotifyConfig(notifyRes.data);
            if (tasksRes.ok && tasksRes.data) setTasks(tasksRes.data);
        } catch { }
        setLoading(false);
    };

    useEffect(() => {
        fetchData();
    }, []);

    const handleTestNotify = async () => {
        await api.notifications.test();
    };

    const handleToggleTask = async (task: ScheduledTask) => {
        await api.tasks.update(task.name, task.schedule, !task.enabled);
        fetchData();
    };

    const handleRunTask = async (name: string) => {
        await api.tasks.trigger(name);
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', minHeight: '100vh' }}>
                <Sidebar />
                <main style={{ marginLeft: '240px', flex: 1, padding: '32px' }}>
                    <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading settings...</div>
                </main>
            </div>
        );
    }

    return (
        <div style={{ display: 'flex', minHeight: '100vh' }}>
            <Sidebar />

            <main style={{ marginLeft: '240px', flex: 1, padding: '32px', maxWidth: '900px' }}>
                {/* Header */}
                <div style={{ marginBottom: '28px' }}>
                    <h1 style={{
                        fontSize: '28px',
                        fontWeight: 800,
                        background: 'linear-gradient(135deg, #e2e8f0, #94a3b8)',
                        WebkitBackgroundClip: 'text',
                        WebkitTextFillColor: 'transparent',
                        marginBottom: '4px',
                    }}>
                        Settings
                    </h1>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                        System configuration and notification providers
                    </p>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
                    {/* System Info */}
                    <div className="glass-card" style={{ padding: '24px' }}>
                        <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '16px' }}>System Info</h3>
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', fontSize: '13px' }}>
                            <div>
                                <span style={{ color: 'var(--color-text-muted)' }}>Proxmaid Version:</span>
                                <span style={{ marginLeft: '8px', fontWeight: 600 }}>0.1.0</span>
                            </div>
                            <div>
                                <span style={{ color: 'var(--color-text-muted)' }}>NonRAID Module:</span>
                                <span style={{
                                    marginLeft: '8px',
                                    fontWeight: 600,
                                    color: moduleLoaded ? 'var(--color-success)' : 'var(--color-danger)',
                                }}>
                                    {moduleLoaded ? 'Loaded' : 'Not Loaded'}
                                </span>
                            </div>
                            <div>
                                <span style={{ color: 'var(--color-text-muted)' }}>Timezone:</span>
                                <span style={{ marginLeft: '8px', fontWeight: 600 }}>{timezone}</span>
                            </div>
                            <div>
                                <span style={{ color: 'var(--color-text-muted)' }}>Mode:</span>
                                <span style={{ marginLeft: '8px', fontWeight: 600, color: 'var(--color-warning)' }}>Mock</span>
                            </div>
                        </div>
                    </div>

                    {/* UPS Status */}
                    {ups && (
                        <div className="glass-card" style={{ padding: '24px' }}>
                            <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '16px' }}>UPS Status</h3>
                            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '16px' }}>
                                <div style={{ textAlign: 'center' }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase' }}>Power</div>
                                    <div style={{
                                        fontSize: '18px',
                                        fontWeight: 700,
                                        color: ups.online ? 'var(--color-success)' : 'var(--color-danger)',
                                    }}>
                                        {ups.online ? 'Online' : 'Battery'}
                                    </div>
                                </div>
                                <div style={{ textAlign: 'center' }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase' }}>Battery</div>
                                    <div style={{ fontSize: '18px', fontWeight: 700 }}>{ups.battery_pct}%</div>
                                </div>
                                <div style={{ textAlign: 'center' }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase' }}>Runtime</div>
                                    <div style={{ fontSize: '18px', fontWeight: 700 }}>{Math.floor(ups.runtime_sec / 60)}m</div>
                                </div>
                                <div style={{ textAlign: 'center' }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase' }}>Load</div>
                                    <div style={{ fontSize: '18px', fontWeight: 700 }}>{ups.load}%</div>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Notification Providers */}
                    <div className="glass-card" style={{ padding: '24px' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                            <h3 style={{ fontSize: '16px', fontWeight: 700 }}>Notifications</h3>
                            <button
                                onClick={handleTestNotify}
                                style={{
                                    padding: '6px 14px',
                                    fontSize: '12px',
                                    background: 'rgba(59, 130, 246, 0.1)',
                                    color: 'var(--color-accent)',
                                    border: '1px solid rgba(59, 130, 246, 0.2)',
                                    borderRadius: '6px',
                                    cursor: 'pointer',
                                }}
                            >
                                Send Test
                            </button>
                        </div>

                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                            {[
                                { name: 'Discord', enabled: notifyConfig.discord?.enabled, color: '#5865F2' },
                                { name: 'Pushover', enabled: notifyConfig.pushover?.enabled, color: '#249DF1' },
                                { name: 'Email', enabled: notifyConfig.email?.enabled, color: '#10B981' },
                                { name: 'Apprise', enabled: notifyConfig.apprise?.enabled, color: '#F59E0B' },
                            ].map(provider => (
                                <div key={provider.name} style={{
                                    padding: '14px 16px',
                                    background: 'var(--color-bg-secondary)',
                                    borderRadius: '10px',
                                    border: '1px solid var(--color-border)',
                                    display: 'flex',
                                    justifyContent: 'space-between',
                                    alignItems: 'center',
                                }}>
                                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                                        <div style={{
                                            width: '8px',
                                            height: '8px',
                                            borderRadius: '50%',
                                            background: provider.enabled ? provider.color : 'var(--color-text-muted)',
                                        }} />
                                        <span style={{ fontSize: '13px', fontWeight: 600 }}>{provider.name}</span>
                                    </div>
                                    <span style={{
                                        fontSize: '11px',
                                        color: provider.enabled ? 'var(--color-success)' : 'var(--color-text-muted)',
                                    }}>
                                        {provider.enabled ? 'Enabled' : 'Disabled'}
                                    </span>
                                </div>
                            ))}
                        </div>
                    </div>
                    {/* Scheduled Tasks */}
                    {tasks.length > 0 && (
                        <div className="glass-card" style={{ padding: '24px' }}>
                            <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '16px' }}>Scheduled Tasks</h3>
                            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                                {tasks.map(task => (
                                    <div key={task.name} style={{
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'space-between',
                                        padding: '12px 16px',
                                        background: 'var(--color-bg-secondary)',
                                        borderRadius: '10px',
                                        border: '1px solid var(--color-border)',
                                    }}>
                                        <div style={{ flex: 1 }}>
                                            <div style={{ fontSize: '13px', fontWeight: 600 }}>{task.name.replace(/_/g, ' ')}</div>
                                            <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginTop: '2px' }}>
                                                Schedule: {task.schedule} | Last run: {task.last_run ? new Date(task.last_run).toLocaleString() : 'Never'}
                                            </div>
                                        </div>
                                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                            <button
                                                onClick={() => handleRunTask(task.name)}
                                                style={{
                                                    padding: '4px 10px',
                                                    fontSize: '11px',
                                                    background: 'rgba(59, 130, 246, 0.1)',
                                                    color: 'var(--color-accent)',
                                                    border: '1px solid rgba(59, 130, 246, 0.2)',
                                                    borderRadius: '6px',
                                                    cursor: 'pointer',
                                                }}
                                            >
                                                Run Now
                                            </button>
                                            <div
                                                onClick={() => handleToggleTask(task)}
                                                style={{
                                                    width: '36px',
                                                    height: '20px',
                                                    borderRadius: '10px',
                                                    background: task.enabled ? 'var(--color-accent)' : 'var(--color-bg-secondary)',
                                                    border: `1px solid ${task.enabled ? 'var(--color-accent)' : 'var(--color-border)'}`,
                                                    cursor: 'pointer',
                                                    position: 'relative',
                                                }}
                                            >
                                                <div style={{
                                                    width: '14px',
                                                    height: '14px',
                                                    borderRadius: '50%',
                                                    background: '#fff',
                                                    position: 'absolute',
                                                    top: '2px',
                                                    left: task.enabled ? '18px' : '2px',
                                                    transition: 'left 0.2s',
                                                }} />
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            </main>
        </div>
    );
}
