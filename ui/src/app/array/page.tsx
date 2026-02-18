'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import { api, type ArrayStatus, type SystemDisk } from '@/lib/api';

type DragItem = SystemDisk | null;

export default function ArrayPage() {
    const [arrayStatus, setArrayStatus] = useState<ArrayStatus | null>(null);
    const [systemDisks, setSystemDisks] = useState<SystemDisk[]>([]);
    const [loading, setLoading] = useState(true);
    const [actionLoading, setActionLoading] = useState(false);
    const [dragItem, setDragItem] = useState<DragItem>(null);
    const [dragOverSlot, setDragOverSlot] = useState<number | null>(null);

    const fetchData = async () => {
        try {
            const [statusRes, disksRes] = await Promise.all([
                api.array.status(),
                api.disks.list(),
            ]);
            if (statusRes.ok && statusRes.data) setArrayStatus(statusRes.data);
            if (disksRes.ok && disksRes.data) setSystemDisks(disksRes.data);
        } catch { }
        setLoading(false);
    };

    useEffect(() => {
        fetchData();
        const interval = setInterval(fetchData, 5000);
        return () => clearInterval(interval);
    }, []);

    const handleStart = async () => {
        setActionLoading(true);
        await api.array.start();
        await fetchData();
        setActionLoading(false);
    };

    const handleStop = async () => {
        setActionLoading(true);
        await api.array.stop();
        await fetchData();
        setActionLoading(false);
    };

    const handleCheck = async () => {
        setActionLoading(true);
        await api.array.check();
        await fetchData();
        setActionLoading(false);
    };

    // Get disks that are already assigned to the array
    const assignedDevices = new Set(
        arrayStatus?.disks?.map(d => d.device_name).filter(Boolean) || []
    );

    // Unassigned disks (not in the array)
    const unassignedDisks = systemDisks.filter(
        d => !assignedDevices.has(d.name) && !assignedDevices.has(d.name + '1')
    );

    const stateColor = (state: string) => {
        switch (state) {
            case 'STARTED': return 'var(--color-success)';
            case 'STOPPED': return 'var(--color-text-muted)';
            case 'DEGRADED': return 'var(--color-warning)';
            case 'ERROR': return 'var(--color-danger)';
            default: return 'var(--color-info)';
        }
    };

    const roleLabel = (role: string) => {
        switch (role) {
            case 'parity': return 'Parity';
            case 'q-parity': return 'Q-Parity';
            default: return 'Data';
        }
    };

    const roleColor = (role: string) => {
        switch (role) {
            case 'parity': return 'var(--color-info)';
            case 'q-parity': return 'var(--color-info)';
            default: return 'var(--color-success)';
        }
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', minHeight: '100vh' }}>
                <Sidebar />
                <main style={{ marginLeft: '240px', flex: 1, padding: '32px' }}>
                    <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading array...</div>
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
                        Array Manager
                    </h1>
                    <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                        Configure disk slots, roles, and array operations
                    </p>
                </div>

                {/* Array Controls Card */}
                <div className="glass-card" style={{ padding: '24px', marginBottom: '20px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                            <div style={{
                                width: '12px',
                                height: '12px',
                                borderRadius: '50%',
                                background: stateColor(arrayStatus?.state || ''),
                                boxShadow: `0 0 10px ${stateColor(arrayStatus?.state || '')}`,
                                animation: arrayStatus?.state === 'STARTED' ? 'pulse-glow 2s ease-in-out infinite' : 'none',
                            }} />
                            <div>
                                <h3 style={{ fontSize: '18px', fontWeight: 700 }}>
                                    Array — <span style={{ color: stateColor(arrayStatus?.state || '') }}>
                                        {arrayStatus?.state || 'UNKNOWN'}
                                    </span>
                                </h3>
                                <p style={{ color: 'var(--color-text-muted)', fontSize: '13px' }}>
                                    {arrayStatus?.num_disks || 0} disks &bull; {arrayStatus?.num_invalid || 0} invalid &bull; Parity {arrayStatus?.synced ? 'Synced' : 'Not Synced'}
                                </p>
                            </div>
                        </div>
                        <div style={{ display: 'flex', gap: '8px' }}>
                            {arrayStatus?.state === 'STARTED' ? (
                                <>
                                    <button className="btn btn-ghost" onClick={handleCheck} disabled={actionLoading}>
                                        ⟲ Parity Check
                                    </button>
                                    <button className="btn btn-danger" onClick={handleStop} disabled={actionLoading}>
                                        ■ Stop Array
                                    </button>
                                </>
                            ) : (
                                <button className="btn btn-success" onClick={handleStart} disabled={actionLoading}>
                                    ▶ Start Array
                                </button>
                            )}
                        </div>
                    </div>

                    {/* Resync Progress */}
                    {arrayStatus?.resync_active && (
                        <div style={{ marginTop: '16px' }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
                                <span style={{ color: 'var(--color-text-secondary)' }}>Parity Check</span>
                                <span style={{ color: 'var(--color-info)', fontWeight: 600 }}>{arrayStatus.resync_pct.toFixed(1)}%</span>
                            </div>
                            <div className="progress-bar">
                                <div className="progress-bar-fill" style={{ width: `${arrayStatus.resync_pct}%` }} />
                            </div>
                        </div>
                    )}
                </div>

                {/* Two-column layout: Slot Grid + Unassigned Disks */}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 340px', gap: '20px' }}>
                    {/* Slot Grid */}
                    <div className="glass-card" style={{ padding: '24px' }}>
                        <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '16px' }}>
                            Disk Slots
                        </h3>
                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '10px' }}>
                            {arrayStatus?.disks && arrayStatus.disks.length > 0 ? (
                                arrayStatus.disks.map((disk) => (
                                    <div
                                        key={disk.slot}
                                        style={{
                                            padding: '16px',
                                            background: dragOverSlot === disk.slot
                                                ? 'rgba(59, 130, 246, 0.15)'
                                                : 'var(--color-bg-card)',
                                            border: `1px solid ${dragOverSlot === disk.slot
                                                ? 'var(--color-accent)'
                                                : disk.status === 'DISK_OK'
                                                    ? 'var(--color-border)'
                                                    : 'var(--color-danger)'}`,
                                            borderRadius: '12px',
                                            transition: 'all 0.2s ease',
                                            position: 'relative',
                                            overflow: 'hidden',
                                            cursor: 'default',
                                        }}
                                        onDragOver={(e) => {
                                            e.preventDefault();
                                            setDragOverSlot(disk.slot);
                                        }}
                                        onDragLeave={() => setDragOverSlot(null)}
                                        onDrop={() => {
                                            setDragOverSlot(null);
                                            if (dragItem) {
                                                // Would call assign API here
                                                console.log(`Assign ${dragItem.name} to slot ${disk.slot}`);
                                            }
                                            setDragItem(null);
                                        }}
                                    >
                                        {/* Top accent bar */}
                                        <div style={{
                                            position: 'absolute',
                                            top: 0,
                                            left: 0,
                                            right: 0,
                                            height: '3px',
                                            background: roleColor(disk.role),
                                            borderRadius: '3px 3px 0 0',
                                        }} />
                                        <div style={{
                                            fontSize: '11px',
                                            color: roleColor(disk.role),
                                            marginBottom: '6px',
                                            textTransform: 'uppercase',
                                            letterSpacing: '0.06em',
                                            fontWeight: 600,
                                        }}>
                                            {roleLabel(disk.role)} {disk.role === 'data' && `${disk.slot}`}
                                        </div>
                                        <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '2px' }}>
                                            {disk.device_name || '— Empty —'}
                                        </div>
                                        {disk.size_human && (
                                            <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', marginBottom: '4px' }}>
                                                {disk.size_human}
                                            </div>
                                        )}
                                        <div style={{
                                            fontSize: '11px',
                                            color: disk.status === 'DISK_OK' ? 'var(--color-success)' : 'var(--color-danger)',
                                            display: 'flex',
                                            alignItems: 'center',
                                            gap: '4px',
                                        }}>
                                            <span style={{
                                                width: '6px',
                                                height: '6px',
                                                borderRadius: '50%',
                                                background: disk.status === 'DISK_OK' ? 'var(--color-success)' : 'var(--color-danger)',
                                                display: 'inline-block',
                                            }} />
                                            {disk.status}
                                        </div>
                                    </div>
                                ))
                            ) : (
                                <div style={{
                                    gridColumn: '1 / -1',
                                    padding: '40px',
                                    textAlign: 'center',
                                    color: 'var(--color-text-muted)',
                                    fontSize: '14px',
                                }}>
                                    No disks configured. Drag disks from the panel to assign them.
                                </div>
                            )}
                        </div>
                    </div>

                    {/* Unassigned Disks Panel */}
                    <div className="glass-card" style={{ padding: '24px' }}>
                        <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '4px' }}>
                            Available Disks
                        </h3>
                        <p style={{ fontSize: '12px', color: 'var(--color-text-muted)', marginBottom: '16px' }}>
                            Drag to assign to a slot
                        </p>

                        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                            {unassignedDisks.length > 0 ? (
                                unassignedDisks.map((disk) => (
                                    <div
                                        key={disk.name}
                                        draggable
                                        onDragStart={() => setDragItem(disk)}
                                        onDragEnd={() => {
                                            setDragItem(null);
                                            setDragOverSlot(null);
                                        }}
                                        style={{
                                            padding: '12px 14px',
                                            background: 'var(--color-bg-secondary)',
                                            borderRadius: '10px',
                                            border: '1px solid var(--color-border)',
                                            cursor: 'grab',
                                            transition: 'all 0.2s ease',
                                            fontSize: '13px',
                                        }}
                                        onMouseOver={(e) => {
                                            e.currentTarget.style.borderColor = 'var(--color-border-accent)';
                                            e.currentTarget.style.transform = 'translateX(4px)';
                                        }}
                                        onMouseOut={(e) => {
                                            e.currentTarget.style.borderColor = 'var(--color-border)';
                                            e.currentTarget.style.transform = 'translateX(0)';
                                        }}
                                    >
                                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                                            <span style={{ fontWeight: 600, color: 'var(--color-accent)' }}>
                                                {disk.path}
                                            </span>
                                            <span style={{
                                                padding: '2px 6px',
                                                borderRadius: '4px',
                                                fontSize: '10px',
                                                fontWeight: 600,
                                                background: disk.rotational
                                                    ? 'rgba(245, 158, 11, 0.15)'
                                                    : disk.path.includes('nvme')
                                                        ? 'rgba(139, 92, 246, 0.15)'
                                                        : 'rgba(16, 185, 129, 0.15)',
                                                color: disk.rotational
                                                    ? 'var(--color-warning)'
                                                    : disk.path.includes('nvme')
                                                        ? '#a78bfa'
                                                        : 'var(--color-success)',
                                            }}>
                                                {disk.rotational ? 'HDD' : disk.path.includes('nvme') ? 'NVMe' : 'SSD'}
                                            </span>
                                        </div>
                                        <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>
                                            {disk.model || 'Unknown'} &bull; {disk.size_human}
                                        </div>
                                        <div style={{ fontSize: '11px', color: 'var(--color-text-muted)' }}>
                                            {disk.serial || '—'}
                                        </div>
                                    </div>
                                ))
                            ) : (
                                <div style={{
                                    padding: '24px',
                                    textAlign: 'center',
                                    color: 'var(--color-text-muted)',
                                    fontSize: '13px',
                                }}>
                                    All disks assigned
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </main>
        </div>
    );
}
