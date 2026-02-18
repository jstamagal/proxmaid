'use client';

import { type ChangeEvent } from 'react';

const labelStyle = {
    display: 'block' as const,
    fontSize: '12px',
    fontWeight: 600,
    color: 'var(--color-text-muted)',
    marginBottom: '6px',
    textTransform: 'uppercase' as const,
};

const inputStyle = {
    width: '100%',
    padding: '10px 14px',
    background: 'var(--color-bg-secondary)',
    border: '1px solid var(--color-border)',
    borderRadius: '8px',
    color: 'var(--color-text)',
    fontSize: '14px',
    outline: 'none',
    boxSizing: 'border-box' as const,
};

const errorStyle = {
    fontSize: '11px',
    color: 'var(--color-danger)',
    marginTop: '4px',
};

interface TextInputProps {
    label: string;
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    error?: string;
    disabled?: boolean;
    type?: string;
}

export function TextInput({ label, value, onChange, placeholder, error, disabled, type = 'text' }: TextInputProps) {
    return (
        <div>
            <label style={labelStyle}>{label}</label>
            <input
                type={type}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                placeholder={placeholder}
                disabled={disabled}
                style={{
                    ...inputStyle,
                    opacity: disabled ? 0.5 : 1,
                    borderColor: error ? 'var(--color-danger)' : 'var(--color-border)',
                }}
            />
            {error && <div style={errorStyle}>{error}</div>}
        </div>
    );
}

interface SelectProps {
    label: string;
    value: string;
    onChange: (value: string) => void;
    options: { value: string; label: string }[];
    error?: string;
    disabled?: boolean;
}

export function Select({ label, value, onChange, options, error, disabled }: SelectProps) {
    return (
        <div>
            <label style={labelStyle}>{label}</label>
            <select
                value={value}
                onChange={(e) => onChange(e.target.value)}
                disabled={disabled}
                style={{
                    ...inputStyle,
                    opacity: disabled ? 0.5 : 1,
                    borderColor: error ? 'var(--color-danger)' : 'var(--color-border)',
                    cursor: disabled ? 'not-allowed' : 'pointer',
                }}
            >
                {options.map(opt => (
                    <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
            </select>
            {error && <div style={errorStyle}>{error}</div>}
        </div>
    );
}

interface ToggleProps {
    label: string;
    checked: boolean;
    onChange: (checked: boolean) => void;
    disabled?: boolean;
}

export function Toggle({ label, checked, onChange, disabled }: ToggleProps) {
    return (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: '13px', fontWeight: 600 }}>{label}</span>
            <div
                onClick={() => !disabled && onChange(!checked)}
                style={{
                    width: '40px',
                    height: '22px',
                    borderRadius: '11px',
                    background: checked ? 'var(--color-accent)' : 'var(--color-bg-secondary)',
                    border: `1px solid ${checked ? 'var(--color-accent)' : 'var(--color-border)'}`,
                    cursor: disabled ? 'not-allowed' : 'pointer',
                    position: 'relative',
                    transition: 'background 0.2s',
                    opacity: disabled ? 0.5 : 1,
                }}
            >
                <div style={{
                    width: '16px',
                    height: '16px',
                    borderRadius: '50%',
                    background: '#fff',
                    position: 'absolute',
                    top: '2px',
                    left: checked ? '20px' : '2px',
                    transition: 'left 0.2s',
                }} />
            </div>
        </div>
    );
}

interface NumberInputProps {
    label: string;
    value: number;
    onChange: (value: number) => void;
    min?: number;
    max?: number;
    error?: string;
    disabled?: boolean;
}

export function NumberInput({ label, value, onChange, min, max, error, disabled }: NumberInputProps) {
    const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
        const num = parseInt(e.target.value, 10);
        if (!isNaN(num)) onChange(num);
    };

    return (
        <div>
            <label style={labelStyle}>{label}</label>
            <input
                type="number"
                value={value}
                onChange={handleChange}
                min={min}
                max={max}
                disabled={disabled}
                style={{
                    ...inputStyle,
                    opacity: disabled ? 0.5 : 1,
                    borderColor: error ? 'var(--color-danger)' : 'var(--color-border)',
                }}
            />
            {error && <div style={errorStyle}>{error}</div>}
        </div>
    );
}
