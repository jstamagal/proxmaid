'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import { api, type SystemDisk, type SmartHealth } from '@/lib/api';

type FilterType = 'all' | 'hdd' | 'ssd' | 'nvme' | 'unassigned';

export default function DisksPage() {
    const [disks, setDisks] = useState<SystemDisk[]>([]);
    const [healthCache, setHealthCache] = useState<Record<string, SmartHealth>>({});
    const [loading, setLoading] = useState(true);
    const [filter, setFilter] = useState<FilterType>('all');
    const [selectedDisk, setSelectedDisk] = useState<SystemDisk | null>(null);
    const [selectedHealth, setSelectedHealth] = useState<SmartHealth | null>(null);

    const fetchData = async () => {
        try {
            const [disksRes, healthRes] = await Promise.all([
                api.disks.list(),
                api.disks.health(),
            ]);
            if (disksRes.ok && disksRes.data) setDisks(disksRes.data);
            if (healthRes.ok && healthRes.data) setHealthCache(healthRes.data);
        } catch { }
        setLoading(false);
    };

    useEffect(() => {
        fetchData();
        const interval = setInterval(fetchData, 10000);
        return () => clearInterval(interval);
    }, []);

    const diskType = (d: SystemDisk): string => {
        if (d.path.includes('nvme')) return 'NVMe';
        if (!d.rotational) return 'SSD';
        return 'HDD';
    };

    const typeColor = (type: string) => {
        switch (type) {
            case 'NVMe': return { bg: 'rgba(139, 92, 246, 0.15)', fg: '#a78bfa' };
            case 'SSD': return { bg: 'rgba(16, 185, 129, 0.15)', fg: 'var(--color-success)' };
            default: return { bg: 'rgba(245, 158, 11, 0.15)', fg: 'var(--color-warning)' };
        }
    };

    const filteredDisks = disks.filter(d => {
        if (filter === 'all') return true;
        if (filter === 'hdd') return d.rotational;
        if (filter === 'ssd') return !d.rotational && !d.path.includes('nvme');
        if (filter === 'nvme') return d.path.includes('nvme');
        if (filter === 'unassigned') return !d.mountpoint;
        return true;
    });

    const handleIdentify = async (device: string) => {
        await api.disks.identify(device);
    };

    const handleSpindown = async (device: string) => {
        await api.disks.spindown(device);
    };

    const openDetail = async (disk: SystemDisk) => {
        setSelectedDisk(disk);
        const res = await api.disks.smart(disk.path);
        if (res.ok && res.data) setSelectedHealth(res.data);
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', minHeight: '100vh' }}>
                <Sidebar />
                <main style={{ marginLeft: '240px', flex: 1, padding: '32px' }}>
                    <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading disks...</div>
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
                        Disk Manager
                    </h1>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                        {disks.length} disks detected &bull; Monitor health, temperature, and manage storage
                    </p>
                </div>

                {/* Filters */}
                <div style={{ display: 'flex', gap: '8px', marginBottom: '20px' }}>
                    {(['all', 'hdd', 'ssd', 'nvme', 'unassigned'] as FilterType[]).map(f => (
                        <button
                            key={f}
                            onClick={() => setFilter(f)}
                            className={filter === f ? 'btn btn-ghost' : 'btn'}
                            style={{
                                padding: '6px 14px',
                                fontSize: '12px',
                                textTransform: 'uppercase',
                                letterSpacing: '0.05em',
                                background: filter === f ? 'rgba(59, 130, 246, 0.15)' : 'transparent',
                                color: filter === f ? 'var(--color-accent)' : 'var(--color-text-secondary)',
                                border: `1px solid ${filter === f ? 'var(--color-accent)' : 'var(--color-border)'}`,
                                borderRadius: '8px',
                                cursor: 'pointer',
                            }}
                        >
                            {f}
                        </button>
                    ))}
                </div>

                {/* Disk Table */}
                <div className="glass-card" style={{ padding: '0', overflow: 'hidden' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '13px' }}>
                        <thead>
                            <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
                                {['Device', 'Model', 'Serial', 'Size', 'Temp', 'Health', 'Mount', 'Type', 'Actions'].map(h => (
                                    <th key={h} style={{
                                        padding: '14px 16px',
                                        textAlign: 'left',
                                        fontSize: '11px',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.06em',
                                        color: 'var(--color-text-muted)',
                                        fontWeight: 600,
                                    }}>
                                        {h}
                                    </th>
                                ))}
                            </tr>
                        </thead>
                        <tbody>
                            {filteredDisks.map(disk => {
                                const health = healthCache[disk.path];
                                const type = diskType(disk);
                                const tc = typeColor(type);

                                return (
                                    <tr
                                        key={disk.name}
                                        onClick={() => openDetail(disk)}
                                        style={{
                                            borderBottom: '1px solid var(--color-border)',
                                            cursor: 'pointer',
                                            transition: 'background 0.15s ease',
                                        }}
                                        onMouseOver={(e) => e.currentTarget.style.background = 'rgba(255,255,255,0.02)'}
                                        onMouseOut={(e) => e.currentTarget.style.background = 'transparent'}
                                    >
                                        <td style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--color-accent)' }}>
                                            {disk.path}
                                        </td>
                                        <td style={{ padding: '12px 16px', color: 'var(--color-text-secondary)' }}>
                                            {disk.model || '—'}
                                        </td>
                                        <td style={{ padding: '12px 16px', color: 'var(--color-text-muted)', fontFamily: 'monospace', fontSize: '11px' }}>
                                            {disk.serial || '—'}
                                        </td>
                                        <td style={{ padding: '12px 16px' }}>
                                            {disk.size_human}
                                        </td>
                                        <td style={{ padding: '12px 16px' }}>
                                            {health ? (
                                                <span style={{
                                                    color: health.temperature > 50 ? 'var(--color-danger)' :
                                                           health.temperature > 40 ? 'var(--color-warning)' :
                                                           'var(--color-success)',
                                                }}>
                                                    {health.temperature}C
                                                </span>
                                            ) : '—'}
                                        </td>
                                        <td style={{ padding: '12px 16px' }}>
                                            {health ? (
                                                <span style={{
                                                    display: 'inline-flex',
                                                    alignItems: 'center',
                                                    gap: '4px',
                                                    color: health.healthy ? 'var(--color-success)' : 'var(--color-danger)',
                                                    fontSize: '12px',
                                                }}>
                                                    <span style={{
                                                        width: '6px',
                                                        height: '6px',
                                                        borderRadius: '50%',
                                                        background: health.healthy ? 'var(--color-success)' : 'var(--color-danger)',
                                                    }} />
                                                    {health.healthy ? 'OK' : 'FAIL'}
                                                </span>
                                            ) : '—'}
                                        </td>
                                        <td style={{ padding: '12px 16px', color: 'var(--color-text-muted)', fontSize: '12px' }}>
                                            {disk.mountpoint || '—'}
                                        </td>
                                        <td style={{ padding: '12px 16px' }}>
                                            <span style={{
                                                padding: '2px 8px',
                                                borderRadius: '4px',
                                                fontSize: '10px',
                                                fontWeight: 600,
                                                background: tc.bg,
                                                color: tc.fg,
                                            }}>
                                                {type}
                                            </span>
                                        </td>
                                        <td style={{ padding: '12px 16px' }}>
                                            <div style={{ display: 'flex', gap: '6px' }} onClick={e => e.stopPropagation()}>
                                                <button
                                                    onClick={() => handleIdentify(disk.path)}
                                                    title="Blink LED"
                                                    style={{
                                                        padding: '4px 8px',
                                                        fontSize: '11px',
                                                        background: 'rgba(59, 130, 246, 0.1)',
                                                        color: 'var(--color-accent)',
                                                        border: '1px solid rgba(59, 130, 246, 0.2)',
                                                        borderRadius: '6px',
                                                        cursor: 'pointer',
                                                    }}
                                                >
                                                    Locate
                                                </button>
                                                {disk.rotational && (
                                                    <button
                                                        onClick={() => handleSpindown(disk.path)}
                                                        title="Spin down"
                                                        style={{
                                                            padding: '4px 8px',
                                                            fontSize: '11px',
                                                            background: 'rgba(245, 158, 11, 0.1)',
                                                            color: 'var(--color-warning)',
                                                            border: '1px solid rgba(245, 158, 11, 0.2)',
                                                            borderRadius: '6px',
                                                            cursor: 'pointer',
                                                        }}
                                                    >
                                                        Spindown
                                                    </button>
                                                )}
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>

                    {filteredDisks.length === 0 && (
                        <div style={{ padding: '40px', textAlign: 'center', color: 'var(--color-text-muted)' }}>
                            No disks match the current filter.
                        </div>
                    )}
                </div>

                {/* SMART Detail Drawer */}
                {selectedDisk && (
                    <div
                        style={{
                            position: 'fixed',
                            top: 0,
                            right: 0,
                            bottom: 0,
                            width: '420px',
                            background: 'var(--color-bg-primary)',
                            borderLeft: '1px solid var(--color-border)',
                            padding: '32px 24px',
                            zIndex: 100,
                            overflowY: 'auto',
                            boxShadow: '-10px 0 40px rgba(0,0,0,0.5)',
                        }}
                    >
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px' }}>
                            <h2 style={{ fontSize: '20px', fontWeight: 700 }}>{selectedDisk.path}</h2>
                            <button
                                onClick={() => { setSelectedDisk(null); setSelectedHealth(null); }}
                                style={{
                                    background: 'none',
                                    border: 'none',
                                    color: 'var(--color-text-muted)',
                                    fontSize: '20px',
                                    cursor: 'pointer',
                                }}
                            >
                                x
                            </button>
                        </div>

                        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                            <div className="glass-card" style={{ padding: '16px' }}>
                                <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '8px', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Device Info</div>
                                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px', fontSize: '13px' }}>
                                    <div><span style={{ color: 'var(--color-text-muted)' }}>Model:</span> {selectedDisk.model || '—'}</div>
                                    <div><span style={{ color: 'var(--color-text-muted)' }}>Serial:</span> {selectedDisk.serial || '—'}</div>
                                    <div><span style={{ color: 'var(--color-text-muted)' }}>Size:</span> {selectedDisk.size_human}</div>
                                    <div><span style={{ color: 'var(--color-text-muted)' }}>Type:</span> {diskType(selectedDisk)}</div>
                                    <div><span style={{ color: 'var(--color-text-muted)' }}>FS:</span> {selectedDisk.fstype || 'none'}</div>
                                    <div><span style={{ color: 'var(--color-text-muted)' }}>Mount:</span> {selectedDisk.mountpoint || 'none'}</div>
                                </div>
                            </div>

                            {selectedHealth && (
                                <div className="glass-card" style={{ padding: '16px' }}>
                                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', marginBottom: '8px', textTransform: 'uppercase', letterSpacing: '0.06em' }}>SMART Health</div>
                                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px', fontSize: '13px' }}>
                                        <div>
                                            <span style={{ color: 'var(--color-text-muted)' }}>Status: </span>
                                            <span style={{ color: selectedHealth.healthy ? 'var(--color-success)' : 'var(--color-danger)', fontWeight: 600 }}>
                                                {selectedHealth.healthy ? 'PASSED' : 'FAILED'}
                                            </span>
                                        </div>
                                        <div>
                                            <span style={{ color: 'var(--color-text-muted)' }}>Temp: </span>
                                            <span style={{
                                                color: selectedHealth.temperature > 50 ? 'var(--color-danger)' :
                                                       selectedHealth.temperature > 40 ? 'var(--color-warning)' :
                                                       'var(--color-success)',
                                                fontWeight: 600,
                                            }}>
                                                {selectedHealth.temperature}C
                                            </span>
                                        </div>
                                        <div>
                                            <span style={{ color: 'var(--color-text-muted)' }}>Power-on:</span> {selectedHealth.power_on_hours.toLocaleString()}h
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                )}

                {/* Overlay when drawer open */}
                {selectedDisk && (
                    <div
                        onClick={() => { setSelectedDisk(null); setSelectedHealth(null); }}
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
                )}
            </main>
        </div>
    );
}
