'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import { api, type Container } from '@/lib/api';

export default function AppsPage() {
    const [containers, setContainers] = useState<Container[]>([]);
    const [loading, setLoading] = useState(true);
    const [actionLoading, setActionLoading] = useState<string | null>(null);
    const [selectedApp, setSelectedApp] = useState<Container | null>(null);
    const [appLogs, setAppLogs] = useState<string>('');

    const fetchData = async () => {
        try {
            const res = await api.apps.list();
            if (res.ok && res.data) setContainers(res.data);
        } catch { }
        setLoading(false);
    };

    useEffect(() => {
        fetchData();
        const interval = setInterval(fetchData, 5000);
        return () => clearInterval(interval);
    }, []);

    const handleStart = async (id: string) => {
        setActionLoading(id);
        await api.apps.start(id);
        await fetchData();
        setActionLoading(null);
    };

    const handleStop = async (id: string) => {
        setActionLoading(id);
        await api.apps.stop(id);
        await fetchData();
        setActionLoading(null);
    };

    const handleRemove = async (id: string) => {
        setActionLoading(id);
        await api.apps.remove(id);
        await fetchData();
        setActionLoading(null);
    };

    const openLogs = async (container: Container) => {
        setSelectedApp(container);
        const res = await api.apps.logs(container.id, 200);
        if (res.ok && res.data) setAppLogs(res.data);
    };

    const stateColor = (state: string) => {
        switch (state) {
            case 'running': return 'var(--color-success)';
            case 'exited': return 'var(--color-text-muted)';
            case 'paused': return 'var(--color-warning)';
            default: return 'var(--color-text-muted)';
        }
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', minHeight: '100vh' }}>
                <Sidebar />
                <main style={{ marginLeft: '240px', flex: 1, padding: '32px' }}>
                    <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading apps...</div>
                </main>
            </div>
        );
    }

    const running = containers.filter(c => c.state === 'running').length;
    const stopped = containers.filter(c => c.state !== 'running').length;

    return (
        <div style={{ display: 'flex', minHeight: '100vh' }}>
            <Sidebar />

            <main style={{ marginLeft: '240px', flex: 1, padding: '32px', maxWidth: '1200px' }}>
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
                        Apps
                    </h1>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                        {containers.length} containers &bull; {running} running &bull; {stopped} stopped
                    </p>
                </div>

                {/* Container Cards */}
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: '16px' }}>
                    {containers.map(container => (
                        <div key={container.id} className="glass-card" style={{ padding: '20px' }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                                    <div style={{
                                        width: '10px',
                                        height: '10px',
                                        borderRadius: '50%',
                                        background: stateColor(container.state),
                                        boxShadow: container.state === 'running' ? `0 0 8px ${stateColor(container.state)}` : 'none',
                                    }} />
                                    <h3 style={{ fontSize: '16px', fontWeight: 700 }}>{container.name}</h3>
                                </div>
                                <span style={{
                                    fontSize: '11px',
                                    color: stateColor(container.state),
                                    textTransform: 'uppercase',
                                    fontWeight: 600,
                                }}>
                                    {container.state}
                                </span>
                            </div>

                            <div style={{ fontSize: '12px', color: 'var(--color-text-muted)', marginBottom: '8px', fontFamily: 'monospace' }}>
                                {container.image}
                            </div>

                            <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', marginBottom: '12px' }}>
                                {container.status}
                            </div>

                            {/* Ports */}
                            {container.ports && container.ports.length > 0 && (
                                <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap', marginBottom: '12px' }}>
                                    {container.ports.filter(p => p.host_port > 0).map((p, i) => (
                                        <span key={i} style={{
                                            padding: '2px 6px',
                                            borderRadius: '4px',
                                            fontSize: '10px',
                                            background: 'rgba(59, 130, 246, 0.1)',
                                            color: 'var(--color-accent)',
                                            border: '1px solid rgba(59, 130, 246, 0.2)',
                                            fontFamily: 'monospace',
                                        }}>
                                            {p.host_port}:{p.container_port}/{p.protocol}
                                        </span>
                                    ))}
                                </div>
                            )}

                            {/* Actions */}
                            <div style={{ display: 'flex', gap: '6px', marginTop: '8px' }}>
                                {container.state === 'running' ? (
                                    <button
                                        onClick={() => handleStop(container.id)}
                                        disabled={actionLoading === container.id}
                                        style={{
                                            padding: '5px 12px',
                                            fontSize: '11px',
                                            background: 'rgba(239, 68, 68, 0.1)',
                                            color: 'var(--color-danger)',
                                            border: '1px solid rgba(239, 68, 68, 0.2)',
                                            borderRadius: '6px',
                                            cursor: 'pointer',
                                        }}
                                    >
                                        Stop
                                    </button>
                                ) : (
                                    <>
                                        <button
                                            onClick={() => handleStart(container.id)}
                                            disabled={actionLoading === container.id}
                                            style={{
                                                padding: '5px 12px',
                                                fontSize: '11px',
                                                background: 'rgba(16, 185, 129, 0.1)',
                                                color: 'var(--color-success)',
                                                border: '1px solid rgba(16, 185, 129, 0.2)',
                                                borderRadius: '6px',
                                                cursor: 'pointer',
                                            }}
                                        >
                                            Start
                                        </button>
                                        <button
                                            onClick={() => handleRemove(container.id)}
                                            disabled={actionLoading === container.id}
                                            style={{
                                                padding: '5px 12px',
                                                fontSize: '11px',
                                                background: 'rgba(107, 114, 128, 0.1)',
                                                color: 'var(--color-text-muted)',
                                                border: '1px solid rgba(107, 114, 128, 0.2)',
                                                borderRadius: '6px',
                                                cursor: 'pointer',
                                            }}
                                        >
                                            Remove
                                        </button>
                                    </>
                                )}
                                <button
                                    onClick={() => openLogs(container)}
                                    style={{
                                        padding: '5px 12px',
                                        fontSize: '11px',
                                        background: 'rgba(139, 92, 246, 0.1)',
                                        color: '#a78bfa',
                                        border: '1px solid rgba(139, 92, 246, 0.2)',
                                        borderRadius: '6px',
                                        cursor: 'pointer',
                                        marginLeft: 'auto',
                                    }}
                                >
                                    Logs
                                </button>
                            </div>
                        </div>
                    ))}
                </div>

                {containers.length === 0 && (
                    <div className="glass-card" style={{ padding: '60px', textAlign: 'center' }}>
                        <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                            No Docker containers found. Install Docker to get started.
                        </p>
                    </div>
                )}

                {/* Logs Drawer */}
                {selectedApp && (
                    <>
                        <div
                            onClick={() => { setSelectedApp(null); setAppLogs(''); }}
                            style={{
                                position: 'fixed',
                                top: 0,
                                left: 0,
                                right: 0,
                                bottom: 0,
                                background: 'rgba(0,0,0,0.4)',
                                zIndex: 99,
                            }}
                        />
                        <div style={{
                            position: 'fixed',
                            top: 0,
                            right: 0,
                            bottom: 0,
                            width: '560px',
                            background: 'var(--color-bg-primary)',
                            borderLeft: '1px solid var(--color-border)',
                            padding: '24px',
                            zIndex: 100,
                            display: 'flex',
                            flexDirection: 'column',
                            boxShadow: '-10px 0 40px rgba(0,0,0,0.5)',
                        }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                                <h2 style={{ fontSize: '18px', fontWeight: 700 }}>
                                    {selectedApp.name} — Logs
                                </h2>
                                <button
                                    onClick={() => { setSelectedApp(null); setAppLogs(''); }}
                                    style={{ background: 'none', border: 'none', color: 'var(--color-text-muted)', fontSize: '20px', cursor: 'pointer' }}
                                >
                                    x
                                </button>
                            </div>
                            <pre style={{
                                flex: 1,
                                overflow: 'auto',
                                background: 'var(--color-bg-secondary)',
                                borderRadius: '8px',
                                padding: '16px',
                                fontSize: '11px',
                                lineHeight: 1.6,
                                fontFamily: 'monospace',
                                color: 'var(--color-text-secondary)',
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-all',
                            }}>
                                {appLogs || 'Loading logs...'}
                            </pre>
                        </div>
                    </>
                )}
            </main>
        </div>
    );
}
