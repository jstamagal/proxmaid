'use client';

interface StatItemProps {
    label: string;
    value: string | number;
    color?: string;
    subtext?: string;
}

function StatItem({ label, value, color, subtext }: StatItemProps) {
    return (
        <div className="glass-card" style={{
            padding: '20px 24px',
            flex: 1,
            minWidth: '160px',
        }}>
            <div style={{ fontSize: '12px', color: 'var(--color-text-muted)', marginBottom: '8px', letterSpacing: '0.04em', textTransform: 'uppercase' }}>
                {label}
            </div>
            <div style={{
                fontSize: '28px',
                fontWeight: 800,
                color: color || 'var(--color-text-primary)',
                lineHeight: 1,
                marginBottom: subtext ? '4px' : 0,
            }}>
                {value}
            </div>
            {subtext && (
                <div style={{ fontSize: '11px', color: 'var(--color-text-muted)' }}>
                    {subtext}
                </div>
            )}
        </div>
    );
}

interface StatsBarProps {
    arrayState: string;
    numDisks: number;
    numInvalid: number;
    synced: boolean;
}

export default function StatsBar({ arrayState, numDisks, numInvalid, synced }: StatsBarProps) {
    return (
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
            <StatItem
                label="Array"
                value={arrayState || '—'}
                color={arrayState === 'STARTED' ? 'var(--color-success)' : 'var(--color-text-muted)'}
            />
            <StatItem
                label="Disks"
                value={numDisks}
                color="var(--color-accent)"
                subtext={numInvalid > 0 ? `${numInvalid} degraded` : 'All healthy'}
            />
            <StatItem
                label="Parity"
                value={synced ? 'Synced' : 'Not Synced'}
                color={synced ? 'var(--color-success)' : 'var(--color-warning)'}
            />
            <StatItem
                label="Uptime"
                value="—"
                subtext="v0.1.0"
            />
        </div>
    );
}
