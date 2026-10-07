import type { YouTubeConfig } from '@bindings/ghost-chat/internal/config/models.js';

import type { DeepPartial } from '@/types/utils';

import { ResolveYouTubeVideo, SetYouTubeAPIBypassPassword } from '@bindings/ghost-chat/app.js';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useConfigStore } from '@/stores/config';

import { BlacklistField } from './fields/BlacklistField';
import { ChannelField } from './fields/ChannelField';
import { FadeControls } from './fields/FadeControls';

export function YouTubeSettings() {
    const { t } = useTranslation();
    const yt = useConfigStore((s) => s.config?.youtube);
    const update = useConfigStore((s) => s.update);

    const [channelText, setChannelText] = useState(yt?.channel_id ?? '');
    const [resolving, setResolving] = useState(false);
    const [resolveError, setResolveError] = useState<string | null>(null);
    const [resolvedUrl, setResolvedUrl] = useState<string | null>(null);
    const [bypassPassword, setBypassPassword] = useState('');
    const [bypassStatus, setBypassStatus] = useState<string | null>(null);

    const set = (partial: DeepPartial<YouTubeConfig>) => update({ youtube: partial });

    const handleAutoDetect = async () => {
        if (!channelText) {
            return;
        }

        setResolving(true);
        setResolveError(null);
        setResolvedUrl(null);

        try {
            const url = await ResolveYouTubeVideo(channelText);

            setResolvedUrl(url);
            void set({ channel_id: channelText, video_url: url });
        } catch (err) {
            setResolveError(String(err));
        } finally {
            setResolving(false);
        }
    };

    return (
        <>
            <ChannelField
                initialValue={yt?.channel_id ?? ''}
                labelKey="settings.youtube.channel"
                placeholderKey="settings.youtube.channel_placeholder"
                onSave={(v) => {
                    setResolveError(null);
                    setResolvedUrl(null);
                    void set({ channel_id: v });
                }}
                onTextChange={(v) => {
                    setChannelText(v);
                    setResolvedUrl(null);
                    setResolveError(null);
                }}
                footer={
                    <>
                        {resolvedUrl && <span className="field-success">{resolvedUrl}</span>}
                        {resolveError && <span className="field-error">{resolveError}</span>}
                    </>
                }
            >
                <button
                    className="btn btn-ghost"
                    onClick={handleAutoDetect}
                    disabled={!channelText || resolving}
                    style={{ flexShrink: 0 }}
                >
                    {resolving ? t('settings.youtube.detecting') : t('settings.youtube.detect')}
                </button>
            </ChannelField>

            <div className="field-section">
                <label className="field-section-label">{t('settings.youtube.api_safety_section')}</label>
                <span className="field-hint">{t('settings.youtube.api_safety_hint')}</span>
            </div>

            <div className="field">
                <label className="field-label">{t('settings.youtube.api_bypass_password')}</label>
                <div className="field-row">
                    <input
                        type="password"
                        value={bypassPassword}
                        onChange={(e) => {
                            setBypassPassword(e.target.value);
                            setBypassStatus(null);
                        }}
                        placeholder={t('settings.youtube.api_bypass_placeholder')}
                        autoComplete="off"
                    />
                    <button
                        className="btn btn-ghost"
                        disabled={!bypassPassword}
                        onClick={async () => {
                            try {
                                const enabled = await SetYouTubeAPIBypassPassword(bypassPassword);
                                setBypassStatus(enabled ? t('settings.youtube.api_bypass_enabled') : null);
                                setBypassPassword('');
                            } catch (error) {
                                setBypassStatus(String(error));
                            }
                        }}
                    >
                        {t('settings.youtube.api_bypass_button')}
                    </button>
                </div>
                {bypassStatus && <span className="field-success">{bypassStatus}</span>}
            </div>

            <FadeControls
                fade={yt?.fade ?? false}
                fadeTimeout={yt?.fade_timeout ?? 30}
                onFadeChange={(v) => set({ fade: v })}
                onFadeTimeoutChange={(v) => set({ fade_timeout: v })}
            />

            <BlacklistField
                initialValue={yt?.user_blacklist ?? []}
                onSave={(v) => set({ user_blacklist: v })}
            />
        </>
    );
}
