import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { GetYouTubeAPIRequestUsage, SelectChatLogDirectory, SetYouTubeAPIBypassEnabled, SetYouTubeAPIBypassPassword } from '@bindings/ghost-chat/app.js';

import { Toggle } from '@/components/Toggle';
import { useConfigStore } from '@/stores/config';

import { HotkeyInput } from './HotkeyInput';

const languages = [
    { value: 'en-US', label: 'English' },
    { value: 'de-DE', label: 'Deutsch' },
];

export function GeneralSettings() {
    const { t, i18n } = useTranslation();
    const config = useConfigStore((s) => s.config);
    const update = useConfigStore((s) => s.update);
    const [bypassPassword, setBypassPassword] = useState('');
    const [bypassStatus, setBypassStatus] = useState<string | null>(null);
    const [bypassEnabled, setBypassEnabled] = useState(false);

    useEffect(() => {
        void GetYouTubeAPIRequestUsage().then((usage) => {
            setBypassEnabled(Boolean((usage as { bypass?: boolean }).bypass));
        }).catch(() => {});
    }, []);

    const handleLanguageChange = (lang: string) => {
        void i18n.changeLanguage(lang);
        void update({ general: { language: lang } });
    };

    return (
        <>
            <div className="field">
                <label className="field-label">{t('settings.general.language')}</label>
                <select
                    value={config?.general?.language ?? 'en-US'}
                    onChange={(e) => handleLanguageChange(e.target.value)}
                >
                    {languages.map((lang) => (
                        <option
                            key={lang.value}
                            value={lang.value}
                        >
                            {lang.label}
                        </option>
                    ))}
                </select>
            </div>

            <div className="field-row">
                <label className="field-label">{t('settings.general.show_timestamps')}</label>
                <Toggle
                    checked={config?.general?.show_timestamps ?? false}
                    onChange={(v) => void update({ general: { show_timestamps: v } })}
                />
            </div>

            <div className="field-row">
                <label className="field-label">{t('settings.general.show_waiting_message')}</label>
                <Toggle
                    checked={config?.general?.show_waiting_message ?? true}
                    onChange={(v) => void update({ general: { show_waiting_message: v } })}
                />
            </div>

            <div className="field-row">
                <label className="field-label">{t('settings.general.minimize_to_tray')}</label>
                <Toggle
                    checked={config?.general?.minimize_to_tray ?? false}
                    onChange={(v) => void update({ general: { minimize_to_tray: v } })}
                />
            </div>

            <div className="field">
                <label className="field-label">{t('settings.general.live_poll_interval')}</label>
                <select
                    value={config?.general?.live_poll_interval ?? 15}
                    onChange={(e) => void update({ general: { live_poll_interval: Number(e.target.value) } })}
                >
                    {[5, 10, 15, 30, 60, 120, 300].map((seconds) => (
                        <option key={seconds} value={seconds}>
                            {seconds < 60 ? `${seconds} seconds` : `${seconds / 60} minute${seconds === 60 ? '' : 's'}`}
                        </option>
                    ))}
                </select>
            </div>

            <div className="field-section">
                <label className="field-section-label">{t('settings.general.youtube_api_safety_section')}</label>
                <span className="field-hint">{t('settings.general.youtube_api_safety_hint')}</span>
            </div>

            <div className="field">
                <label className="field-label">{t('settings.general.youtube_api_bypass_password')}</label>
                <div className="field-row">
                    <input
                        type="password"
                        value={bypassPassword}
                        onChange={(e) => {
                            setBypassPassword(e.target.value);
                            setBypassStatus(null);
                        }}
                        placeholder={t('settings.general.youtube_api_bypass_placeholder')}
                        autoComplete="off"
                    />
                    <button
                        className="btn btn-ghost"
                        disabled={bypassEnabled ? false : !bypassPassword}
                        onClick={async () => {
                            try {
                                if (bypassEnabled) {
                                    await SetYouTubeAPIBypassEnabled(false);
                                    setBypassEnabled(false);
                                    setBypassStatus(t('settings.general.youtube_api_bypass_disabled'));
                                    return;
                                }
                                const enabled = await SetYouTubeAPIBypassPassword(bypassPassword);
                                setBypassEnabled(enabled);
                                setBypassStatus(enabled ? t('settings.general.youtube_api_bypass_enabled') : null);
                                setBypassPassword('');
                            } catch (error) {
                                setBypassStatus(String(error));
                            }
                        }}
                    >
                        {bypassEnabled ? t('settings.general.youtube_api_bypass_disable') : t('settings.general.youtube_api_bypass_button')}
                    </button>
                </div>
                {bypassStatus && <span className="field-success">{bypassStatus}</span>}
            </div>

            <div className="field-section">
                <label className="field-section-label">{t('settings.general.chat_log_section')}</label>
                <span className="field-hint">{t('settings.general.chat_log_hint')}</span>
            </div>

            <div className="field-row">
                <label className="field-label">{t('settings.general.chat_log_enabled')}</label>
                <Toggle
                    checked={config?.general?.chat_log_enabled ?? false}
                    onChange={(v) => void update({ general: { chat_log_enabled: v } })}
                />
            </div>

            <div className="field">
                <label className="field-label">{t('settings.general.chat_log_location')}</label>
                <div className="field-row">
                    <input
                        className="chat-log-location-input"
                        value={config?.general?.chat_log_directory ?? ''}
                        placeholder={t('settings.general.chat_log_placeholder')}
                        title={config?.general?.chat_log_directory ?? ''}
                        readOnly
                    />
                    <button
                        className="btn btn-ghost"
                        onClick={async () => {
                            try {
                                const path = await SelectChatLogDirectory();
                                if (path) {
                                    await update({ general: { chat_log_directory: path } });
                                }
                            } catch (error) {
                                console.error('Failed to select chat log directory:', error);
                            }
                        }}
                    >
                        {t('settings.general.chat_log_browse')}
                    </button>
                </div>
                <span className="field-hint">{t('settings.general.chat_log_location_hint')}</span>
            </div>

            <div className="field-row">
                <label className="field-label">{t('settings.general.auto_show_on_live')}</label>
                <Toggle
                    checked={config?.general?.auto_show_on_live ?? false}
                    onChange={(v) => void update({ general: { auto_show_on_live: v } })}
                />
            </div>

            <div className="field">
                <label className="field-label">{t('settings.general.vanish_hotkey')}</label>
                <HotkeyInput
                    value={config?.keybinds?.vanish?.keybind ?? ''}
                    onChange={(v) => void update({ keybinds: { vanish: { keybind: v } } })}
                />
            </div>
        </>
    );
}
