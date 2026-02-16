'use client';

import { useState, useEffect } from 'react';
import { api, type SystemDisk } from '@/lib/api';

export default function DiskListCard() {
    const [disks, setDisks] = useState<SystemDisk[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const fetchDisks = async () => {
            try {
                const res = await api.disks.list();
                if (res.ok && res.data) {
                    setDisks(res.data);
                    setError(null);
                } else {
                    setError(res.error || 'Failed to fetch disks');
                }
            } catch (e) {
                setError('API unreachable');
            } finally {
                setLoading(false);
            }
        };
        fetchDisks();
    }, []);

    if (loading) {
        return (
            <div className="glass-card" style={{ padding: '24px' }}>
                <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Discovering disks...</div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="glass-card" style={{ padding: '24px', borderColor: 'var(--color-warning)' }}>
                <h3 style={{ fontSize: '16px', fontWeight: 600, marginBottom: '4px' }}>System Disks</h3>
                <p style={{ color: 'var(--color-warning)', fontSize: '13px' }}>{error}</p>
            </div>
        );
    }

    return (
        <div className="glass-card" style={{ padding: '24px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                <h3 style={{ fontSize: '18px', fontWeight: 700 }}>System Disks</h3>
                <span style={{ fontSize: '12px', color: 'var(--color-text-muted)' }}>
                    {disks.length} devices
                </span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {disks.map((disk) => (
                    <div
                        key={disk.name}
                        style={{
                            display: 'grid',
                            gridTemplateColumns: '80px 1fr 100px 80px',
                            alignItems: 'center',
                            padding: '12px 16px',
                            background: 'var(--color-bg-secondary)',
                            borderRadius: '10px',
                            border: '1px solid var(--color-border)',
                            fontSize: '13px',
                            transition: 'all 0.2s ease',
                            cursor: 'pointer',
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
                        <div style={{ fontWeight: 600, color: 'var(--color-accent)' }}>
                            {disk.path}
                        </div>
                        <div>
                            <div style={{ fontWeight: 500 }}>{disk.model || 'Unknown'}</div>
                            <div style={{ fontSize: '11px', color: 'var(--color-text-muted)' }}>{disk.serial || '-'}</div>
                        </div>
                        <div style={{ textAlign: 'right', fontWeight: 600 }}>
                            {disk.size_human}
                        </div>
                        <div style={{ textAlign: 'right' }}>
                            <span style={{
                                display: 'inline-block',
                                padding: '2px 8px',
                                borderRadius: '4px',
                                fontSize: '11px',
                                fontWeight: 600,
                                background: disk.rotational
                                    ? 'rgba(245, 158, 11, 0.15)'
                                    : 'rgba(16, 185, 129, 0.15)',
                                color: disk.rotational
                                    ? 'var(--color-warning)'
                                    : 'var(--color-success)',
                            }}>
                                {disk.rotational ? 'HDD' : 'SSD'}
                            </span>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}
