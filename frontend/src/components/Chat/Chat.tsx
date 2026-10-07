import type React from 'react';

import type { ChatMessage as ChatMessageType } from '@/types/chat';

import { ToggleVanish } from '@bindings/ghost-chat/app.js';
import { Platform } from '@bindings/ghost-chat/internal/chat/models.js';
import { Events } from '@wailsio/runtime';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

import { fadePolicy, shouldDisplay } from '@/filter/messageFilter';
import { useConfigStore } from '@/stores/config';
import { useConnectionStore } from '@/stores/connection';
import { getThemeById, themeToCSS } from '@/types/theme';

import styles from './Chat.module.css';
import { ChatMessage } from './ChatMessage';
import { EventMessage } from './EventMessage';

const MAX_MESSAGES = 500;

export function Chat() {
    const { t } = useTranslation();
    const navigate = useNavigate();
    const config = useConfigStore((s) => s.config);
    const [messages, setMessages] = useState<ChatMessageType[]>([]);
    const [autoScroll, setAutoScroll] = useState(true);
    const autoScrollRef = useRef(true);
    const userScrollingRef = useRef(false);
    const [showTwitch, setShowTwitch] = useState(true);
    const [showYoutube, setShowYoutube] = useState(true);
    const [showKick, setShowKick] = useState(true);
    const twitchConnected = useConnectionStore((s) => s.connected[Platform.PlatformTwitch]);
    const youtubeConnected = useConnectionStore((s) => s.connected[Platform.PlatformYouTube]);
    const youtubeTransport = useConnectionStore((s) => s.transports[Platform.PlatformYouTube]);
    const kickConnected = useConnectionStore((s) => s.connected[Platform.PlatformKick]);
    const connected = twitchConnected || youtubeConnected || kickConnected;
    const connectedCount = [twitchConnected, youtubeConnected, kickConnected].filter(Boolean).length;
    const messagesRef = useRef<HTMLDivElement>(null);
    const seenMessageIdsRef = useRef<Set<string>>(new Set());

    const activeThemeId = config?.theme?.active_theme_id ?? 'default';
    const customThemes = config?.theme?.custom_themes ?? [];
    const theme = getThemeById(activeThemeId, customThemes);
    const themeCSSVars = themeToCSS(theme);

    const topToBottom = theme.top_to_bottom ?? false;

    const hideBadges = config?.twitch?.hide_badges ?? false;
    const showTimestamp = config?.general?.show_timestamps ?? false;

    useEffect(() => {
        const cancelMessage = Events.On('chat:message', (ev) => {
            const msg = ev.data as ChatMessageType;
            const cfg = useConfigStore.getState().config;

            if (!shouldDisplay(msg, cfg)) {
                return;
            }

            // Keep a frontend-side guard as a final line of defense against
            // replayed messages after a reconnect or transport recovery.
            // Use platform + ID because different services can use overlapping IDs.
            if (msg.id) {
                const messageKey = `${msg.platform}:${msg.id}`;
                if (seenMessageIdsRef.current.has(messageKey)) {
                    return;
                }
                seenMessageIdsRef.current.add(messageKey);

                // Bound the set so a very long stream cannot grow memory forever.
                if (seenMessageIdsRef.current.size > MAX_MESSAGES * 4) {
                    const keep = Array.from(seenMessageIdsRef.current).slice(-MAX_MESSAGES * 2);
                    seenMessageIdsRef.current = new Set(keep);
                }
            }

            setMessages((prev) => {
                const next = [...prev, msg];

                // YouTube can deliver a batch of existing chat messages in an
                // order that differs from the live message order. Keep the
                // displayed history chronological so older messages remain
                // above newer messages.
                next.sort((a, b) => {
                    const aTime = Date.parse(a.timestamp);
                    const bTime = Date.parse(b.timestamp);

                    if (Number.isNaN(aTime) || Number.isNaN(bTime)) {
                        return 0;
                    }

                    return aTime - bTime;
                });

                if (next.length > MAX_MESSAGES) {
                    return next.slice(next.length - MAX_MESSAGES);
                }
                return next;
            });
        });

        const cancelClear = Events.On('chat:clear', (ev) => {
            const username = ev.data as string;

            setMessages((prev) => prev.filter((m) => m.username.toLowerCase() !== username.toLowerCase()));
        });

        const cancelDelete = Events.On('chat:delete-message', (ev) => {
            const msgId = ev.data as string;

            setMessages((prev) => prev.filter((m) => m.id !== msgId));
        });

        return () => {
            cancelMessage();
            cancelClear();
            cancelDelete();
        };
    }, []);

    useEffect(() => {
        if (autoScrollRef.current && !userScrollingRef.current && messagesRef.current) {
            messagesRef.current.scrollTop = topToBottom ? 0 : messagesRef.current.scrollHeight;
        }
    }, [messages, topToBottom]);

    useEffect(() => {
        const el = messagesRef.current;

        if (!el) {
            return;
        }

        let scrollTimer: ReturnType<typeof setTimeout>;

        const handleWheel = (e: WheelEvent) => {
            const scrollingAway = topToBottom ? e.deltaY > 0 : e.deltaY < 0;

            if (scrollingAway) {
                userScrollingRef.current = true;
                autoScrollRef.current = false;
                setAutoScroll(false);

                clearTimeout(scrollTimer);

                scrollTimer = setTimeout(() => {
                    userScrollingRef.current = false;
                }, 150);
            }
        };

        el.addEventListener('wheel', handleWheel, { passive: true });

        return () => {
            el.removeEventListener('wheel', handleWheel);
            clearTimeout(scrollTimer);
        };
    }, [topToBottom]);

    const handleScroll = () => {
        const el = messagesRef.current;

        if (!el) {
            return;
        }

        let atEdge: boolean;

        if (topToBottom) {
            atEdge = el.scrollTop < 40;
        } else {
            atEdge = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
        }

        if (atEdge && !autoScrollRef.current) {
            autoScrollRef.current = true;
            setAutoScroll(true);
        }
    };

    const displayMessages = topToBottom ? [...messages].toReversed() : messages;

    const visibleMessages = displayMessages.filter((m) => {
        if (m.platform === Platform.PlatformTwitch) {
            return showTwitch;
        }

        if (m.platform === Platform.PlatformYouTube) {
            return showYoutube;
        }

        if (m.platform === Platform.PlatformKick) {
            return showKick;
        }

        return true;
    });

    return (
        <div className={styles.chat}>
            <div className={styles.header}>
                <button
                    className="btn btn-ghost"
                    onClick={() => navigate('/')}
                >
                    <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <polyline points="15 18 9 12 15 6" />
                    </svg>
                    {t('chat.back')}
                </button>
                {connectedCount > 1 && (
                    <div className={styles.filters}>
                        {twitchConnected && (
                            <button
                                className={`${styles.filterBtn} ${showTwitch ? styles.filterActive : ''}`}
                                onClick={() => setShowTwitch((v) => !v)}
                                title="Toggle Twitch messages"
                            >
                                T
                            </button>
                        )}
                        {youtubeConnected && (
                            <button
                                className={`${styles.filterBtn} ${showYoutube ? styles.filterActive : ''}`}
                                onClick={() => setShowYoutube((v) => !v)}
                                title="Toggle YouTube messages"
                            >
                                YT
                            </button>
                        )}
                        {kickConnected && (
                            <button
                                className={`${styles.filterBtn} ${showKick ? styles.filterActive : ''}`}
                                onClick={() => setShowKick((v) => !v)}
                                title="Toggle Kick messages"
                            >
                                K
                            </button>
                        )}
                    </div>
                )}
                <button
                    className={styles.vanishBtn}
                    onClick={() => void ToggleVanish()}
                    title={t('chat.vanish')}
                >
                    <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                        <circle
                            cx="12"
                            cy="12"
                            r="3"
                        />
                    </svg>
                </button>
                {youtubeConnected && youtubeTransport && (
                    <span className={styles.transport} title={youtubeTransport}>
                        YouTube: {youtubeTransport === 'streamList'
                            ? 'StreamList'
                            : youtubeTransport.startsWith('innertube:')
                              ? `Innertube fallback — ${youtubeTransport.slice('innertube:'.length)}`
                              : 'Innertube fallback'}
                    </span>
                )}
                {!connected && <span className={styles.disconnected}>{t('chat.disconnected')}</span>}
            </div>
            <div
                ref={messagesRef}
                className={styles.messages}
                style={themeCSSVars as React.CSSProperties}
                onScroll={handleScroll}
            >
                {!topToBottom && <div className={styles.messagesSpacer} />}
                {visibleMessages.length === 0
                    ? config?.general?.show_waiting_message !== false && (
                          <div className={styles.empty}>
                              <span>{t('chat.waiting')}</span>
                          </div>
                      )
                    : visibleMessages.map((msg) => {
                          const { fade, timeoutSeconds } = fadePolicy(msg.platform, config);

                          return msg.eventType ? (
                              <EventMessage
                                  key={msg.id}
                                  message={msg}
                                  showTimestamp={showTimestamp}
                                  fade={fade}
                                  fadeTimeout={timeoutSeconds}
                                  onFaded={(id) => setMessages((prev) => prev.filter((m) => m.id !== id))}
                              />
                          ) : (
                              <ChatMessage
                                  key={msg.id}
                                  message={msg}
                                  hideBadges={hideBadges}
                                  showTimestamp={showTimestamp}
                                  showPlatformIcon={connectedCount > 1}
                                  showColon={theme.show_colon}
                                  showAvatars={theme.show_avatars}
                                  fade={fade}
                                  fadeTimeout={timeoutSeconds}
                                  onFaded={(id) => setMessages((prev) => prev.filter((m) => m.id !== id))}
                              />
                          );
                      })}
            </div>
            {!autoScroll && (
                <button
                    className={`btn btn-ghost ${styles.scrollBtn}`}
                    onClick={() => {
                        autoScrollRef.current = true;
                        setAutoScroll(true);
                        messagesRef.current?.scrollTo({
                            top: topToBottom ? 0 : messagesRef.current.scrollHeight,
                            behavior: 'smooth',
                        });
                    }}
                >
                    {topToBottom ? t('chat.scroll_to_top') : t('chat.scroll_to_bottom')}
                </button>
            )}
        </div>
    );
}
