'use client';

import { useState, useEffect } from 'react';
import Sidebar from '@/components/Sidebar';
import ArrayStatusCard from '@/components/ArrayStatusCard';
import DiskListCard from '@/components/DiskListCard';
import StatsBar from '@/components/StatsBar';
import { api, type ArrayStatus } from '@/lib/api';

export default function Dashboard() {
  const [arrayStatus, setArrayStatus] = useState<ArrayStatus | null>(null);

  useEffect(() => {
    const fetch = async () => {
      try {
        const res = await api.array.status();
        if (res.ok && res.data) setArrayStatus(res.data);
      } catch { }
    };
    fetch();
    const interval = setInterval(fetch, 5000);
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

        {/* Main content grid */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: '20px' }}>
          <ArrayStatusCard />
          <DiskListCard />
        </div>
      </main>
    </div>
  );
}
