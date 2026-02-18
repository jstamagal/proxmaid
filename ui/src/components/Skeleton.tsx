'use client';

interface SkeletonProps {
    width?: string;
    height?: string;
    borderRadius?: string;
}

function SkeletonBase({ width = '100%', height = '16px', borderRadius = '6px' }: SkeletonProps) {
    return (
        <div style={{
            width,
            height,
            borderRadius,
            background: 'linear-gradient(90deg, var(--color-bg-secondary) 25%, rgba(255,255,255,0.05) 50%, var(--color-bg-secondary) 75%)',
            backgroundSize: '200% 100%',
            animation: 'shimmer 1.5s infinite',
        }} />
    );
}

export function SkeletonText({ lines = 3 }: { lines?: number }) {
    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {Array.from({ length: lines }).map((_, i) => (
                <SkeletonBase
                    key={i}
                    width={i === lines - 1 ? '60%' : '100%'}
                    height="14px"
                />
            ))}
        </div>
    );
}

export function SkeletonCard() {
    return (
        <div className="glass-card" style={{ padding: '24px' }}>
            <SkeletonBase width="40%" height="20px" />
            <div style={{ marginTop: '16px' }}>
                <SkeletonText lines={3} />
            </div>
        </div>
    );
}

export function SkeletonTable({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {/* Header */}
            <div style={{
                display: 'grid',
                gridTemplateColumns: `repeat(${cols}, 1fr)`,
                gap: '12px',
                padding: '12px 16px',
            }}>
                {Array.from({ length: cols }).map((_, i) => (
                    <SkeletonBase key={i} height="12px" width="80%" />
                ))}
            </div>
            {/* Rows */}
            {Array.from({ length: rows }).map((_, row) => (
                <div key={row} style={{
                    display: 'grid',
                    gridTemplateColumns: `repeat(${cols}, 1fr)`,
                    gap: '12px',
                    padding: '12px 16px',
                    background: 'var(--color-bg-secondary)',
                    borderRadius: '8px',
                }}>
                    {Array.from({ length: cols }).map((_, col) => (
                        <SkeletonBase key={col} height="14px" />
                    ))}
                </div>
            ))}
        </div>
    );
}

export default SkeletonBase;
