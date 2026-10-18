import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { GetRunningExecutables } from '@bindings/ghost-chat/app.js';
import { Toggle } from '@/components/Toggle';
import { useConfigStore } from '@/stores/config';

export function GameSettings() {
    const { t } = useTranslation();
    const config = useConfigStore((s) => s.config);
    const update = useConfigStore((s) => s.update);
    const [running, setRunning] = useState<string[]>([]);
    const [selected, setSelected] = useState('');
    const [manual, setManual] = useState('');
    const [loading, setLoading] = useState(false);

    const configured = config?.general?.fullscreen_game_executables ?? [];

    const refreshRunning = async () => {
        setLoading(true);
        try {
            const names = await GetRunningExecutables();
            setRunning([...names].sort((a, b) => a.localeCompare(b)));
        } catch (error) {
            console.error('Failed to list running executables:', error);
        } finally {
            setLoading(false);
        }
    };

    const addExecutable = (value: string) => {
        const name = value.trim().split(/[\\/]/).pop() ?? '';
        if (!name) return;
        const normalized = name.toLowerCase().endsWith('.exe') ? name : name + '.exe';
        if (configured.some((item) => item.toLowerCase() === normalized.toLowerCase())) {
            setManual('');
            return;
        }
        void update({ general: { fullscreen_game_executables: [...configured, normalized] } });
        setManual('');
        setSelected('');
    };

    const removeExecutable = (name: string) => {
        void update({ general: { fullscreen_game_executables: configured.filter((item) => item !== name) } });
    };

    return (
        <>
            <div className="field-section">
                <label className="field-section-label">{t('settings.games.title')}</label>
                <span className="field-hint">{t('settings.games.description')}</span>
            </div>

            <div className="field-row">
                <label className="field-label">{t('settings.games.enable')}</label>
                <Toggle
                    checked={config?.general?.fullscreen_game_mode ?? false}
                    onChange={(value) => void update({ general: { fullscreen_game_mode: value } })}
                />
            </div>

            <span className="field-hint">{t('settings.games.hint')}</span>

            <div className="field">
                <label className="field-label">{t('settings.games.running_apps')}</label>
                <div className="field-row">
                    <select value={selected} onChange={(event) => setSelected(event.target.value)}>
                        <option value="">{t('settings.games.choose_running')}</option>
                        {running.map((name) => (
                            <option key={name} value={name}>{name}</option>
                        ))}
                    </select>
                    <button className="btn btn-ghost" disabled={loading} onClick={() => void refreshRunning()}>
                        {loading ? '…' : t('settings.games.refresh')}
                    </button>
                    <button className="btn btn-ghost" disabled={!selected} onClick={() => addExecutable(selected)}>
                        {t('settings.games.add_selected')}
                    </button>
                </div>
            </div>

            <div className="field">
                <label className="field-label">{t('settings.games.manual_exe')}</label>
                <div className="field-row">
                    <input
                        value={manual}
                        placeholder={t('settings.games.manual_placeholder')}
                        onChange={(event) => setManual(event.target.value)}
                        onKeyDown={(event) => {
                            if (event.key === 'Enter') addExecutable(manual);
                        }}
                    />
                    <button className="btn btn-ghost" disabled={!manual.trim()} onClick={() => addExecutable(manual)}>
                        {t('settings.games.add')}
                    </button>
                </div>
            </div>

            <div className="field-section">
                <label className="field-section-label">{t('settings.games.configured')}</label>
            </div>
            {configured.length === 0 ? (
                <span className="field-hint">{t('settings.games.empty')}</span>
            ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {configured.map((name) => (
                        <div key={name} className="field-row">
                            <span className="field-label">{name}</span>
                            <button className="btn btn-ghost" onClick={() => removeExecutable(name)}>
                                {t('settings.games.remove')}
                            </button>
                        </div>
                    ))}
                </div>
            )}
        </>
    );
}
