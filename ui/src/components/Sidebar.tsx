'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

const navItems = [
    { href: '/', label: 'Dashboard', icon: '⬡' },
    { href: '/array', label: 'Array', icon: '◫' },
    { href: '/disks', label: 'Disks', icon: '◉' },
    { href: '/shares', label: 'Shares', icon: '⊞' },
    { href: '/apps', label: 'Apps', icon: '▦' },
    { href: '/settings', label: 'Settings', icon: '⚙' },
];

export default function Sidebar() {
    const pathname = usePathname();

    return (
        <aside style={{
            width: '240px',
            minHeight: '100vh',
            background: 'linear-gradient(180deg, #0d1321, #0a0e1a)',
            borderRight: '1px solid var(--color-border)',
            padding: '24px 12px',
            display: 'flex',
            flexDirection: 'column',
            gap: '4px',
            position: 'fixed',
            left: 0,
            top: 0,
            zIndex: 50,
        }}>
            {/* Logo */}
            <div style={{
                padding: '8px 16px',
                marginBottom: '24px',
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
            }}>
                <div style={{
                    width: '36px',
                    height: '36px',
                    borderRadius: '10px',
                    background: 'linear-gradient(135deg, #3b82f6, #06b6d4)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '18px',
                    fontWeight: 800,
                    color: 'white',
                    boxShadow: '0 4px 15px rgba(59, 130, 246, 0.4)',
                }}>
                    P
                </div>
                <div>
                    <div style={{ fontWeight: 700, fontSize: '16px', color: 'var(--color-text-primary)' }}>
                        Proxmaid
                    </div>
                    <div style={{ fontSize: '11px', color: 'var(--color-text-muted)', letterSpacing: '0.05em' }}>
                        v0.1.0
                    </div>
                </div>
            </div>

            {/* Navigation */}
            <nav style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                {navItems.map((item) => (
                    <Link
                        key={item.href}
                        href={item.href}
                        className={`nav-item ${pathname === item.href ? 'active' : ''}`}
                    >
                        <span style={{ fontSize: '16px', width: '20px', textAlign: 'center' }}>{item.icon}</span>
                        {item.label}
                    </Link>
                ))}
            </nav>

            {/* Bottom info */}
            <div style={{ marginTop: 'auto', padding: '16px', fontSize: '11px', color: 'var(--color-text-muted)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '4px' }}>
                    <span className="status-dot ok" style={{ width: '6px', height: '6px' }}></span>
                    API Connected
                </div>
                <div>NonRAID • Mock Mode</div>
            </div>
        </aside>
    );
}
