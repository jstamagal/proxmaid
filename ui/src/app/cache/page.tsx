'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import { api, type CachePool, type MoverStatus } from '@/lib/api';

function humanBytes(bytes: number): string {
    if (bytes >= 1024 ** 4) return `${(bytes / 1024 ** 4).toFixed(1)} TB`;
    if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
    if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
    return `${bytes} B`;
}

function timeAgo(iso: string): string {
    if (!iso) return '—';
    const diff = Date.now() - new Date(iso).getTime();
    const hrs = Math.floor(diff / 3600000);
    const mins = Math.floor((diff % 3600000) / 60000);
    if (hrs > 24) return `${Math.floor(hrs / 24)}d ago`;
    if (hrs > 0) return `${hrs}h ${mins}m ago`;
    return `${mins}m ago`;
}

function timeUntil(iso: string): string {
    if (!iso) return '—';
    const diff = new Date(iso).getTime() - Date.now();
    if (diff < 0) return 'overdue';
    const hrs = Math.floor(diff / 3600000);
    const mins = Math.floor((diff % 3600000) / 60000);
    if (hrs > 24) return `in ${Math.floor(hrs / 24)}d`;
    if (hrs > 0) return `in ${hrs}h ${mins}m`;
    return `in ${mins}m`;
}

export default function CachePage() {
    const [pools, setPools] = useState<CachePool[]>([]);
    const [mover, setMover] = useState<MoverStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [moverRunning, setMoverRunning] = useState(false);

    const fetchData = async () => {
        try {
            const [poolsRes, moverRes] = await Promise.all([
                api.cache.pools(),
                api.cache.mover(),
            ]);
            if (poolsRes.ok && poolsRes.data) setPools(poolsRes.data);
            if (moverRes.ok && moverRes.data) {
                setMover(moverRes.data);
                setMoverRunning(moverRes.data.running);
            }
        } catch { }
        setLoading(false);
    };

    useEffect(() => {
        fetchData();
        const interval = setInterval(fetchData, 5000);
        return () => clearInterval(interval);
    }, []);

    const handleRunMover = async () => {
        setMoverRunning(true);
        await api.cache.runMover();
        setTimeout(fetchData, 1000);
    };

    const totalCacheSize = pools.reduce((s, p) => s + p.total_bytes, 0);
    const totalUsed = pools.reduce((s, p) => s + p.used_bytes, 0);
    const overallPct = totalCacheSize > 0 ? (totalUsed / totalCacheSize) * 100 : 0;

    if (loading) {
        return (
            <div style={{ display: 'flex', minHeight: '100vh' }}>
                <Sidebar />
                <main style={{ marginLeft: '240px', flex: 1, padding: '32px' }}>
                    <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading cache pools...</div>
                </main>
            </div>
        );
    }

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
                        Cache &amp; Tiering
                    </h1>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                        Manage cache pools and data mover
                    </p>
                </div>

                {/* Overview Stats */}
                <div style={{ display: 'flex', gap: '12px', marginBottom: '20px', flexWrap: 'wrap' }}>
                    <div className="glass-card" style={{ padding: '20px 24px', flex: 1, minWidth: '160px' }}>
                        <div style={{ fontSize: '12px', color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: '8px' }}>
                            Cache Pools
                        </div>
                        <div style={{ fontSize: '28px', fontWeight: 800, color: 'var(--color-accent)' }}>
                            {pools.length}
                        </div>
                    </div>
                    <div className="glass-card" style={{ padding: '20px 24px', flex: 1, minWidth: '160px' }}>
                        <div style={{ fontSize: '12px', color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: '8px' }}>
                            Total Cache
                        </div>
                        <div style={{ fontSize: '28px', fontWeight: 800, color: 'var(--color-text-primary)' }}>
                            {humanBytes(totalCacheSize)}
                        </div>
                        <div style={{ fontSize: '11px', color: 'var(--color-text-muted)' }}>
                            {humanBytes(totalUsed)} used ({overallPct.toFixed(0)}%)
                        </div>
                    </div>
                    <div className="glass-card" style={{ padding: '20px 24px', flex: 1, minWidth: '160px' }}>
                        <div style={{ fontSize: '12px', color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: '8px' }}>
                            Mover
                        </div>
                        <div style={{
                            fontSize: '28px',
                            fontWeight: 800,
                            color: moverRunning ? 'var(--color-info)' : 'var(--color-success)',
                        }}>
                            {moverRunning ? 'Running' : 'Idle'}
                        </div>
                        <div style={{ fontSize: '11px', color: 'var(--color-text-muted)' }}>
                            Last: {timeAgo(mover?.last_run || '')}
                        </div>
                    </div>
                </div>

                {/* Main content grid */}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 380px', gap: '20px' }}>
                    {/* Pool Cards */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                        {pools.length > 0 ? pools.map((pool) => (
                            <div key={pool.name} className="glass-card" style={{ padding: '24px' }}>
                                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '16px' }}>
                                    <div>
                                        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
                                            <h3 style={{ fontSize: '18px', fontWeight: 700 }}>{pool.name}</h3>
                                            <span style={{
                                                padding: '2px 8px',
                                                borderRadius: '4px',
                                                fontSize: '11px',
                                                fontWeight: 600,
                                                background: pool.status === 'active'
                                                    ? 'rgba(16, 185, 129, 0.15)'
                                                    : 'rgba(245, 158, 11, 0.15)',
                                                color: pool.status === 'active'
                                                    ? 'var(--color-success)'
                                                    : 'var(--color-warning)',
                                            }}>
                                                {pool.status}
                                            </span>
                                        </div>
                                        <p style={{ color: 'var(--color-text-muted)', fontSize: '12px' }}>
                                            {pool.mount_point} &bull; {pool.fs_type.toUpperCase()} &bull; {pool.devices.length} device{pool.devices.length !== 1 ? 's' : ''}
                                        </p>
                                    </div>
                                </div>

                                {/* Capacity Bar */}
                                <div style={{ marginBottom: '16px' }}>
                                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                                        <span style={{ color: 'var(--color-text-secondary)' }}>
                                            {humanBytes(pool.used_bytes)} / {humanBytes(pool.total_bytes)}
                                        </span>
                                        <span style={{
                                            fontWeight: 600,
                                            color: pool.used_pct > 90
                                                ? 'var(--color-danger)'
                                                : pool.used_pct > 70
                                                    ? 'var(--color-warning)'
                                                    : 'var(--color-success)',
                                        }}>
                                            {pool.used_pct.toFixed(0)}%
                                        </span>
                                    </div>
                                    <div style={{
                                        height: '8px',
                                        borderRadius: '4px',
                                        background: 'var(--color-bg-secondary)',
                                        overflow: 'hidden',
                                    }}>
                                        <div style={{
                                            height: '100%',
                                            width: `${pool.used_pct}%`,
                                            borderRadius: '4px',
                                            background: pool.used_pct > 90
                                                ? 'linear-gradient(90deg, #ef4444, #dc2626)'
                                                : pool.used_pct > 70
                                                    ? 'linear-gradient(90deg, #f59e0b, #d97706)'
                                                    : 'linear-gradient(90deg, #10b981, #06b6d4)',
                                            transition: 'width 0.5s ease',
                                        }} />
                                    </div>
                                </div>

                                {/* Devices */}
                                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                                    {pool.devices.map((dev) => (
                                        <div key={dev.path} style={{
                                            display: 'flex',
                                            justifyContent: 'space-between',
                                            alignItems: 'center',
                                            padding: '8px 12px',
                                            background: 'var(--color-bg-secondary)',
                                            borderRadius: '8px',
                                            fontSize: '12px',
                                        }}>
                                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                                <span style={{
                                                    width: '6px',
                                                    height: '6px',
                                                    borderRadius: '50%',
                                                    background: dev.path.includes('nvme') ? '#a78bfa' : 'var(--color-success)',
                                                    display: 'inline-block',
                                                }} />
                                                <span style={{ fontWeight: 600, color: 'var(--color-accent)' }}>
                                                    {dev.path}
                                                </span>
                                            </div>
                                            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                                                <span style={{ color: 'var(--color-text-secondary)' }}>
                                                    {dev.model}
                                                </span>
                                                <span style={{ fontWeight: 600 }}>
                                                    {dev.size_human}
                                                </span>
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )) : (
                            <div className="glass-card" style={{
                                padding: '48px',
                                textAlign: 'center',
                                color: 'var(--color-text-muted)',
                            }}>
                                <div style={{ fontSize: '32px', marginBottom: '12px' }}>⊕</div>
                                <div style={{ fontSize: '14px', marginBottom: '4px' }}>No cache pools configured</div>
                                <div style={{ fontSize: '12px' }}>Create a pool to start tiering data</div>
                            </div>
                        )}
                    </div>

                    {/* Mover Panel */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                        {/* Mover Status Card */}
                        <div className="glass-card" style={{ padding: '24px' }}>
                            <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '16px' }}>
                                Data Mover
                            </h3>

                            {/* Status indicator */}
                            <div style={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: '10px',
                                padding: '14px 16px',
                                background: moverRunning
                                    ? 'rgba(6, 182, 212, 0.08)'
                                    : 'var(--color-bg-secondary)',
                                borderRadius: '10px',
                                border: `1px solid ${moverRunning ? 'rgba(6, 182, 212, 0.3)' : 'var(--color-border)'}`,
                                marginBottom: '16px',
                            }}>
                                <div style={{
                                    width: '10px',
                                    height: '10px',
                                    borderRadius: '50%',
                                    background: moverRunning ? 'var(--color-info)' : 'var(--color-success)',
                                    boxShadow: moverRunning ? '0 0 8px var(--color-info)' : 'none',
                                    animation: moverRunning ? 'pulse-glow 2s ease-in-out infinite' : 'none',
                                }} />
                                <div>
                                    <div style={{ fontSize: '14px', fontWeight: 600 }}>
                                        {moverRunning ? 'Moving data...' : 'Idle'}
                                    </div>
                                    {moverRunning && mover && (
                                        <div style={{ fontSize: '11px', color: 'var(--color-text-muted)' }}>
                                            {mover.progress.toFixed(0)}% complete
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Progress bar when running */}
                            {moverRunning && mover && (
                                <div style={{ marginBottom: '16px' }}>
                                    <div className="progress-bar" style={{ height: '4px' }}>
                                        <div className="progress-bar-fill" style={{ width: `${mover.progress}%` }} />
                                    </div>
                                </div>
                            )}

                            {/* Stats grid */}
                            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '16px' }}>
                                <div style={{
                                    padding: '12px',
                                    background: 'var(--color-bg-secondary)',
                                    borderRadius: '8px',
                                }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                        Last Run
                                    </div>
                                    <div style={{ fontSize: '14px', fontWeight: 600 }}>
                                        {timeAgo(mover?.last_run || '')}
                                    </div>
                                </div>
                                <div style={{
                                    padding: '12px',
                                    background: 'var(--color-bg-secondary)',
                                    borderRadius: '8px',
                                }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                        Next Run
                                    </div>
                                    <div style={{ fontSize: '14px', fontWeight: 600 }}>
                                        {timeUntil(mover?.next_run || '')}
                                    </div>
                                </div>
                                <div style={{
                                    padding: '12px',
                                    background: 'var(--color-bg-secondary)',
                                    borderRadius: '8px',
                                }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                        Files Moved
                                    </div>
                                    <div style={{ fontSize: '14px', fontWeight: 600 }}>
                                        {mover?.files_moved || 0}
                                    </div>
                                </div>
                                <div style={{
                                    padding: '12px',
                                    background: 'var(--color-bg-secondary)',
                                    borderRadius: '8px',
                                }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                                        Data Moved
                                    </div>
                                    <div style={{ fontSize: '14px', fontWeight: 600 }}>
                                        {humanBytes(mover?.bytes_moved || 0)}
                                    </div>
                                </div>
                            </div>

                            {/* Move Now button */}
                            <button
                                className="btn btn-primary"
                                onClick={handleRunMover}
                                disabled={moverRunning}
                                style={{ width: '100%', justifyContent: 'center' }}
                            >
                                {moverRunning ? '⟲ Moving...' : '▶ Move Now'}
                            </button>
                        </div>

                        {/* Mover Config Card */}
                        <div className="glass-card" style={{ padding: '24px' }}>
                            <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '16px' }}>
                                Mover Schedule
                            </h3>
                            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                                <div>
                                    <label style={{ fontSize: '12px', color: 'var(--color-text-muted)', display: 'block', marginBottom: '6px' }}>
                                        Schedule (cron)
                                    </label>
                                    <div style={{
                                        padding: '10px 12px',
                                        background: 'var(--color-bg-secondary)',
                                        borderRadius: '8px',
                                        border: '1px solid var(--color-border)',
                                        fontSize: '13px',
                                        fontFamily: 'monospace',
                                        color: 'var(--color-text-primary)',
                                    }}>
                                        {mover?.config.schedule || '40 3 * * *'}
                                    </div>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginTop: '4px' }}>
                                        Daily at 3:40 AM
                                    </div>
                                </div>
                                <div>
                                    <label style={{ fontSize: '12px', color: 'var(--color-text-muted)', display: 'block', marginBottom: '6px' }}>
                                        Age Threshold
                                    </label>
                                    <div style={{
                                        padding: '10px 12px',
                                        background: 'var(--color-bg-secondary)',
                                        borderRadius: '8px',
                                        border: '1px solid var(--color-border)',
                                        fontSize: '13px',
                                        fontFamily: 'monospace',
                                        color: 'var(--color-text-primary)',
                                    }}>
                                        {mover?.config.age_threshold || '1d'}
                                    </div>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginTop: '4px' }}>
                                        Files older than this are moved to array
                                    </div>
                                </div>
                                <div style={{
                                    display: 'flex',
                                    justifyContent: 'space-between',
                                    alignItems: 'center',
                                    padding: '10px 12px',
                                    background: 'var(--color-bg-secondary)',
                                    borderRadius: '8px',
                                }}>
                                    <span style={{ fontSize: '13px' }}>Mover Enabled</span>
                                    <span style={{
                                        width: '36px',
                                        height: '20px',
                                        borderRadius: '10px',
                                        background: mover?.config.enabled
                                            ? 'var(--color-success)'
                                            : 'var(--color-text-muted)',
                                        display: 'flex',
                                        alignItems: 'center',
                                        padding: '2px',
                                        transition: 'background 0.2s ease',
                                    }}>
                                        <span style={{
                                            width: '16px',
                                            height: '16px',
                                            borderRadius: '50%',
                                            background: 'white',
                                            transition: 'transform 0.2s ease',
                                            transform: mover?.config.enabled ? 'translateX(16px)' : 'translateX(0)',
                                        }} />
                                    </span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    );
}
