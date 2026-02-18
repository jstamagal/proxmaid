'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import { api, type Share } from '@/lib/api';

export default function SharesPage() {
    const [shares, setShares] = useState<Share[]>([]);
    const [users, setUsers] = useState<string[]>([]);
    const [loading, setLoading] = useState(true);
    const [showCreateDialog, setShowCreateDialog] = useState(false);
    const [newShare, setNewShare] = useState({ name: '', export_smb: true, export_nfs: false, cache_policy: 'yes', security_mode: 'public' });

    const fetchData = async () => {
        try {
            const [sharesRes, usersRes] = await Promise.all([
                api.shares.list(),
                api.users.list(),
            ]);
            if (sharesRes.ok && sharesRes.data) setShares(sharesRes.data);
            if (usersRes.ok && usersRes.data) setUsers(usersRes.data);
        } catch { }
        setLoading(false);
    };

    useEffect(() => {
        fetchData();
        const interval = setInterval(fetchData, 5000);
        return () => clearInterval(interval);
    }, []);

    const handleCreate = async () => {
        const res = await api.shares.create(newShare);
        if (res.ok) {
            setShowCreateDialog(false);
            setNewShare({ name: '', export_smb: true, export_nfs: false, cache_policy: 'yes', security_mode: 'public' });
            await fetchData();
        }
    };

    const handleDelete = async (name: string) => {
        await api.shares.delete(name);
        await fetchData();
    };

    const cachePolicyColor = (policy: string) => {
        switch (policy) {
            case 'yes': return { bg: 'rgba(16, 185, 129, 0.15)', fg: 'var(--color-success)' };
            case 'prefer': return { bg: 'rgba(59, 130, 246, 0.15)', fg: 'var(--color-accent)' };
            case 'only': return { bg: 'rgba(139, 92, 246, 0.15)', fg: '#a78bfa' };
            case 'no': return { bg: 'rgba(107, 114, 128, 0.15)', fg: 'var(--color-text-muted)' };
            default: return { bg: 'transparent', fg: 'var(--color-text-muted)' };
        }
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', minHeight: '100vh' }}>
                <Sidebar />
                <main style={{ marginLeft: '240px', flex: 1, padding: '32px' }}>
                    <div style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>Loading shares...</div>
                </main>
            </div>
        );
    }

    return (
        <div style={{ display: 'flex', minHeight: '100vh' }}>
            <Sidebar />

            <main style={{ marginLeft: '240px', flex: 1, padding: '32px', maxWidth: '1200px' }}>
                {/* Header */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '28px' }}>
                    <div>
                        <h1 style={{
                            fontSize: '28px',
                            fontWeight: 800,
                            background: 'linear-gradient(135deg, #e2e8f0, #94a3b8)',
                            WebkitBackgroundClip: 'text',
                            WebkitTextFillColor: 'transparent',
                            marginBottom: '4px',
                        }}>
                            Shares
                        </h1>
                        <p style={{ color: 'var(--color-text-muted)', fontSize: '14px' }}>
                            {shares.length} shares &bull; {users.length} users
                        </p>
                    </div>
                    <button
                        onClick={() => setShowCreateDialog(true)}
                        className="btn btn-success"
                        style={{ padding: '8px 20px', fontSize: '13px' }}
                    >
                        + New Share
                    </button>
                </div>

                {/* Share Cards */}
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: '16px' }}>
                    {shares.map(share => {
                        const cp = cachePolicyColor(share.cache_policy);
                        return (
                            <div key={share.name} className="glass-card" style={{ padding: '20px' }}>
                                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
                                    <h3 style={{ fontSize: '18px', fontWeight: 700 }}>{share.name}</h3>
                                    <button
                                        onClick={() => handleDelete(share.name)}
                                        style={{
                                            background: 'rgba(239, 68, 68, 0.1)',
                                            color: 'var(--color-danger)',
                                            border: '1px solid rgba(239, 68, 68, 0.2)',
                                            borderRadius: '6px',
                                            padding: '4px 10px',
                                            fontSize: '11px',
                                            cursor: 'pointer',
                                        }}
                                    >
                                        Delete
                                    </button>
                                </div>

                                <div style={{ fontSize: '12px', color: 'var(--color-text-muted)', marginBottom: '12px', fontFamily: 'monospace' }}>
                                    {share.path}
                                </div>

                                {/* Badges */}
                                <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginBottom: '12px' }}>
                                    {share.export_smb && (
                                        <span style={{
                                            padding: '2px 8px',
                                            borderRadius: '4px',
                                            fontSize: '10px',
                                            fontWeight: 600,
                                            background: 'rgba(59, 130, 246, 0.15)',
                                            color: 'var(--color-accent)',
                                        }}>
                                            SMB
                                        </span>
                                    )}
                                    {share.export_nfs && (
                                        <span style={{
                                            padding: '2px 8px',
                                            borderRadius: '4px',
                                            fontSize: '10px',
                                            fontWeight: 600,
                                            background: 'rgba(16, 185, 129, 0.15)',
                                            color: 'var(--color-success)',
                                        }}>
                                            NFS
                                        </span>
                                    )}
                                    <span style={{
                                        padding: '2px 8px',
                                        borderRadius: '4px',
                                        fontSize: '10px',
                                        fontWeight: 600,
                                        background: cp.bg,
                                        color: cp.fg,
                                    }}>
                                        Cache: {share.cache_policy}
                                    </span>
                                    <span style={{
                                        padding: '2px 8px',
                                        borderRadius: '4px',
                                        fontSize: '10px',
                                        fontWeight: 600,
                                        background: 'rgba(107, 114, 128, 0.15)',
                                        color: 'var(--color-text-secondary)',
                                    }}>
                                        {share.security_mode}
                                    </span>
                                    {share.recycle_bin && (
                                        <span style={{
                                            padding: '2px 8px',
                                            borderRadius: '4px',
                                            fontSize: '10px',
                                            fontWeight: 600,
                                            background: 'rgba(245, 158, 11, 0.15)',
                                            color: 'var(--color-warning)',
                                        }}>
                                            Recycle Bin
                                        </span>
                                    )}
                                </div>

                                <div style={{ fontSize: '12px', color: 'var(--color-text-muted)' }}>
                                    Allocation: {share.alloc_method?.toUpperCase() || 'MFS'}
                                </div>
                            </div>
                        );
                    })}
                </div>

                {shares.length === 0 && (
                    <div className="glass-card" style={{ padding: '60px', textAlign: 'center' }}>
                        <p style={{ color: 'var(--color-text-muted)', fontSize: '14px', marginBottom: '16px' }}>
                            No shares configured yet.
                        </p>
                        <button onClick={() => setShowCreateDialog(true)} className="btn btn-success" style={{ padding: '8px 20px' }}>
                            Create First Share
                        </button>
                    </div>
                )}

                {/* Create Share Dialog */}
                {showCreateDialog && (
                    <>
                        <div
                            onClick={() => setShowCreateDialog(false)}
                            style={{
                                position: 'fixed',
                                top: 0,
                                left: 0,
                                right: 0,
                                bottom: 0,
                                background: 'rgba(0,0,0,0.5)',
                                zIndex: 99,
                            }}
                        />
                        <div style={{
                            position: 'fixed',
                            top: '50%',
                            left: '50%',
                            transform: 'translate(-50%, -50%)',
                            width: '440px',
                            background: 'var(--color-bg-primary)',
                            border: '1px solid var(--color-border)',
                            borderRadius: '16px',
                            padding: '28px',
                            zIndex: 100,
                            boxShadow: '0 20px 60px rgba(0,0,0,0.5)',
                        }}>
                            <h2 style={{ fontSize: '20px', fontWeight: 700, marginBottom: '20px' }}>New Share</h2>

                            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
                                <div>
                                    <label style={{ fontSize: '12px', color: 'var(--color-text-muted)', display: 'block', marginBottom: '6px' }}>Name</label>
                                    <input
                                        type="text"
                                        value={newShare.name}
                                        onChange={e => setNewShare({ ...newShare, name: e.target.value })}
                                        placeholder="e.g. media"
                                        style={{
                                            width: '100%',
                                            padding: '8px 12px',
                                            background: 'var(--color-bg-secondary)',
                                            border: '1px solid var(--color-border)',
                                            borderRadius: '8px',
                                            color: 'var(--color-text-primary)',
                                            fontSize: '13px',
                                        }}
                                    />
                                </div>

                                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                                    <div>
                                        <label style={{ fontSize: '12px', color: 'var(--color-text-muted)', display: 'block', marginBottom: '6px' }}>Cache Policy</label>
                                        <select
                                            value={newShare.cache_policy}
                                            onChange={e => setNewShare({ ...newShare, cache_policy: e.target.value })}
                                            style={{
                                                width: '100%',
                                                padding: '8px 12px',
                                                background: 'var(--color-bg-secondary)',
                                                border: '1px solid var(--color-border)',
                                                borderRadius: '8px',
                                                color: 'var(--color-text-primary)',
                                                fontSize: '13px',
                                            }}
                                        >
                                            <option value="yes">Yes (cache + mover)</option>
                                            <option value="prefer">Prefer (keep on cache)</option>
                                            <option value="only">Only (cache only)</option>
                                            <option value="no">No (direct to array)</option>
                                        </select>
                                    </div>
                                    <div>
                                        <label style={{ fontSize: '12px', color: 'var(--color-text-muted)', display: 'block', marginBottom: '6px' }}>Security</label>
                                        <select
                                            value={newShare.security_mode}
                                            onChange={e => setNewShare({ ...newShare, security_mode: e.target.value })}
                                            style={{
                                                width: '100%',
                                                padding: '8px 12px',
                                                background: 'var(--color-bg-secondary)',
                                                border: '1px solid var(--color-border)',
                                                borderRadius: '8px',
                                                color: 'var(--color-text-primary)',
                                                fontSize: '13px',
                                            }}
                                        >
                                            <option value="public">Public</option>
                                            <option value="private">Private</option>
                                            <option value="secure">Secure</option>
                                        </select>
                                    </div>
                                </div>

                                <div style={{ display: 'flex', gap: '16px' }}>
                                    <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', cursor: 'pointer' }}>
                                        <input
                                            type="checkbox"
                                            checked={newShare.export_smb}
                                            onChange={e => setNewShare({ ...newShare, export_smb: e.target.checked })}
                                        />
                                        Export SMB
                                    </label>
                                    <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', cursor: 'pointer' }}>
                                        <input
                                            type="checkbox"
                                            checked={newShare.export_nfs}
                                            onChange={e => setNewShare({ ...newShare, export_nfs: e.target.checked })}
                                        />
                                        Export NFS
                                    </label>
                                </div>

                                <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '8px' }}>
                                    <button
                                        onClick={() => setShowCreateDialog(false)}
                                        style={{
                                            padding: '8px 16px',
                                            background: 'transparent',
                                            border: '1px solid var(--color-border)',
                                            borderRadius: '8px',
                                            color: 'var(--color-text-secondary)',
                                            cursor: 'pointer',
                                            fontSize: '13px',
                                        }}
                                    >
                                        Cancel
                                    </button>
                                    <button onClick={handleCreate} className="btn btn-success" style={{ padding: '8px 20px', fontSize: '13px' }}>
                                        Create
                                    </button>
                                </div>
                            </div>
                        </div>
                    </>
                )}

                {/* Users Panel */}
                <div className="glass-card" style={{ padding: '20px', marginTop: '20px' }}>
                    <h3 style={{ fontSize: '16px', fontWeight: 700, marginBottom: '12px' }}>Users</h3>
                    <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                        {users.map(user => (
                            <span key={user} style={{
                                padding: '6px 12px',
                                background: 'var(--color-bg-secondary)',
                                border: '1px solid var(--color-border)',
                                borderRadius: '8px',
                                fontSize: '12px',
                            }}>
                                {user}
                            </span>
                        ))}
                    </div>
                </div>
            </main>
        </div>
    );
}
