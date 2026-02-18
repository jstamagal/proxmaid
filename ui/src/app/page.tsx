'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import ArrayStatusCard from '@/components/ArrayStatusCard';
import DiskListCard from '@/components/DiskListCard';
import StatsBar from '@/components/StatsBar';
import { api, type ArrayStatus, type CachePool, type MoverStatus, type NotifyEvent } from '@/lib/api';

function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export default function Dashboard() {
    const [arrayStatus, setArrayStatus] = useState<ArrayStatus | null>(null);
    const [pools, setPools] = useState<CachePool[]>([]);
    const [mover, setMover] = useState<MoverStatus | null>(null);
    const [events, setEvents] = useState<NotifyEvent[]>([]);

    useEffect(() => {
        const fetchAll = async () => {
            try {
                const [arrRes, poolRes, moverRes, evtRes] = await Promise.all([
                    api.array.status(),
                    api.cache.pools(),
                    api.cache.mover(),
                    api.notifications.history(),
                ]);
                if (arrRes.ok && arrRes.data) setArrayStatus(arrRes.data);
                if (poolRes.ok && poolRes.data) setPools(poolRes.data);
                if (moverRes.ok && moverRes.data) setMover(moverRes.data);
                if (evtRes.ok && evtRes.data) setEvents(evtRes.data.slice(0, 10));
            } catch { }
        };
        fetchAll();
        const interval = setInterval(fetchAll, 5000);
        return () => clearInterval(interval);
    }, []);

    return (
        <div style={{ display: 'flex', minHeight: '100vh' }}>
            <Sidebar />

            <main style={{
                marginLeft: '240px',
                flex: 1,
                padding: '32px',
                maxWidth: '1200px',
            }}>
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
                        Dashboard
                    </h1>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                        Proxmaid Storage Management
                    </p>
                </div>

                {/* Stats */}
                <div style={{ marginBottom: '24px' }}>
                    <StatsBar
                        arrayState={arrayStatus?.state || '—'}
                        numDisks={arrayStatus?.num_disks || 0}
                        numInvalid={arrayStatus?.num_invalid || 0}
                        synced={arrayStatus?.synced || false}
                    />
                </div>

                {/* Two-column dashboard grid */}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px', marginBottom: '20px' }}>

                    {/* Cache Usage Widget */}
                    <div className="glass-card" style={{ padding: '20px' }}>
                        <h3 style={{ fontSize: '14px', fontWeight: 700, marginBottom: '14px', color: 'var(--color-text-secondary)' }}>
                            Cache Pools
                        </h3>
                        {pools.length === 0 ? (
                            <div style={{ fontSize: '13px', color: 'var(--color-text-muted)' }}>No pools configured</div>
                        ) : (
                            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                                {pools.map(pool => (
                                    <div key={pool.name}>
                                        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '6px' }}>
                                            <span style={{ fontWeight: 600 }}>{pool.name}</span>
                                            <span style={{ color: 'var(--color-text-muted)' }}>
                                                {formatBytes(pool.used_bytes)} / {formatBytes(pool.total_bytes)}
                                            </span>
                                        </div>
                                        <div className="progress-bar">
                                            <div
                                                className="progress-bar-fill"
                                                style={{
                                                    width: `${pool.used_pct}%`,
                                                    background: pool.used_pct > 85
                                                        ? 'linear-gradient(90deg, #f59e0b, #ef4444)'
                                                        : 'linear-gradient(90deg, #3b82f6, #06b6d4)',
                                                }}
                                            />
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>

                    {/* Mover Status Widget */}
                    <div className="glass-card" style={{ padding: '20px' }}>
                        <h3 style={{ fontSize: '14px', fontWeight: 700, marginBottom: '14px', color: 'var(--color-text-secondary)' }}>
                            Mover
                        </h3>
                        {mover ? (
                            <div style={{ fontSize: '13px' }}>
                                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
                                    <div>
                                        <div style={{ color: 'var(--color-text-muted)', fontSize: '11px', textTransform: 'uppercase', marginBottom: '2px' }}>Status</div>
                                        <div style={{
                                            fontWeight: 700,
                                            color: mover.running ? 'var(--color-accent)' : 'var(--color-text-secondary)',
                                        }}>
                                            {mover.running ? `Running (${mover.progress}%)` : 'Idle'}
                                        </div>
                                    </div>
                                    <div>
                                        <div style={{ color: 'var(--color-text-muted)', fontSize: '11px', textTransform: 'uppercase', marginBottom: '2px' }}>Schedule</div>
                                        <div style={{ fontWeight: 600 }}>{mover.config.schedule || 'Manual'}</div>
                                    </div>
                                    <div>
                                        <div style={{ color: 'var(--color-text-muted)', fontSize: '11px', textTransform: 'uppercase', marginBottom: '2px' }}>Last Run</div>
                                        <div style={{ fontWeight: 600 }}>
                                            {mover.last_run ? new Date(mover.last_run).toLocaleString() : 'Never'}
                                        </div>
                                    </div>
                                    <div>
                                        <div style={{ color: 'var(--color-text-muted)', fontSize: '11px', textTransform: 'uppercase', marginBottom: '2px' }}>Files Moved</div>
                                        <div style={{ fontWeight: 600 }}>{mover.files_moved}</div>
                                    </div>
                                </div>
                                {mover.running && (
                                    <div style={{ marginTop: '10px' }}>
                                        <div className="progress-bar">
                                            <div className="progress-bar-fill" style={{ width: `${mover.progress}%` }} />
                                        </div>
                                    </div>
                                )}
                            </div>
                        ) : (
                            <div style={{ fontSize: '13px', color: 'var(--color-text-muted)' }}>Loading...</div>
                        )}
                    </div>
                </div>

                {/* Array & Disk cards */}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: '20px', marginBottom: '20px' }}>
                    <ArrayStatusCard />
                    <DiskListCard />
                </div>

                {/* Recent Activity Feed */}
                {events.length > 0 && (
                    <div className="glass-card" style={{ padding: '20px' }}>
                        <h3 style={{ fontSize: '14px', fontWeight: 700, marginBottom: '14px', color: 'var(--color-text-secondary)' }}>
                            Recent Activity
                        </h3>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                            {events.map((evt, i) => {
                                const severityColor = evt.severity === 'critical' ? 'var(--color-danger)'
                                    : evt.severity === 'warning' ? 'var(--color-warning)'
                                    : 'var(--color-text-muted)';
                                return (
                                    <div key={i} style={{
                                        display: 'flex',
                                        alignItems: 'center',
                                        gap: '10px',
                                        padding: '8px 12px',
                                        background: 'var(--color-bg-secondary)',
                                        borderRadius: '8px',
                                        fontSize: '12px',
                                    }}>
                                        <div style={{
                                            width: '6px',
                                            height: '6px',
                                            borderRadius: '50%',
                                            background: severityColor,
                                            flexShrink: 0,
                                        }} />
                                        <span style={{ color: 'var(--color-text-muted)', minWidth: '80px' }}>
                                            {evt.type}
                                        </span>
                                        <span style={{ flex: 1 }}>{evt.message}</span>
                                        <span style={{ color: 'var(--color-text-muted)', fontSize: '11px' }}>
                                            {new Date(evt.timestamp).toLocaleTimeString()}
                                        </span>
                                    </div>
                                );
                            })}
                        </div>
                    </div>
                )}
            </main>
        </div>
    );
}
