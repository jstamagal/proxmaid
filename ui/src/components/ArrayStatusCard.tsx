'use client';

import { useState, useEffect } from 'react';
import { api, type ArrayStatus } from '@/lib/api';

export default function ArrayStatusCard() {
    const [status, setStatus] = useState<ArrayStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [actionLoading, setActionLoading] = useState(false);

    const fetchStatus = async () => {
        try {
            const res = await api.array.status();
            if (res.ok && res.data) {
                setStatus(res.data);
                setError(null);
            } else {
                setError(res.error || 'Failed to fetch status');
            }
        } catch (e) {
            setError('API unreachable');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchStatus();
        const interval = setInterval(fetchStatus, 5000);
        return () => clearInterval(interval);
    }, []);

    const handleStart = async () => {
        setActionLoading(true);
        await api.array.start();
        await fetchStatus();
        setActionLoading(false);
    };

    const handleStop = async () => {
        setActionLoading(true);
        await api.array.stop();
        await fetchStatus();
        setActionLoading(false);
    };

    const stateColor = (state: string) => {
        switch (state) {
            case 'STARTED': return 'var(--color-success)';
            case 'STOPPED': return 'var(--color-text-muted)';
            case 'DEGRADED': return 'var(--color-warning)';
            case 'ERROR': return 'var(--color-danger)';
            default: return 'var(--color-info)';
        }
    };

    const statusDotClass = (state: string) => {
        switch (state) {
            case 'STARTED': return 'ok';
            case 'STOPPED': return 'stopped';
            case 'DEGRADED': return 'warning';
            case 'ERROR': return 'error';
            default: return 'ok';
        }
    };

    if (loading) {
        return (
            <div className="glass-card" style={{ padding: '24px' }}>
                <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading array status...</div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="glass-card" style={{ padding: '24px', borderColor: 'var(--color-danger)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                        <h3 style={{ fontSize: '16px', fontWeight: 600, marginBottom: '4px' }}>Array Status</h3>
                        <p style={{ color: 'var(--color-danger)', fontSize: '13px' }}>{error}</p>
                    </div>
                    <button className="btn btn-ghost" onClick={fetchStatus}>Retry</button>
                </div>
            </div>
        );
    }

    return (
        <div className="glass-card" style={{ padding: '24px' }}>
            {/* Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '20px' }}>
                <div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
                        <h3 style={{ fontSize: '18px', fontWeight: 700 }}>Array</h3>
                        <span className={`status-dot ${statusDotClass(status?.state || '')}`}></span>
                        <span style={{
                            fontSize: '12px',
                            fontWeight: 600,
                            color: stateColor(status?.state || ''),
                            textTransform: 'uppercase',
                            letterSpacing: '0.05em',
                        }}>
                            {status?.state || 'UNKNOWN'}
                        </span>
                    </div>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '13px' }}>
                        {status?.num_disks || 0} disks • {status?.num_invalid || 0} invalid
                    </p>
                </div>

                <div style={{ display: 'flex', gap: '8px' }}>
                    {status?.state === 'STARTED' ? (
                        <>
                            <button className="btn btn-ghost" onClick={() => api.array.check()} disabled={actionLoading}>
                                ⟲ Check
                            </button>
                            <button className="btn btn-danger" onClick={handleStop} disabled={actionLoading}>
                                ■ Stop
                            </button>
                        </>
                    ) : (
                        <button className="btn btn-success" onClick={handleStart} disabled={actionLoading}>
                            ▶ Start
                        </button>
                    )}
                </div>
            </div>

            {/* Resync Progress */}
            {status?.resync_active && (
                <div style={{ marginBottom: '16px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                        <span style={{ color: 'var(--color-text-secondary)' }}>Parity Check</span>
                        <span style={{ color: 'var(--color-info)', fontWeight: 600 }}>{status.resync_pct.toFixed(1)}%</span>
                    </div>
                    <div className="progress-bar">
                        <div className="progress-bar-fill" style={{ width: `${status.resync_pct}%` }}></div>
                    </div>
                </div>
            )}

            {/* Disk Slots */}
            {status?.disks && status.disks.length > 0 && (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: '8px' }}>
                    {status.disks.map((disk) => (
                        <div
                            key={disk.slot}
                            className={`disk-slot ${disk.role}`}
                            style={{ padding: '12px' }}
                        >
                            <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                                {disk.role === 'parity' ? 'Parity' : `Disk ${disk.slot}`}
                            </div>
                            <div style={{ fontSize: '13px', fontWeight: 600 }}>
                                {disk.device_name || 'Empty'}
                            </div>
                            <div style={{ fontSize: '11px', color: disk.status === 'DISK_OK' ? 'var(--color-success)' : 'var(--color-danger)' }}>
                                {disk.status}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
