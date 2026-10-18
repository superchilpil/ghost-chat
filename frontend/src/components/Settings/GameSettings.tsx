import { useCallback, useEffect, useState } from 'react';

import { GetRunningExecutables } from '@bindings/ghost-chat/app.js';
import { useTranslation } from 'react-i18next';

import { Toggle } from '@/components/Toggle';
import { useConfigStore } from '@/stores/config';

export function GameSettings() {
    const { t } = useTranslation();
    const config = useConfigStore((s) => s.config);
    const update = useConfigStore((s) => s.update);
    const [manual, setManual] = useState('');
    const [runningExecutables, setRunningExecutables] = useState<string[]>([]);
    const [selectedRunning, setSelectedRunning] = useState('');
    const [loadingRunning, setLoadingRunning] = useState(false);
    const [runningError, setRunningError] = useState(false);
    const configured = config?.general?.fullscreen_game_executables ?? [];

    const refreshRunningExecutables = useCallback(async () => {
        setLoadingRunning(true);
        setRunningError(false);
        try {
            const executables = await GetRunningExecutables();
            setRunningExecutables([...new Set(executables)].sort((a, b) => a.localeCompare(b)));
        } catch {
            setRunningError(true);
            setRunningExecutables([]);
        } finally {
            setLoadingRunning(false);
        }
    }, []);

    useEffect(() => {
        void refreshRunningExecutables();
    }, [refreshRunningExecutables]);

    const availableExecutables = runningExecutables.filter(
        (name) => !configured.some((item) => item.toLowerCase() === name.toLowerCase()),
    );

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
                <label className="field-label">{t('settings.games.running_label')}</label>
                <div className="field-row">
                    <select
                        value={selectedRunning}
                        onChange={(event) => setSelectedRunning(event.target.value)}
                        disabled={loadingRunning || availableExecutables.length === 0}
                    >
                        <option value="">
                            {loadingRunning
                                ? t('settings.games.loading_running')
                                : availableExecutables.length === 0
                                  ? t('settings.games.no_running')
                                  : t('settings.games.running_placeholder')}
                        </option>
                        {availableExecutables.map((name) => (
                            <option key={name} value={name}>{name}</option>
                        ))}
                    </select>
                    <button className="btn btn-ghost" onClick={() => void refreshRunningExecutables()} disabled={loadingRunning}>
                        {loadingRunning ? t('settings.games.refreshing') : t('settings.games.refresh')}
                    </button>
                    <button
                        className="btn btn-ghost"
                        disabled={!selectedRunning}
                        onClick={() => {
                            addExecutable(selectedRunning);
                            setSelectedRunning('');
                        }}
                    >
                        {t('settings.games.add_running')}
                    </button>
                </div>
                {runningError && <span className="field-error">{t('settings.games.running_error')}</span>}
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
