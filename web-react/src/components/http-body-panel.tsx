import { json } from '@codemirror/lang-json';
import { html } from '@codemirror/lang-html';
import { javascript } from '@codemirror/lang-javascript';
import { xml } from '@codemirror/lang-xml';
import { yaml } from '@codemirror/lang-yaml';
import { defaultHighlightStyle, foldGutter, foldKeymap, syntaxHighlighting } from '@codemirror/language';
import { EditorState } from '@codemirror/state';
import { EditorView, keymap, lineNumbers } from '@codemirror/view';
import { useCallback, useEffect, useRef, useState } from 'react';

import CodeMirror from '@uiw/react-codemirror';

import { cn } from '@/lib/utils';

export type BodyFormat = 'json' | 'html' | 'xml' | 'yaml' | 'javascript' | 'text' | 'sse';
export type BodyConfig = { value: string; format: BodyFormat };

const MAX_HIGHLIGHT_LENGTH = 200000;

const baseExtensions = [
  lineNumbers(),
  foldGutter(),
  keymap.of(foldKeymap),
  EditorView.lineWrapping,
  EditorView.editable.of(false),
  EditorState.readOnly.of(true),
  syntaxHighlighting(defaultHighlightStyle),
];

const getLanguageExtension = (format: BodyFormat) => {
  switch (format) {
    case 'json':
      return json();
    case 'html':
      return html();
    case 'xml':
      return xml();
    case 'yaml':
      return yaml();
    case 'javascript':
      return javascript();
    default:
      return null;
  }
};

const BodyViewer = ({ config, className }: { config: BodyConfig; className?: string }) => {
  if (!config.value) {
    return (
      <div
        className={cn(
          'mt-1 flex min-h-0 flex-1 items-center justify-center rounded-md border border-border/60 bg-muted/30 p-2 text-xs text-muted-foreground',
          className
        )}
      >
        无正文
      </div>
    );
  }

  if (config.value.length > MAX_HIGHLIGHT_LENGTH) {
    return (
      <pre
        className={cn(
          'mt-1 min-h-0 flex-1 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground',
          className
        )}
      >
        {config.value}
      </pre>
    );
  }

  if (config.format === 'sse') {
    const lines = config.value.split(/\r?\n/);
    const sseKeys = new Set(['event', 'data', 'id', 'retry']);

    return (
      <pre
        className={cn(
          'mt-1 min-h-0 flex-1 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground',
          className
        )}
      >
        {lines.map((line, index) => {
          const lineKey = `${index}-${line}`;
          if (!line) {
            return <span key={lineKey}>{'\n'}</span>;
          }

          if (line.startsWith(':')) {
            return (
              <span key={lineKey} className="text-muted-foreground">
                {line}
                {'\n'}
              </span>
            );
          }

          const match = line.match(/^([^:]+):(.*)$/);
          if (!match) {
            return (
              <span key={lineKey} className="text-foreground">
                {line}
                {'\n'}
              </span>
            );
          }

          const key = match[1].trim();
          const value = match[2].replace(/^\s?/, '');

          return (
            <span key={lineKey}>
              <span className={sseKeys.has(key) ? 'text-primary' : 'text-foreground'}>{key}:</span>
              {value ? <span className="ml-1 text-foreground">{value}</span> : null}
              {'\n'}
            </span>
          );
        })}
      </pre>
    );
  }

  const language = getLanguageExtension(config.format);
  const extensions = language ? [...baseExtensions, language] : baseExtensions;

  return (
    <div
      className={cn(
        'mt-1 min-h-0 flex-1 overflow-hidden rounded-md border border-border/60 bg-muted/40 text-xs',
        className
      )}
    >
      <CodeMirror
        value={config.value}
        extensions={extensions}
        editable={false}
        basicSetup={false}
        height="100%"
        className="h-full text-xs"
      />
    </div>
  );
};

export function HttpBodyPanel({
  title,
  config,
  className,
}: {
  title: string;
  config: BodyConfig;
  className?: string;
}) {
  const [copyState, setCopyState] = useState<'idle' | 'success' | 'error'>('idle');
  const resetTimer = useRef<number | null>(null);
  const hasBody = Boolean(config.value);

  const clearResetTimer = useCallback(() => {
    if (resetTimer.current !== null) {
      window.clearTimeout(resetTimer.current);
      resetTimer.current = null;
    }
  }, []);

  const scheduleReset = () => {
    clearResetTimer();
    resetTimer.current = window.setTimeout(() => {
      setCopyState('idle');
      resetTimer.current = null;
    }, 1400);
  };

  const handleCopy = async () => {
    if (!hasBody) {
      return;
    }

    try {
      await navigator.clipboard.writeText(config.value);
      setCopyState('success');
    } catch (error) {
      setCopyState('error');
    }

    scheduleReset();
  };

  useEffect(() => {
    if (copyState !== 'idle') {
      return undefined;
    }

    clearResetTimer();
    return () => {
      clearResetTimer();
    };
  }, [clearResetTimer, copyState]);

  return (
    <div className={cn('flex min-h-0 flex-col', className)}>
      <div className="flex items-start justify-between gap-3">
        <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">{title}</p>
        <div className="flex items-center gap-2">
          {copyState === 'success' ? (
            <span className="text-[10px] font-semibold uppercase tracking-[0.24em] text-primary">Copied</span>
          ) : null}
          {copyState === 'error' ? (
            <span className="text-[10px] font-semibold uppercase tracking-[0.24em] text-destructive">Failed</span>
          ) : null}
          <button
            type="button"
            onClick={handleCopy}
            aria-label="Copy body"
            disabled={!hasBody}
            title={hasBody ? 'Copy body' : 'No body to copy'}
            className={cn(
              'inline-flex h-6 w-6 items-center justify-center rounded-md border border-border/60 text-muted-foreground transition',
              'hover:bg-muted/40 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              'disabled:cursor-not-allowed disabled:opacity-40'
            )}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true" className="h-3.5 w-3.5">
              <path
                fill="currentColor"
                d="M16 1H6C4.9 1 4 1.9 4 3v12h2V3h10V1zm3 4H10c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h9c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H10V7h9v14z"
              />
            </svg>
          </button>
        </div>
      </div>
      <BodyViewer config={config} className="flex-1 min-h-0" />
    </div>
  );
}
